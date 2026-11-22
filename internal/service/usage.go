package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"

	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
)

// Usage events, sent once per day when today's tokens cross 80% and 100%
// of limits.max_tokens_per_day.
const (
	EventUsageWarning = "usage.warning"
	EventUsageLimit   = "usage.limit"
)

// ErrTokenLimit means a token limit stops further model calls (HTTP 429).
var ErrTokenLimit = errors.New("token limit reached")

const (
	usageFile     = "usage.json"
	keepUsageDays = 90
	maxUsageFile  = 64 << 10
	warnPercent   = 80
)

// Usage is today's token use and the limits that apply.
type Usage struct {
	Day      string `json:"day"`
	Tokens   int    `json:"tokens"`
	DayLimit int    `json:"dayLimit"` // 0: none
	RunLimit int    `json:"runLimit"` // 0: none
}

type usageLedger struct {
	Days map[string]int `json:"days"`
}

func (s *Service) usagePath() string { return filepath.Join(filepath.Dir(s.Workspace), usageFile) }

func (s *Service) today() string { return s.now().Format("2006-01-02") }

// readUsage loads the ledger; a missing or unreadable file starts empty.
func (s *Service) readUsage() (usageLedger, error) {
	l := usageLedger{Days: map[string]int{}}
	f, err := os.Open(s.usagePath())
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, fmt.Errorf("usage: %w", err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxUsageFile))
	if err != nil {
		return l, fmt.Errorf("usage: %w", err)
	}
	if json.Unmarshal(b, &l) != nil || l.Days == nil {
		return usageLedger{Days: map[string]int{}}, nil // damaged: start over rather than block every call
	}
	return l, nil
}

// addUsage adds n tokens to today and returns today's total before and after.
func (s *Service) addUsage(n int) (before, after int, err error) {
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	l, err := s.readUsage()
	if err != nil {
		return 0, 0, err
	}
	day := s.today()
	before = l.Days[day]
	l.Days[day] = before + n
	days := make([]string, 0, len(l.Days))
	for d := range l.Days {
		days = append(days, d)
	}
	sort.Strings(days)
	for _, d := range days[:max(0, len(days)-keepUsageDays)] {
		delete(l.Days, d)
	}
	if err := os.MkdirAll(filepath.Dir(s.usagePath()), 0o700); err != nil {
		return before, before + n, fmt.Errorf("usage: %w", err)
	}
	return before, before + n, writeJSONFile(s.usagePath(), l)
}

// Usage reports today's tokens and the configured limits.
func (s *Service) Usage() (Usage, error) {
	l, err := s.readUsage()
	day := s.today()
	return Usage{
		Day: day, Tokens: l.Days[day],
		DayLimit: s.cfg().Limits.MaxTokensPerDay, RunLimit: s.cfg().Limits.MaxTokensPerRun,
	}, err
}

// metered counts every call's tokens against the day and refuses calls once
// the day's limit is reached. A single call may still go past a limit.
type metered struct {
	inner executor.Model
	s     *Service
}

func (m metered) ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	if lim := m.s.cfg().Limits.MaxTokensPerDay; lim > 0 {
		if l, err := m.s.readUsage(); err == nil && l.Days[m.s.today()] >= lim {
			return llm.Result{}, fmt.Errorf("%w: %d of %d tokens used today (limits.max_tokens_per_day)",
				ErrTokenLimit, l.Days[m.s.today()], lim)
		}
	}
	res, err := m.inner.ChatStream(ctx, req, onDelta)
	if n := res.Usage.TotalTokens; n > 0 {
		if before, after, uerr := m.s.addUsage(n); uerr == nil {
			m.s.usageEvents(before, after)
		}
	}
	return res, err
}

func (s *Service) usageEvents(before, after int) {
	lim := s.cfg().Limits.MaxTokensPerDay
	if lim <= 0 || s.OnRunEvent == nil {
		return
	}
	warn := lim * warnPercent / 100
	switch {
	case before < lim && after >= lim:
		s.OnRunEvent(RunEvent{
			Kind: EventUsageLimit, Time: s.now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			Error: fmt.Sprintf("daily token limit reached: %d of %d", after, lim),
		})
	case before < warn && after >= warn:
		s.OnRunEvent(RunEvent{
			Kind: EventUsageWarning, Time: s.now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			Error: fmt.Sprintf("%d%% of the daily token limit used: %d of %d", warnPercent, after, lim),
		})
	}
}

// runCapped stops a run before its next call once it used limit tokens.
type runCapped struct {
	inner executor.Model
	limit int
	used  atomic.Int64
	name  string
}

func (r *runCapped) ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	if used := int(r.used.Load()); used >= r.limit {
		return llm.Result{}, fmt.Errorf("%w: this run used %d of %d tokens (%s)", ErrTokenLimit, used, r.limit, r.name)
	}
	res, err := r.inner.ChatStream(ctx, req, onDelta)
	r.used.Add(int64(res.Usage.TotalTokens))
	return res, err
}

// capRun applies the run's token limit: the schedule's own, else limits.max_tokens_per_run.
func (s *Service) capRun(m executor.Model, req RunRequest) executor.Model {
	limit, name := s.cfg().Limits.MaxTokensPerRun, "limits.max_tokens_per_run"
	if req.MaxTokens > 0 {
		limit, name = req.MaxTokens, "the schedule's maxTokens"
	}
	if limit <= 0 {
		return m
	}
	return &runCapped{inner: m, limit: limit, name: name}
}

// writeJSONFile writes v atomically (0600 temp file, fsync, rename).
func writeJSONFile(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(append(b, '\n'))
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return errors.Join(fmt.Errorf("write %s: %w", filepath.Base(path), err), os.Remove(name))
	}
	if err := os.Rename(name, path); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", filepath.Base(path), err), os.Remove(name))
	}
	return nil
}
