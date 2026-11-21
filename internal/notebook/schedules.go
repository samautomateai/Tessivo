package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/cron"
)

// Schedule files. Definitions are edited by users; state is written only by
// the scheduler, so the two never overwrite each other.
const (
	SchedulesFile     = "schedules.json"
	ScheduleStateFile = "schedule-state.json"
)

// Missed-run policies.
const (
	MissedSkip    = "skip"
	MissedRunOnce = "run_once"
)

// MaxSchedules per notebook and MaxHistory entries kept per schedule.
const (
	MaxSchedules = 50
	MaxHistory   = 50
)

// Schedule runs a notebook prompt on a cron expression while the core runs.
type Schedule struct {
	ID         string             `json:"id"`
	Cron       string             `json:"cron"`
	Prompt     string             `json:"prompt,omitempty"`
	PromptName string             `json:"promptName,omitempty"`
	Vars       map[string]string  `json:"vars,omitempty"`
	Overrides  config.RunSettings `json:"overrides"`
	Missed     string             `json:"missed"`
	Enabled    bool               `json:"enabled"`
	CreatedAt  string             `json:"createdAt"`
}

// Validate checks the expression, policy and settings.
func (s Schedule) Validate() error {
	if _, err := cron.Parse(s.Cron, time.Local); err != nil {
		return err
	}
	if s.Missed != MissedSkip && s.Missed != MissedRunOnce {
		return fmt.Errorf("missed policy %q: want skip or run_once", s.Missed)
	}
	if s.PromptName != "" && !safeName.MatchString(s.PromptName) {
		return fmt.Errorf("prompt name %q is not valid", s.PromptName)
	}
	return s.Overrides.Validate()
}

// HistoryEntry is one scheduled run's outcome.
type HistoryEntry struct {
	ScheduledFor string `json:"scheduledFor"`
	StartedAt    string `json:"startedAt"`
	DurationMS   int64  `json:"durationMs"`
	Status       string `json:"status"` // ok, failed, skipped
	RunID        string `json:"runId,omitempty"`
	OutputID     string `json:"outputId,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ScheduleRunState tracks one schedule.
type ScheduleRunState struct {
	LastScheduled string         `json:"lastScheduled,omitempty"` // latest occurrence handled
	History       []HistoryEntry `json:"history"`                 // newest first
}

// ScheduleState is the scheduler's per-notebook state.
type ScheduleState struct {
	Schedules map[string]*ScheduleRunState `json:"schedules"`
}

// Record prepends a history entry, keeping at most MaxHistory.
func (s *ScheduleRunState) Record(e HistoryEntry) {
	s.History = append([]HistoryEntry{e}, s.History...)
	if len(s.History) > MaxHistory {
		s.History = s.History[:MaxHistory]
	}
}

// Schedules returns the notebook's schedules (empty if none).
func (n *Notebook) Schedules() ([]Schedule, error) {
	var out []Schedule
	err := readJSON(filepath.Join(n.Dir, SchedulesFile), &out)
	if errors.Is(err, os.ErrNotExist) {
		return []Schedule{}, nil
	}
	if out == nil {
		out = []Schedule{}
	}
	return out, err
}

// SaveSchedules validates and writes all schedules.
func (n *Notebook) SaveSchedules(list []Schedule) error {
	if len(list) > MaxSchedules {
		return fmt.Errorf("at most %d schedules per notebook", MaxSchedules)
	}
	seen := map[string]bool{}
	for _, s := range list {
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("schedule ids must be unique and non-empty")
		}
		seen[s.ID] = true
		if err := s.Validate(); err != nil {
			return fmt.Errorf("schedule %s: %w", s.ID, err)
		}
	}
	return writeJSON(filepath.Join(n.Dir, SchedulesFile), list)
}

// LoadScheduleState reads the scheduler state (empty if none).
func (n *Notebook) LoadScheduleState() (ScheduleState, error) {
	st := ScheduleState{}
	err := readJSON(filepath.Join(n.Dir, ScheduleStateFile), &st)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if st.Schedules == nil {
		st.Schedules = map[string]*ScheduleRunState{}
	}
	return st, err
}

// SaveScheduleState writes the scheduler state.
func (n *Notebook) SaveScheduleState(st ScheduleState) error {
	return writeJSON(filepath.Join(n.Dir, ScheduleStateFile), st)
}
