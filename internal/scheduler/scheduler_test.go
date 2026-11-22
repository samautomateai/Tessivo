package scheduler

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

const summary = `{"title":"Weekly digest","tldr":"t","key_points":["k"],"per_source":[],"action_items":[]}`

type model struct {
	reply string
	err   error
	block chan struct{}
	mu    sync.Mutex
	calls int
}

func (m *model) ChatStream(ctx context.Context, _ llm.Request, _ func(string)) (llm.Result, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.block != nil {
		select {
		case <-m.block:
		case <-ctx.Done():
			return llm.Result{}, ctx.Err()
		}
	}
	if m.err != nil {
		return llm.Result{}, m.err
	}
	return llm.Result{Response: llm.Response{Content: m.reply}, Target: "litellm/m"}, nil
}

func (m *model) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type harness struct {
	sched  *Scheduler
	svc    *service.Service
	model  *model
	now    time.Time
	events []Event
	mu     sync.Mutex
	nbID   string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{model: &model{reply: summary}, now: time.Date(2026, 11, 23, 7, 59, 0, 0, time.UTC)}
	f := config.Default()
	f.Defaults.Model = "litellm/m"
	dir := t.TempDir()
	h.svc = &service.Service{
		Config: f, Secrets: secrets.Map{}, Workspace: filepath.Join(dir, "ws"),
		UserAuditPath: filepath.Join(dir, "audit.jsonl"),
		NewModel: func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
			return h.model, nil
		},
	}
	info, err := h.svc.CreateNotebook("Ops", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	h.nbID = info.ID
	h.sched = &Scheduler{
		Svc: h.svc, Location: time.UTC, Interval: 30 * time.Second,
		Now:     func() time.Time { h.mu.Lock(); defer h.mu.Unlock(); return h.now },
		OnEvent: func(e Event) { h.mu.Lock(); h.events = append(h.events, e); h.mu.Unlock() },
	}
	return h
}

func (h *harness) set(t time.Time) { h.mu.Lock(); h.now = t; h.mu.Unlock() }

func (h *harness) kinds() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.events))
	for _, e := range h.events {
		out = append(out, e.Kind)
	}
	return out
}

func (h *harness) addSchedule(t *testing.T, cronSpec, missed string, created time.Time) string {
	t.Helper()
	nb, _ := h.svc.OpenNotebook(h.nbID)
	list, _ := nb.Schedules()
	sch := notebook.Schedule{
		ID: "s" + cronSpec[:1] + missed, Cron: cronSpec, Missed: missed, Enabled: true,
		CreatedAt: created.Format(time.RFC3339),
	}
	if err := nb.SaveSchedules(append(list, sch)); err != nil {
		t.Fatal(err)
	}
	return sch.ID
}

func (h *harness) history(t *testing.T, id string) []notebook.HistoryEntry {
	t.Helper()
	nb, _ := h.svc.OpenNotebook(h.nbID)
	st, err := nb.LoadScheduleState()
	if err != nil || st.Schedules[id] == nil {
		return nil
	}
	return st.Schedules[id].History
}

func TestRunsWhenDueAndOnlyOnce(t *testing.T) {
	h := newHarness(t)
	id := h.addSchedule(t, "0 8 * * 1", notebook.MissedSkip, h.now.Add(-time.Hour))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if h.model.count() != 0 {
		t.Fatal("not due yet")
	}
	h.set(time.Date(2026, 11, 23, 8, 0, 10, 0, time.UTC))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	h.sched.Tick(context.Background()) // same minute: must not run again
	h.sched.Wait()
	if h.model.count() != 1 {
		t.Fatalf("want 1 run, got %d", h.model.count())
	}
	hist := h.history(t, id)
	if len(hist) != 1 || hist[0].Status != "ok" || hist[0].OutputID == "" {
		t.Fatalf("history: %+v", hist)
	}
	if got := h.kinds(); len(got) != 2 || got[0] != EventStarted || got[1] != EventFinished {
		t.Fatalf("events: %v", got)
	}
	if h.events[1].Title != "Weekly digest" || h.events[1].NotebookName != "Ops" {
		t.Fatalf("finished event: %+v", h.events[1])
	}
}

func TestMissedPolicies(t *testing.T) {
	h := newHarness(t)
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	skip := h.addSchedule(t, "0 8 * * *", notebook.MissedSkip, created)
	once := h.addSchedule(t, "30 8 * * *", notebook.MissedRunOnce, created)
	h.set(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)) // core was down for weeks
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if h.model.count() != 1 {
		t.Fatalf("only the run_once schedule should catch up, got %d runs", h.model.count())
	}
	if hs := h.history(t, skip); len(hs) != 1 || hs[0].Status != "skipped" {
		t.Fatalf("skip history: %+v", hs)
	}
	if ho := h.history(t, once); len(ho) != 1 || ho[0].Status != "ok" || ho[0].Error == "" {
		t.Fatalf("run_once history: %+v", ho)
	}
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if h.model.count() != 1 {
		t.Fatal("caught-up schedules must not run again until the next occurrence")
	}
}

func TestFailureDisabledAndNoOverlap(t *testing.T) {
	h := newHarness(t)
	h.model.err = errors.New("provider down")
	id := h.addSchedule(t, "* * * * *", notebook.MissedSkip, h.now)
	nb, _ := h.svc.OpenNotebook(h.nbID)
	list, _ := nb.Schedules()
	list = append(list, notebook.Schedule{
		ID: "off", Cron: "* * * * *", Missed: notebook.MissedSkip,
		CreatedAt: h.now.Add(-time.Hour).Format(time.RFC3339),
	})
	_ = nb.SaveSchedules(list)

	h.set(h.now.Add(time.Minute + time.Second))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if hs := h.history(t, id); len(hs) != 1 || hs[0].Status != "failed" {
		t.Fatalf("history: %+v", hs)
	}
	if h.history(t, "off") != nil {
		t.Fatal("disabled schedule ran")
	}

	h.model.err, h.model.block = nil, make(chan struct{})
	h.set(h.now.Add(time.Minute))
	h.sched.Tick(context.Background())
	for h.model.count() < 2 {
		time.Sleep(time.Millisecond)
	}
	h.set(h.now.Add(time.Minute))
	h.sched.Tick(context.Background()) // previous run still going: no overlap
	close(h.model.block)
	h.sched.Wait()
	if h.model.count() != 2 {
		t.Fatalf("overlapping run started: %d calls", h.model.count())
	}
}

func TestRunLoopStopsOnCancel(t *testing.T) {
	h := newHarness(t)
	h.sched.Interval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.sched.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	var zero Scheduler
	zero.init()
	if zero.Interval != 30*time.Second || zero.MaxConcurrent != 2 || zero.Location != time.Local {
		t.Fatal("defaults")
	}
}

// queue answers from a list of replies.
type queue struct {
	mu      sync.Mutex
	replies []llm.Response
}

func (q *queue) ChatStream(context.Context, llm.Request, func(string)) (llm.Result, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.replies) == 0 {
		return llm.Result{}, errors.New("no reply queued")
	}
	r := q.replies[0]
	q.replies = q.replies[1:]
	return llm.Result{Response: r, Target: "litellm/m"}, nil
}

// agentSchedule makes an every-minute agent schedule whose first run asks to
// use grep (rule: ask), with a code folder to search.
func agentSchedule(t *testing.T, h *harness) string {
	t.Helper()
	q := &queue{replies: []llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "grep", Arguments: `{"pattern":"x"}`}}},
		{Content: "done"},
		{Content: summary},
	}}
	h.svc.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return q, nil }
	h.svc.Config.Agent.Tools = map[string]string{"grep": "ask"}
	src := t.TempDir()
	if _, err := h.svc.AddContext(h.nbID, src); err != nil {
		t.Fatal(err)
	}
	nb, _ := h.svc.OpenNotebook(h.nbID)
	sch := notebook.Schedule{
		ID: "agent", Cron: "* * * * *", Missed: notebook.MissedSkip, Enabled: true,
		CreatedAt: h.now.Format(time.RFC3339),
		Overrides: config.RunSettings{Mode: config.ModeAgent},
	}
	if err := nb.SaveSchedules([]notebook.Schedule{sch}); err != nil {
		t.Fatal(err)
	}
	return sch.ID
}

func TestScheduledAgentRunPauses(t *testing.T) {
	h := newHarness(t)
	id := agentSchedule(t, h)
	h.set(h.now.Add(70 * time.Second))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if got := h.kinds(); len(got) != 2 || got[1] != EventPaused {
		t.Fatalf("events: %v", got)
	}
	hist := h.history(t, id)
	if len(hist) != 1 || hist[0].Status != "waiting_approval" || hist[0].RunID == "" {
		t.Fatalf("history: %+v", hist)
	}
}

func TestSkipWhilePending(t *testing.T) {
	h := newHarness(t)
	id := agentSchedule(t, h)
	h.set(h.now.Add(70 * time.Second))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	h.set(h.now.Add(60 * time.Second))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	got := h.kinds()
	if len(got) != 3 || got[2] != EventSkipped || h.events[2].Error != "previous run awaiting approval" {
		t.Fatalf("events: %v (%+v)", got, h.events)
	}
	pending, err := h.svc.Pending()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	if err := h.svc.Answer(pending[0].RunID, pending[0].ApprovalID, "allow", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Resume(context.Background(), pending[0].RunID, nil); err != nil {
		t.Fatal(err)
	}
	h.svc.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
		return &queue{replies: []llm.Response{{Content: "done"}, {Content: summary}}}, nil
	}
	h.set(h.now.Add(60 * time.Second))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if got := h.kinds(); got[len(got)-1] != EventFinished {
		t.Fatalf("after the approval the next occurrence must run: %v", got)
	}
	if hist := h.history(t, id); len(hist) != 3 {
		t.Fatalf("history: %+v", hist)
	}
}

func TestArchivedNotebooksDoNotRun(t *testing.T) {
	h := newHarness(t)
	h.addSchedule(t, "0 8 * * 1", notebook.MissedSkip, h.now.Add(-time.Hour))
	archived := true
	if _, err := h.svc.UpdateNotebook(h.nbID, service.NotebookUpdate{Archived: &archived}); err != nil {
		t.Fatal(err)
	}
	h.set(time.Date(2026, 11, 23, 8, 0, 10, 0, time.UTC))
	h.sched.Tick(context.Background())
	h.sched.Wait()
	if h.model.count() != 0 || len(h.kinds()) != 0 {
		t.Fatalf("an archived notebook ran: %d runs, events %v", h.model.count(), h.kinds())
	}
}
