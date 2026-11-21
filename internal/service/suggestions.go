package service

import (
	"github.com/samautomateai/tessivo/internal/notebook"
)

// Prompt target kinds: what applying a prompt suggestion changes.
const (
	targetSaved    = "saved"    // a saved prompt (and every schedule using it)
	targetSchedule = "schedule" // a schedule's own inline prompt
	targetNew      = "new"      // an ad-hoc run: a new saved prompt is created
)

// PromptTarget is the prompt an output's run used, and everything a change to
// it reaches.
type PromptTarget struct {
	Kind       string   `json:"kind"`
	PromptName string   `json:"promptName,omitempty"`
	ScheduleID string   `json:"scheduleId,omitempty"`
	Schedules  []string `json:"schedules,omitempty"` // saved: the schedules using that prompt
	Current    string   `json:"current"`             // the prompt text today, for before/after
}

// promptTarget resolves the prompt behind run: its saved prompt, else its
// schedule's saved prompt, else its schedule's inline prompt, else a new one.
func (s *Service) promptTarget(nb *notebook.Notebook, run notebook.Run) (PromptTarget, error) {
	schedules, err := nb.Schedules()
	if err != nil {
		return PromptTarget{}, err
	}
	var sch *notebook.Schedule
	for i := range schedules {
		if run.ScheduleID != "" && schedules[i].ID == run.ScheduleID {
			sch = &schedules[i]
		}
	}
	name := run.PromptName
	if name == "" && sch != nil {
		name = sch.PromptName
	}
	switch {
	case name != "":
		p, err := nb.LoadPrompt(name)
		if err != nil {
			return PromptTarget{}, err
		}
		tg := PromptTarget{Kind: targetSaved, PromptName: name, Current: p.Text}
		for _, sc := range schedules {
			if sc.PromptName == name {
				tg.Schedules = append(tg.Schedules, sc.ID)
			}
		}
		return tg, nil
	case sch != nil:
		current := sch.Prompt
		if current == "" {
			current = run.Prompt
		}
		return PromptTarget{Kind: targetSchedule, ScheduleID: sch.ID, Current: current}, nil
	default:
		return PromptTarget{Kind: targetNew, Current: run.Prompt}, nil
	}
}
