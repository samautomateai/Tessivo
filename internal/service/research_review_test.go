package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/research"
)

// movableClock makes s.Now return the real time plus an offset tests move.
func movableClock(s *Service) func(time.Duration) {
	var off atomic.Int64
	s.Now = func() time.Time { return time.Now().Add(time.Duration(off.Load())) }
	return func(d time.Duration) { off.Add(int64(d)) }
}

// folderSource is a notebook with one summarized folder source holding a.md.
func folderSource(t *testing.T, s *Service) (nbID, srcID, dir string) {
	t.Helper()
	nb, _ := s.CreateResearch("Board", "")
	dir = t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("v1"), 0o600)
	info, err := s.AddResearchSource(context.Background(), nb.ID, dir, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	return nb.ID, info.Sources[0].ID, dir
}

func TestVolatileSourceIsNotSummarizedInALoop(t *testing.T) {
	s, m := researchSvc(t)
	move := movableClock(s)
	nbID, _, dir := folderSource(t, s)
	for i := range 5 { // content changes on every read; the app reloads after every event
		_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("v"+strings.Repeat("x", i+2)), 0o600)
		move(time.Minute)
		if _, err := s.GetResearch(context.Background(), nbID); err != nil {
			t.Fatal(err)
		}
		s.waitSummaries()
	}
	if m.count() > 2 {
		t.Fatalf("a source that changes on every read was summarized %d times in 5 minutes", m.count())
	}
}

func TestContextTotalsUseCachedSizes(t *testing.T) {
	s, _ := researchSvc(t)
	nbID, srcID, dir := folderSource(t, s)
	if _, err := s.SetSourceLevel(nbID, srcID, research.LevelFull); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	tot, err := s.ResearchContext(context.Background(), nbID)
	if err != nil || tot.Sources != 1 || tot.Tokens == 0 {
		t.Fatalf("context re-read the source instead of using its cached size: %v %+v", err, tot)
	}
}

func TestMissingSourceIsUnreadableAndSkipped(t *testing.T) {
	s, m := researchSvc(t)
	nbID, _, dir := folderSource(t, s)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	info, err := s.GetResearch(context.Background(), nbID)
	if err != nil || info.Sources[0].Summary != SummaryUnreadable || info.Sources[0].Error == "" {
		t.Fatalf("%v %+v", err, info.Sources)
	}
	chat, _ := s.CreateResearchChat(nbID, "")
	m.reply = "Nothing to go on."
	r, err := s.SendResearch(context.Background(), nbID, chat.ID, "What is in it?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Skipped, filepath.Base(dir)) || strings.Contains(allText(m), "A short summary") {
		t.Fatalf("skipped %v; prompt %q", r.Skipped, allText(m))
	}
}

func TestSendQueuesAChangedSummarySource(t *testing.T) {
	s, m := researchSvc(t)
	move := movableClock(s)
	nbID, srcID, dir := folderSource(t, s)
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("v2 changed"), 0o600)
	move(10 * time.Minute)
	chat, _ := s.CreateResearchChat(nbID, "")
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "q", nil); err != nil {
		t.Fatal(err)
	}
	s.waitSummaries()
	if m.count() != 3 { // first summary, the answer, the re-summary
		t.Fatalf("calls %d", m.count())
	}
	rnb, _ := research.Open(filepath.Join(filepath.Dir(s.Workspace), "research", nbID))
	sum, _, _ := rnb.Summary(srcID)
	want, _ := s.GetResearch(context.Background(), nbID)
	if want.Sources[0].Summary != SummaryReady || sum.Fingerprint == "" {
		t.Fatalf("%+v %+v", want.Sources[0], sum)
	}
}

func TestSummaryOfAHugeSourceMakesABoundedNumberOfCalls(t *testing.T) {
	s, m := researchSvc(t)
	small := 2000
	s.Config.Defaults.ContextBudget = &small
	nb, _ := s.CreateResearch("Big", "")
	dir := t.TempDir()
	for i := range 6 {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("big%d.md", i)), []byte(strings.Repeat("word ", 40_000)), 0o600)
	}
	if _, err := s.AddResearchSource(context.Background(), nb.ID, dir, "", "", true); err != nil {
		t.Fatal(err)
	}
	if m.count() > maxSummaryPieces+1 {
		t.Fatalf("one summary made %d model calls", m.count())
	}
}
