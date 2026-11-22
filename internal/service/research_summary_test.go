package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
	"github.com/samautomateai/tessivo/internal/research"
)

func TestSourceChangedDuringSummary(t *testing.T) {
	s, m := researchSvc(t)
	m.gate, m.entered = make(chan struct{}), make(chan struct{}, 1)
	nb, _ := s.CreateResearch("Board", "")
	path := writeSource(t, "a.md", "before")
	info, _ := s.AddResearchSource(context.Background(), nb.ID, path, "", "", false)
	<-m.entered // the source was read and fingerprinted; the model call is waiting
	// The copy inside the notebook is what is read; change it while the model works.
	rnb, _ := research.Open(filepath.Join(filepath.Dir(s.Workspace), "research", nb.ID))
	src, _, _ := rnb.Find(info.Sources[0].ID)
	before, _ := research.FileFingerprint(rnb.SourcePath(src))
	_ = os.WriteFile(rnb.SourcePath(src), []byte("after"), 0o600)
	m.gate <- struct{}{}
	s.waitSummaries()
	sum, ok, _ := rnb.Summary(src.ID)
	if !ok || sum.Fingerprint != before {
		t.Fatalf("stored fingerprint %q, want the one read before the call (%q)", sum.Fingerprint, before)
	}
}

func TestRunnerQueuesPendingOnStart(t *testing.T) {
	s, m := researchSvc(t)
	nb, _ := s.CreateResearch("Board", "")
	rnb, _ := research.Open(filepath.Join(filepath.Dir(s.Workspace), "research", nb.ID))
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if _, err := rnb.AddPath(writeSource(t, name, name)); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	running, peak := 0, 0
	release := make(chan struct{})
	m.gate = release
	s.OnRunEvent = func(e RunEvent) {
		mu.Lock()
		defer mu.Unlock()
		switch e.Kind {
		case EventSummaryStarted:
			running++
			peak = max(peak, running)
		case EventSummaryReady, EventSummaryFailed:
			running--
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartSummaries(ctx)
	go func() {
		for range 3 {
			time.Sleep(50 * time.Millisecond)
			release <- struct{}{}
		}
	}()
	s.waitSummaries()
	mu.Lock()
	defer mu.Unlock()
	if m.count() != 3 || peak > maxSummaryJobs {
		t.Fatalf("calls %d, peak %d", m.count(), peak)
	}
}

func TestSummaryEventsCarrySource(t *testing.T) {
	s, _ := researchSvc(t)
	var got []RunEvent
	var mu sync.Mutex
	s.OnRunEvent = func(e RunEvent) { mu.Lock(); got = append(got, e); mu.Unlock() }
	nb, _ := s.CreateResearch("Board", "")
	info, _ := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "a.md", "x"), "", "", true)
	mu.Lock()
	defer mu.Unlock()
	last := got[len(got)-1]
	if last.Kind != EventSummaryReady || last.NotebookID != nb.ID || last.SourceID != info.Sources[0].ID {
		t.Fatalf("%+v", got)
	}
}

func TestMCPResourceSource(t *testing.T) {
	s, _ := researchSvc(t)
	if _, err := s.AddMCPServer(fakeServer(t, "fake", mcptest.ModeOK)); err != nil {
		t.Fatal(err)
	}
	nb, _ := s.CreateResearch("Board", "")
	info, err := s.AddResearchSource(context.Background(), nb.ID, "", "fake", "file:///notes.md", true)
	if err != nil || info.Sources[0].Summary != SummaryReady {
		t.Fatalf("%v %+v", err, info.Sources)
	}
	text, _, err := s.ResearchSource(context.Background(), nb.ID, info.Sources[0].ID)
	if err != nil || !strings.Contains(text, "Ship in March") {
		t.Fatalf("%v %q", err, text)
	}
	if err := s.RemoveMCPServer("fake"); !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "Board") {
		t.Fatalf("remove: %v", err)
	}
	if _, err := s.AddResearchSource(context.Background(), nb.ID, "", "nope", "x", true); !IsInvalid(err) {
		t.Fatalf("unknown server: %v", err)
	}
}

func TestContextTotals(t *testing.T) {
	s, _ := researchSvc(t)
	nb, _ := s.CreateResearch("Board", "")
	full := writeSource(t, "full.md", strings.Repeat("full ", 400))
	_, _ = s.AddResearchSource(context.Background(), nb.ID, full, "", "",
		true)
	info, _ := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "sum.md", "summed"), "", "", true)
	// the first source was switched to summary too; put it back to full
	if _, err := s.SetSourceLevel(nb.ID, info.Sources[0].ID, research.LevelFull); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveResearchNote(nb.ID, research.Note{Title: "n", Text: "note text", Include: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveResearchNote(nb.ID, research.Note{Title: "x", Text: "excluded"}); err != nil {
		t.Fatal(err)
	}
	tot, err := s.ResearchContext(context.Background(), nb.ID)
	if err != nil || tot.Sources != 2 || tot.Summaries != 1 || tot.Notes != 1 || tot.Tokens < 400 {
		t.Fatalf("%v %+v", err, tot)
	}
}
