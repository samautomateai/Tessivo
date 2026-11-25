package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// watched makes a notebook watching a folder source in the given mode.
func watched(t *testing.T, s *Service, mode string) (NotebookInfo, string) {
	t.Helper()
	info, _ := s.CreateNotebook("Ops", "summarize")
	folder := t.TempDir()
	_ = os.WriteFile(filepath.Join(folder, "a.md"), []byte("one"), 0o600)
	if _, err := s.AddContext(info.ID, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateNotebook(info.ID, NotebookUpdate{Watch: &mode}); err != nil {
		t.Fatal(err)
	}
	return info, folder
}

func TestWatcherNotifiesOnceTheChangeSettles(t *testing.T) {
	s := newSvc(t, summary)
	info, folder := watched(t, s, notebook.WatchNotify)
	var events []SourcesChanged
	w := &Watcher{Svc: s, OnChange: func(e SourcesChanged) { events = append(events, e) }}
	ctx := context.Background()
	w.Check(ctx) // baseline
	w.Check(ctx)
	if len(events) != 0 {
		t.Fatal("nothing changed yet")
	}
	_ = os.WriteFile(filepath.Join(folder, "b.md"), []byte("two"), 0o600)
	w.Check(ctx) // sees the change, waits
	if len(events) != 0 {
		t.Fatal("must wait for the change to settle")
	}
	w.Check(ctx) // quiet: report
	if len(events) != 1 || events[0].NotebookID != info.ID || events[0].Ran || len(events[0].Sources) != 1 {
		t.Fatalf("%+v", events)
	}
	w.Check(ctx)
	if len(events) != 1 {
		t.Fatal("a change is reported once")
	}
}

func TestWatcherRunsInRunModeUnlessBusy(t *testing.T) {
	s := newSvc(t, summary)
	info, folder := watched(t, s, notebook.WatchRun)
	var ran []string
	w := &Watcher{Svc: s, Start: func(_ context.Context, id string) error { ran = append(ran, id); return nil }}
	ctx := context.Background()
	w.Check(ctx)
	_ = os.WriteFile(filepath.Join(folder, "b.md"), []byte("two"), 0o600)
	w.Check(ctx)
	w.Check(ctx)
	if len(ran) != 1 || ran[0] != info.ID {
		t.Fatalf("run mode starts a run: %v", ran)
	}
	nb := mustOpen(t, s, info.ID)
	release, err := nb.LockRun("r-busy")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_ = os.WriteFile(filepath.Join(folder, "c.md"), []byte("three"), 0o600)
	w.Check(ctx)
	w.Check(ctx)
	if len(ran) != 1 {
		t.Fatal("a busy notebook is not run again")
	}
}

func TestWatcherToleratesAMissingFolderAndOffMode(t *testing.T) {
	s := newSvc(t, summary)
	_, folder := watched(t, s, notebook.WatchNotify)
	var events []SourcesChanged
	w := &Watcher{Svc: s, OnChange: func(e SourcesChanged) { events = append(events, e) }}
	ctx := context.Background()
	w.Check(ctx)
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	w.Check(ctx)
	w.Check(ctx)
	if len(events) != 0 {
		t.Fatalf("a missing folder is not a change: %+v", events)
	}
	off := notebook.WatchOff
	bad := "sometimes"
	info, _ := s.CreateNotebook("Other", "summarize")
	if _, err := s.UpdateNotebook(info.ID, NotebookUpdate{Watch: &bad}); !IsInvalid(err) {
		t.Fatalf("bad mode: %v", err)
	}
	if got, err := s.UpdateNotebook(info.ID, NotebookUpdate{Watch: &off}); err != nil || got.Watch != "" {
		t.Fatalf("off: %v %+v", err, got)
	}
}
