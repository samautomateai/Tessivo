// Package scheduler runs notebook schedules while the core is running. It
// never overlaps two runs of the same schedule, bounds concurrent runs,
// applies each schedule's missed-run policy after downtime, records history
// and reports events (for desktop notifications).
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/cron"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

// Event kinds.
const (
	EventStarted  = "schedule.started"
	EventFinished = "schedule.finished"
	EventFailed   = "schedule.failed"
	EventSkipped  = "schedule.skipped"
	EventPaused   = "schedule.paused" // an agent run is waiting for approval
)

// Event describes something the scheduler did.
type Event struct {
	Kind         string `json:"kind"`
	NotebookID   string `json:"notebookId"`
	NotebookName string `json:"notebookName"`
	ScheduleID   string `json:"scheduleId"`
	ScheduledFor string `json:"scheduledFor"`
	RunID        string `json:"runId,omitempty"`
	OutputID     string `json:"outputId,omitempty"`
	Title        string `json:"title,omitempty"`
	Error        string `json:"error,omitempty"`
	Time         string `json:"time"`
}

// Scheduler checks schedules every Interval.
type Scheduler struct {
	Svc           *service.Service
	Interval      time.Duration // default 30s
	MaxConcurrent int           // default 2
	OnEvent       func(Event)
	Now           func() time.Time
	Location      *time.Location // for expressions without CRON_TZ; default Local

	mu      sync.Mutex
	running map[string]bool // notebookID/scheduleID
	stateMu sync.Mutex      // serialises state file writes
	sem     chan struct{}
	wg      sync.WaitGroup
}

func (s *Scheduler) init() {
	if s.Interval <= 0 {
		s.Interval = 30 * time.Second
	}
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = 2
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Location == nil {
		s.Location = time.Local
	}
	s.running = map[string]bool{}
	s.sem = make(chan struct{}, s.MaxConcurrent)
}

// Run checks schedules until ctx is cancelled, then waits for in-flight runs
// (which see the cancelled context and stop).
func (s *Scheduler) Run(ctx context.Context) error {
	s.init()
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		s.Tick(ctx)
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return nil
		case <-t.C:
		}
	}
}

// Tick runs one check across the workspace. Exported for tests.
func (s *Scheduler) Tick(ctx context.Context) {
	if s.running == nil {
		s.init()
	}
	nbs, err := s.Svc.ListNotebooks()
	if err != nil {
		return
	}
	now := s.Now()
	for _, info := range nbs {
		nb, err := s.Svc.OpenNotebook(info.ID)
		if err != nil {
			continue
		}
		s.checkNotebook(ctx, info.ID, nb, now)
	}
}

// Wait blocks until in-flight runs finish (for tests).
func (s *Scheduler) Wait() { s.wg.Wait() }

func (s *Scheduler) emit(e Event) {
	e.Time = s.Now().UTC().Format(time.RFC3339)
	if s.OnEvent != nil {
		s.OnEvent(e)
	}
}

// due decides what to do with a schedule now. It returns the occurrence to
// record (zero: nothing due), whether to run, and whether runs were missed.
func (s *Scheduler) due(c *cron.Schedule, sch notebook.Schedule, st *notebook.ScheduleRunState, now time.Time,
) (occurrence time.Time, run, missed bool) {
	since, err := time.Parse(time.RFC3339, st.LastScheduled)
	if err != nil {
		if since, err = time.Parse(time.RFC3339, sch.CreatedAt); err != nil {
			since = now.Add(-s.Interval)
		}
	}
	next, err := c.Next(since)
	if err != nil || next.After(now) {
		return time.Time{}, false, false
	}
	latest, count := next, 1
	for count < 100_000 { // find the latest occurrence <= now (bounded)
		n, err := c.Next(latest)
		if err != nil || n.After(now) {
			break
		}
		latest, count = n, count+1
	}
	grace := 2 * s.Interval
	missed = now.Sub(next) > grace
	if !missed {
		return latest, true, false
	}
	return latest, sch.Missed == notebook.MissedRunOnce, true
}

func (s *Scheduler) checkNotebook(ctx context.Context, id string, nb *notebook.Notebook, now time.Time) {
	list, err := nb.Schedules()
	if err != nil {
		return
	}
	s.stateMu.Lock()
	st, err := nb.LoadScheduleState()
	s.stateMu.Unlock()
	if err != nil {
		return
	}
	for _, sch := range list {
		if !sch.Enabled {
			continue
		}
		c, err := cron.Parse(sch.Cron, s.Location)
		if err != nil {
			continue
		}
		rs := st.Schedules[sch.ID]
		if rs == nil {
			rs = &notebook.ScheduleRunState{}
		}
		occ, run, missed := s.due(c, sch, rs, now)
		if occ.IsZero() || s.isRunning(id, sch.ID) {
			continue
		}
		if s.Svc.PendingForSchedule(id, sch.ID) {
			s.skip(id, nb, sch, occ, "previous run awaiting approval")
			continue
		}
		s.handle(ctx, id, nb, sch, occ, run, missed)
	}
}

func (s *Scheduler) isRunning(nbID, schID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[nbID+"/"+schID]
}

func (s *Scheduler) setRunning(key string, v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v {
		s.running[key] = true
	} else {
		delete(s.running, key)
	}
}
