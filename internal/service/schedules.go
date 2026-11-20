package service

import (
	"fmt"
	"time"

	"github.com/samautomateai/tessivo/internal/cron"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// ScheduleInfo is a schedule with its next run and recent history.
type ScheduleInfo struct {
	notebook.Schedule
	NotebookID string                  `json:"notebookId"`
	NextRun    string                  `json:"nextRun,omitempty"`
	History    []notebook.HistoryEntry `json:"history"`
}

// ScheduleInput is what clients send to create a schedule.
type ScheduleInput struct {
	Cron       string            `json:"cron"`
	Prompt     string            `json:"prompt"`
	PromptName string            `json:"promptName"`
	Vars       map[string]string `json:"vars"`
	Missed     string            `json:"missed"`
	Enabled    *bool             `json:"enabled"`
}

// AddSchedule validates and stores a new schedule.
func (s *Service) AddSchedule(id string, in ScheduleInput) (ScheduleInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return ScheduleInfo{}, err
	}
	if in.Missed == "" {
		in.Missed = notebook.MissedSkip
	}
	sch := notebook.Schedule{
		ID: notebook.NewID(), Cron: in.Cron, Prompt: in.Prompt, PromptName: in.PromptName, Vars: in.Vars,
		Missed: in.Missed, Enabled: in.Enabled == nil || *in.Enabled, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := sch.Validate(); err != nil {
		return ScheduleInfo{}, &InvalidError{err.Error()}
	}
	if sch.PromptName != "" {
		if _, err := nb.LoadPrompt(sch.PromptName); err != nil {
			return ScheduleInfo{}, invalidf("prompt %q does not exist", sch.PromptName)
		}
	}
	list, err := nb.Schedules()
	if err != nil {
		return ScheduleInfo{}, err
	}
	if err := nb.SaveSchedules(append(list, sch)); err != nil {
		return ScheduleInfo{}, &InvalidError{err.Error()}
	}
	return scheduleInfo(id, sch, nil, time.Now()), nil
}

// ListSchedules returns a notebook's schedules with next runs and history.
func (s *Service) ListSchedules(id string) ([]ScheduleInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return nil, err
	}
	list, err := nb.Schedules()
	if err != nil {
		return nil, err
	}
	st, err := nb.LoadScheduleState()
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleInfo, 0, len(list))
	now := time.Now()
	for _, sch := range list {
		out = append(out, scheduleInfo(id, sch, st.Schedules[sch.ID], now))
	}
	return out, nil
}

func scheduleInfo(id string, sch notebook.Schedule, st *notebook.ScheduleRunState, now time.Time) ScheduleInfo {
	info := ScheduleInfo{Schedule: sch, NotebookID: id, History: []notebook.HistoryEntry{}}
	if st != nil {
		info.History = st.History
	}
	if c, err := cron.Parse(sch.Cron, time.Local); err == nil && sch.Enabled {
		if next, err := c.Next(now); err == nil {
			info.NextRun = next.Format(time.RFC3339)
		}
	}
	return info
}

// SetScheduleEnabled pauses or resumes a schedule.
func (s *Service) SetScheduleEnabled(id, scheduleID string, enabled bool) (ScheduleInfo, error) {
	return s.editSchedule(id, scheduleID, func(list []notebook.Schedule, i int) []notebook.Schedule {
		list[i].Enabled = enabled
		return list
	})
}

// DeleteSchedule removes a schedule.
func (s *Service) DeleteSchedule(id, scheduleID string) error {
	_, err := s.editSchedule(id, scheduleID, func(list []notebook.Schedule, i int) []notebook.Schedule {
		return append(list[:i], list[i+1:]...)
	})
	return err
}

func (s *Service) editSchedule(id, scheduleID string, edit func([]notebook.Schedule, int) []notebook.Schedule,
) (ScheduleInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return ScheduleInfo{}, err
	}
	list, err := nb.Schedules()
	if err != nil {
		return ScheduleInfo{}, err
	}
	for i, sch := range list {
		if sch.ID != scheduleID {
			continue
		}
		updated := edit(list, i)
		if err := nb.SaveSchedules(updated); err != nil {
			return ScheduleInfo{}, err
		}
		for _, u := range updated {
			if u.ID == scheduleID {
				return scheduleInfo(id, u, nil, time.Now()), nil
			}
		}
		return ScheduleInfo{}, nil
	}
	return ScheduleInfo{}, fmt.Errorf("schedule %s: %w", scheduleID, ErrNotFound)
}
