package scheduler

import (
	"context"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

// handle records the occurrence first (so it never fires twice), then either
// skips it or starts a run in the background.
func (s *Scheduler) handle(ctx context.Context, id string, nb *notebook.Notebook, sch notebook.Schedule,
	occ time.Time, run, missed bool,
) {
	if !run {
		s.skip(id, nb, sch, occ, "missed while Tessivo was not running")
		return
	}
	base := baseEvent(id, nb, sch, occ)
	s.update(nb, sch.ID, occ, nil)
	key := id + "/" + sch.ID
	s.setRunning(key, true)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.setRunning(key, false)
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		case <-ctx.Done():
			return
		}
		s.execute(ctx, nb, sch, base, missed)
	}()
}

func baseEvent(id string, nb *notebook.Notebook, sch notebook.Schedule, occ time.Time) Event {
	return Event{
		NotebookID: id, NotebookName: nb.Manifest.Name, ScheduleID: sch.ID,
		ScheduledFor: occ.UTC().Format(time.RFC3339),
	}
}

// skip records an occurrence that does not run, with the reason.
func (s *Scheduler) skip(id string, nb *notebook.Notebook, sch notebook.Schedule, occ time.Time, reason string) {
	base := baseEvent(id, nb, sch, occ)
	entry := notebook.HistoryEntry{
		ScheduledFor: base.ScheduledFor, StartedAt: s.stamp(), Status: "skipped", Error: reason,
	}
	s.update(nb, sch.ID, occ, &entry)
	base.Kind, base.Error = EventSkipped, reason
	s.emit(base)
}

func (s *Scheduler) execute(ctx context.Context, nb *notebook.Notebook, sch notebook.Schedule, base Event,
	missed bool,
) {
	start := s.Now()
	started := base
	started.Kind = EventStarted
	s.emit(started)
	out, err := s.Svc.Run(ctx, service.RunRequest{
		Notebook: nb, Prompt: sch.Prompt, PromptName: sch.PromptName, Vars: sch.Vars,
		Overrides: sch.Overrides, Save: true, Actor: "scheduler", ScheduleID: sch.ID,
		MaxTokens: sch.MaxTokens,
	}, nil)
	entry := notebook.HistoryEntry{
		ScheduledFor: base.ScheduledFor, StartedAt: start.UTC().Format(time.RFC3339),
		DurationMS: s.Now().Sub(start).Milliseconds(), RunID: out.RunID, OutputID: out.OutputID, Status: "ok",
	}
	if missed {
		entry.Error = "catch-up run for a missed schedule"
	}
	done := base
	done.RunID, done.OutputID = out.RunID, out.OutputID
	switch {
	case err != nil:
		entry.Status, entry.Error = "failed", err.Error()
		done.Kind, done.Error = EventFailed, err.Error()
	case len(out.Paused) > 0:
		entry.Status = notebook.StatusWaiting
		done.Kind = EventPaused
	default:
		done.Kind, done.Title = EventFinished, titleOf(out)
	}
	s.update(nb, sch.ID, time.Time{}, &entry)
	s.emit(done)
}

func titleOf(out service.RunOutcome) string {
	for _, line := range strings.Split(out.Markdown, "\n") {
		if t, ok := strings.CutPrefix(line, "# "); ok {
			return t
		}
	}
	return ""
}

func (s *Scheduler) stamp() string { return s.Now().UTC().Format(time.RFC3339) }

// update writes the schedule state: the handled occurrence and/or a history
// entry. Errors are ignored: the next tick retries from the stored state.
func (s *Scheduler) update(nb *notebook.Notebook, schID string, occ time.Time, entry *notebook.HistoryEntry) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	st, err := nb.LoadScheduleState()
	if err != nil {
		return
	}
	rs := st.Schedules[schID]
	if rs == nil {
		rs = &notebook.ScheduleRunState{}
		st.Schedules[schID] = rs
	}
	if !occ.IsZero() {
		rs.LastScheduled = occ.UTC().Format(time.RFC3339)
	}
	if entry != nil {
		rs.Record(*entry)
	}
	_ = nb.SaveScheduleState(st)
}
