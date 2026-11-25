package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/watch"
)

// EventSourcesChanged is the event kind a watcher publishes (SRC-8).
const EventSourcesChanged = "sources.changed"

// DefaultWatchInterval is how often tess serve checks watched sources.
const DefaultWatchInterval = 60 * time.Second

// SourcesChanged reports that a watching notebook's sources changed and settled.
type SourcesChanged struct {
	Kind         string   `json:"kind"` // EventSourcesChanged
	NotebookID   string   `json:"notebookId"`
	NotebookName string   `json:"notebookName"`
	Sources      []string `json:"sources"`         // labels of the changed sources
	Ran          bool     `json:"ran,omitempty"`   // run mode: a run was started
	Error        string   `json:"error,omitempty"` // run mode: why no run was started
	Time         string   `json:"time"`
}

// Watcher checks the in-place file and folder sources of notebooks whose
// watch mode is notify or run. It reads metadata only (watch.Fingerprint), so
// it works the same on every OS. A change is reported once two checks in a row
// agree, so a copy in progress is not reported half done.
type Watcher struct {
	Svc      *Service
	Interval time.Duration // 0: DefaultWatchInterval
	// OnChange receives each settled change; nil: none.
	OnChange func(SourcesChanged)
	// Start starts a run for a notebook in run mode; nil: a background run with
	// the notebook's first schedule's prompt, else the work type's starter.
	Start func(ctx context.Context, notebookID string) error

	mu      sync.Mutex
	sources map[string]*watchState // notebook ID + "/" + source ID
	running map[string]bool        // notebooks with a run started by the default Start
}

type watchState struct {
	base, pending string
	failing       bool // the last check could not read it (logged once)
}

// Run checks every Interval until ctx ends. The first check records the baseline.
func (w *Watcher) Run(ctx context.Context) {
	every := w.Interval
	if every <= 0 {
		every = DefaultWatchInterval
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		w.Check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Check makes one pass over the watching notebooks.
func (w *Watcher) Check(ctx context.Context) {
	list, err := w.Svc.ListNotebooks()
	if err != nil {
		slog.Warn("watch: list notebooks", "err", err)
		return
	}
	w.mu.Lock()
	if w.sources == nil {
		w.sources = map[string]*watchState{}
	}
	seen := map[string]bool{}
	var changes []SourcesChanged
	for _, info := range list {
		if info.Watch == "" || info.Archived || info.nb == nil {
			continue
		}
		if labels := w.checkNotebook(info, seen); len(labels) > 0 {
			changes = append(changes, SourcesChanged{
				Kind: EventSourcesChanged, NotebookID: info.ID, NotebookName: info.Name, Sources: labels,
				Time: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
	for key := range w.sources {
		if !seen[key] {
			delete(w.sources, key) // no longer watched
		}
	}
	w.mu.Unlock()
	for _, c := range changes {
		w.report(ctx, c)
	}
}

// checkNotebook fingerprints a notebook's sources and returns the labels of
// those whose change has settled. w.mu is held.
func (w *Watcher) checkNotebook(info NotebookInfo, seen map[string]bool) []string {
	var labels []string
	for _, c := range info.nb.Manifest.Context {
		path := info.nb.ContextPath(c)
		if !c.Enabled || c.NeedsRelink || path == "" || !c.InPlace { // copies never change
			continue
		}
		key := info.ID + "/" + c.ID
		seen[key] = true
		if w.step(key, path, c.Label) {
			labels = append(labels, c.Label)
		}
	}
	return labels
}

// step advances one source's state and reports whether its change settled.
func (w *Watcher) step(key, path, label string) bool {
	fp, err := watch.Fingerprint(path)
	st := w.sources[key]
	if err != nil {
		if st == nil {
			st = &watchState{}
			w.sources[key] = st
		}
		if !st.failing {
			slog.Warn("watch: cannot read a source", "source", label, "err", err)
		}
		st.failing, st.pending = true, ""
		return false
	}
	switch {
	case st == nil:
		w.sources[key] = &watchState{base: fp}
	case st.base == "": // unreadable from the start: this is the baseline
		st.base, st.failing = fp, false
	case fp == st.base:
		st.pending, st.failing = "", false
	case fp != st.pending:
		st.pending, st.failing = fp, false // changed: wait for one quiet check
	default:
		st.base, st.pending = fp, ""
		return true
	}
	return false
}

// report publishes a change and, in run mode, starts a run unless the
// notebook is busy.
func (w *Watcher) report(ctx context.Context, c SourcesChanged) {
	nb, err := w.Svc.OpenNotebook(c.NotebookID)
	if err == nil && nb.Manifest.Watch == notebook.WatchRun {
		switch {
		case nb.Busy() || w.isRunning(c.NotebookID):
			c.Error = "the notebook is busy; no run started"
		default:
			if err := w.start(ctx, c.NotebookID); err != nil {
				c.Error = err.Error()
			} else {
				c.Ran = true
			}
		}
	}
	if w.OnChange != nil {
		w.OnChange(c)
	}
}

func (w *Watcher) isRunning(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running[id]
}

func (w *Watcher) start(ctx context.Context, id string) error {
	if w.Start != nil {
		return w.Start(ctx, id)
	}
	nb, err := w.Svc.OpenNotebook(id)
	if err != nil {
		return err
	}
	req := RunRequest{Notebook: nb, Save: true, Actor: "watch"}
	if schedules, err := nb.Schedules(); err == nil && len(schedules) > 0 {
		req.Prompt, req.PromptName, req.Vars = schedules[0].Prompt, schedules[0].PromptName, schedules[0].Vars
	}
	w.mu.Lock()
	if w.running == nil {
		w.running = map[string]bool{}
	}
	w.running[id] = true
	w.mu.Unlock()
	go func() {
		defer func() {
			w.mu.Lock()
			delete(w.running, id)
			w.mu.Unlock()
		}()
		if _, err := w.Svc.Run(ctx, req, nil); err != nil {
			slog.Warn("watch: run failed", "notebook", id, "err", err)
		}
	}()
	return nil
}
