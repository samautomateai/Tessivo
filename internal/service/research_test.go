package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/research"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// counting answers every request with reply and counts the calls.
type counting struct {
	mu      sync.Mutex
	reply   string
	calls   int
	reqs    []llm.Request
	gate    chan struct{} // when set, each call waits for a value
	entered chan struct{} // when set, gets a value as each call starts (before the gate)
}

func (m *counting) ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	if m.entered != nil {
		m.entered <- struct{}{}
	}
	if m.gate != nil {
		select {
		case <-m.gate:
		case <-ctx.Done():
			return llm.Result{}, ctx.Err()
		}
	}
	m.mu.Lock()
	m.calls++
	m.reqs = append(m.reqs, req)
	reply := m.reply
	m.mu.Unlock()
	if onDelta != nil {
		onDelta(reply)
	}
	return llm.Result{Response: llm.Response{Content: reply}, Target: "litellm/m"}, nil
}

func (m *counting) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func researchSvc(t *testing.T) (*Service, *counting) {
	t.Helper()
	s := mcpSvc(t)
	m := &counting{reply: "A short summary of the source."}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	return s, m
}

func writeSource(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreateListDeleteResearch(t *testing.T) {
	s, _ := researchSvc(t)
	info, err := s.CreateResearch("Board Papers", "Q3 board")
	if err != nil || info.ID != "board-papers" {
		t.Fatalf("%v %+v", err, info)
	}
	manifest := filepath.Join(filepath.Dir(s.Workspace), "research", "board-papers", "research.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListResearch(); len(list) != 1 {
		t.Fatalf("list: %+v", list)
	}
	if jobs, _ := s.ListNotebooks(); len(jobs) != 0 {
		t.Fatalf("a research notebook shows up as a job notebook: %+v", jobs)
	}
	if _, err := s.CreateResearch(" ", ""); !IsInvalid(err) {
		t.Fatalf("empty name: %v", err)
	}
	if err := s.DeleteResearch("board-papers"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetResearch(context.Background(), "board-papers"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestAddSourceSummarizesAndSwitchesLevel(t *testing.T) {
	s, m := researchSvc(t)
	nb, _ := s.CreateResearch("Board", "")
	minutes := writeSource(t, "minutes.md", "# Ship in March")
	info, err := s.AddResearchSource(context.Background(), nb.ID, minutes, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	src := info.Sources[0]
	if src.Summary != SummaryReady || src.Level != research.LevelSummary || src.LevelSet || m.count() != 1 {
		t.Fatalf("source %+v calls %d", src, m.count())
	}
	text, summary, err := s.ResearchSource(context.Background(), nb.ID, src.ID)
	if err != nil || !strings.Contains(text, "Ship in March") || summary != m.reply {
		t.Fatalf("%v %q %q", err, text, summary)
	}
}

func TestSummaryNotRepeatedForSameContent(t *testing.T) {
	s, m := researchSvc(t)
	nb, _ := s.CreateResearch("Board", "")
	info, _ := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "a.md", "same"), "", "", true)
	if err := s.RefreshSummary(context.Background(), nb.ID, info.Sources[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if m.count() != 1 {
		t.Fatalf("unchanged content was summarized again: %d calls", m.count())
	}
}

func TestChangedSourceQueuedAndMarkedUpdating(t *testing.T) {
	s, m := researchSvc(t)
	move := movableClock(s)
	nb, _ := s.CreateResearch("Board", "")
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("v1"), 0o600)
	info, _ := s.AddResearchSource(context.Background(), nb.ID, dir, "", "", true)
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("v2 changed"), 0o600)
	move(resummaryAfter)         // past the cooldown that follows the first summary
	m.gate = make(chan struct{}) // hold the re-summary so "updating" is visible
	got, err := s.GetResearch(context.Background(), nb.ID)
	if err != nil || got.Sources[0].Summary != SummaryUpdating {
		t.Fatalf("%v %+v", err, got.Sources)
	}
	m.gate <- struct{}{}
	s.waitSummaries()
	rnb, _ := research.Open(filepath.Join(filepath.Dir(s.Workspace), "research", nb.ID))
	sum, ok, _ := rnb.Summary(info.Sources[0].ID)
	if !ok || m.count() != 2 || sum.Fingerprint == "" {
		t.Fatalf("not re-summarized: %+v calls %d", sum, m.count())
	}
	if got, _ := s.GetResearch(context.Background(), nb.ID); got.Sources[0].Summary != SummaryReady {
		t.Fatalf("after: %+v", got.Sources[0])
	}
}

func TestUserLevelKept(t *testing.T) {
	s, m := researchSvc(t)
	m.gate = make(chan struct{})
	nb, _ := s.CreateResearch("Board", "")
	info, err := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "a.md", "x"), "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSourceLevel(nb.ID, info.Sources[0].ID, research.LevelOff); err != nil {
		t.Fatal(err)
	}
	m.gate <- struct{}{}
	s.waitSummaries()
	got, _ := s.GetResearch(context.Background(), nb.ID)
	if got.Sources[0].Level != research.LevelOff || got.Sources[0].Summary != SummaryReady {
		t.Fatalf("%+v", got.Sources[0])
	}
}

func TestRemovedWhileQueued(t *testing.T) {
	s, m := researchSvc(t)
	var events []RunEvent
	var mu sync.Mutex
	s.OnRunEvent = func(e RunEvent) { mu.Lock(); events = append(events, e); mu.Unlock() }
	m.gate = make(chan struct{})
	nb, _ := s.CreateResearch("Board", "")
	info, _ := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "a.md", "x"), "", "", false)
	if _, err := s.RemoveResearchSource(nb.ID, info.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	close(m.gate)
	s.waitSummaries()
	entries, _ := os.ReadDir(filepath.Join(filepath.Dir(s.Workspace), "research", nb.ID, "summaries"))
	mu.Lock()
	defer mu.Unlock()
	if len(entries) != 0 {
		t.Fatalf("summary written for a removed source: %v", entries)
	}
	for _, e := range events {
		if e.Kind == EventSummaryFailed {
			t.Fatalf("failed event for a removed source: %+v", e)
		}
	}
}

func TestSetLevelSummaryWithoutSummary400(t *testing.T) {
	s, m := researchSvc(t)
	m.gate = make(chan struct{})
	t.Cleanup(func() { close(m.gate); s.waitSummaries() })
	nb, _ := s.CreateResearch("Board", "")
	info, _ := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "a.md", "x"), "", "", false)
	if _, err := s.SetSourceLevel(nb.ID, info.Sources[0].ID, research.LevelSummary); !IsInvalid(err) {
		t.Fatalf("want invalid, got %v", err)
	}
}
