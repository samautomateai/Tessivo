package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// tokenModel answers reply and reports tokens used per call.
type tokenModel struct {
	mu     sync.Mutex
	reply  string
	tokens int
	calls  int
}

func (m *tokenModel) ChatStream(_ context.Context, _ llm.Request, onDelta func(string)) (llm.Result, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if onDelta != nil {
		onDelta(m.reply)
	}
	return llm.Result{
		Response: llm.Response{Content: m.reply, Usage: llm.Usage{TotalTokens: m.tokens}}, Target: "litellm/m",
	}, nil
}

func usageSvc(t *testing.T, tokens int) (*Service, *tokenModel, *[]RunEvent) {
	t.Helper()
	s := newSvc(t, summary)
	m := &tokenModel{reply: summary, tokens: tokens}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	var mu sync.Mutex
	events := &[]RunEvent{}
	s.OnRunEvent = func(e RunEvent) { mu.Lock(); *events = append(*events, e); mu.Unlock() }
	return s, m, events
}

func runOnce(t *testing.T, s *Service, id string, maxTokens int) error {
	t.Helper()
	nb, err := s.OpenNotebook(id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Run(context.Background(), RunRequest{Notebook: nb, Prompt: "Summarize", MaxTokens: maxTokens}, nil)
	return err
}

func TestDailyLimitWarnsThenStops(t *testing.T) {
	s, m, events := usageSvc(t, 400)
	s.Config.Limits.MaxTokensPerDay = 1000
	info, _ := s.CreateNotebook("Ops", "summarize")
	for range 2 {
		if err := runOnce(t, s, info.ID, 0); err != nil {
			t.Fatal(err)
		}
	}
	u, err := s.Usage()
	if err != nil || u.Tokens != 800 || u.DayLimit != 1000 || u.Day != time.Now().Format(time.DateOnly) {
		t.Fatalf("%v %+v", err, u)
	}
	if len(*events) != 1 || (*events)[0].Kind != EventUsageWarning {
		t.Fatalf("want one warning at 80%%: %+v", *events)
	}
	if err := runOnce(t, s, info.ID, 0); err != nil { // 800 < 1000: allowed, ends at 1200
		t.Fatal(err)
	}
	calls := m.calls
	if err := runOnce(t, s, info.ID, 0); !errors.Is(err, ErrTokenLimit) {
		t.Fatalf("over the day limit: %v", err)
	}
	if m.calls != calls {
		t.Fatal("a call was made past the day limit")
	}
	if last := (*events)[len(*events)-1]; last.Kind != EventUsageLimit {
		t.Fatalf("no limit event: %+v", *events)
	}
}

func TestRunLimitStopsTheNextCall(t *testing.T) {
	s, m, _ := usageSvc(t, 600)
	m.reply = "not json" // the pipeline retries, so it would call again
	s.Config.Limits.MaxTokensPerRun = 500
	info, _ := s.CreateNotebook("Ops", "summarize")
	err := runOnce(t, s, info.ID, 0)
	if !errors.Is(err, ErrTokenLimit) || !strings.Contains(err.Error(), "max_tokens_per_run") || m.calls != 1 {
		t.Fatalf("err %v after %d calls", err, m.calls)
	}
	m.calls = 0
	if err := runOnce(t, s, info.ID, 5000); errors.Is(err, ErrTokenLimit) || m.calls < 2 {
		t.Fatalf("a schedule's own limit must replace the run limit: %v after %d calls", err, m.calls)
	}
}

func TestUsageFileIsBoundedAndSurvivesGarbage(t *testing.T) {
	s, _, _ := usageSvc(t, 10)
	for i := range keepUsageDays + 20 {
		day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		s.Now = func() time.Time { return day }
		if _, _, err := s.addUsage(5); err != nil {
			t.Fatal(err)
		}
	}
	l, err := s.readUsage()
	if err != nil || len(l.Days) != keepUsageDays {
		t.Fatalf("%v: %d days kept", err, len(l.Days))
	}
}
