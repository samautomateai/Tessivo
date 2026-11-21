package service

import (
	"errors"
	"fmt"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/template"
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

// SuggestionInfo is a suggestion plus, for a prompt suggestion, what applying
// it would change.
type SuggestionInfo struct {
	notebook.Suggestion
	Target *PromptTarget `json:"target,omitempty"`
}

// ApplyResult is an applied suggestion; PromptName is set when applying
// created a new saved prompt.
type ApplyResult struct {
	Suggestion SuggestionInfo `json:"suggestion"`
	PromptName string         `json:"promptName,omitempty"`
}

// ListSuggestions returns an output's suggestions, oldest first.
func (s *Service) ListSuggestions(notebookID, outputID string) ([]SuggestionInfo, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return nil, err
	}
	list, err := ref.nb.Suggestions(outputID)
	if err != nil {
		return nil, err
	}
	return s.describeSuggestions(ref, list), nil
}

func (s *Service) describeSuggestions(ref outputRef, list []notebook.Suggestion) []SuggestionInfo {
	var target *PromptTarget
	if tg, err := s.promptTarget(ref.nb, ref.run); err == nil {
		target = &tg
	}
	out := make([]SuggestionInfo, 0, len(list))
	for _, sg := range list {
		info := SuggestionInfo{Suggestion: sg}
		if sg.Kind == notebook.SuggestPrompt {
			info.Target = target
		}
		out = append(out, info)
	}
	return out
}

// DismissSuggestion marks an open suggestion dismissed; nothing else changes.
func (s *Service) DismissSuggestion(notebookID, outputID, suggestionID string) (SuggestionInfo, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return SuggestionInfo{}, err
	}
	var done notebook.Suggestion
	err = ref.nb.UpdateSuggestions(outputID, func(list []notebook.Suggestion) ([]notebook.Suggestion, error) {
		i, err := openSuggestion(list, suggestionID)
		if err != nil {
			return nil, err
		}
		list[i].Status, list[i].DecidedAt = notebook.SuggestDismissed, nowRFC3339()
		done = list[i]
		return list, nil
	})
	if err != nil {
		return SuggestionInfo{}, err
	}
	return s.describeSuggestions(ref, []notebook.Suggestion{done})[0], nil
}

// openSuggestion finds an open suggestion by ID.
func openSuggestion(list []notebook.Suggestion, id string) (int, error) {
	for i, sg := range list {
		if sg.ID != id {
			continue
		}
		if sg.Status != notebook.SuggestOpen {
			return 0, fmt.Errorf("%w: suggestion %s is already %s", ErrConflict, id, sg.Status)
		}
		return i, nil
	}
	return 0, fmt.Errorf("suggestion %s: %w", id, ErrNotFound)
}

// ApplySuggestion makes the change a suggestion proposes and marks it
// applied. Applies for one notebook are serialized, so two cannot interleave.
func (s *Service) ApplySuggestion(notebookID, outputID, suggestionID string) (ApplyResult, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return ApplyResult{}, err
	}
	var res ApplyResult
	var done notebook.Suggestion
	err = ref.nb.UpdateSuggestions(outputID, func(list []notebook.Suggestion) ([]notebook.Suggestion, error) {
		i, err := openSuggestion(list, suggestionID)
		if err != nil {
			return nil, err
		}
		if res.PromptName, err = s.applyChange(notebookID, ref, list[i]); err != nil {
			return nil, err
		}
		list[i].Status, list[i].DecidedAt = notebook.SuggestApplied, nowRFC3339()
		done = list[i]
		return list, nil
	})
	if err != nil {
		return ApplyResult{}, err
	}
	res.Suggestion = s.describeSuggestions(ref, []notebook.Suggestion{done})[0]
	return res, s.auditApply(ref.nb, outputID, done)
}

func (s *Service) applyChange(notebookID string, ref outputRef, sg notebook.Suggestion) (string, error) {
	if sg.Kind == notebook.SuggestMemory {
		nb, err := s.OpenNotebook(notebookID)
		if err != nil {
			return "", err
		}
		if len(nb.Manifest.Memory) >= template.MaxMemory {
			return "", fmt.Errorf("%w: the notebook's memory is full (%d items); remove one first",
				ErrConflict, template.MaxMemory)
		}
		_, err = s.SetMemory(notebookID, append(nb.Manifest.Memory, sg.Text))
		return "", err
	}
	return s.applyPrompt(ref, sg)
}

// applyPrompt replaces the target prompt after checking it has not changed
// since the suggestion was made and that no unset variable is introduced.
func (s *Service) applyPrompt(ref outputRef, sg notebook.Suggestion) (string, error) {
	nb, err := notebook.Open(ref.nb.Dir)
	if err != nil {
		return "", err
	}
	tg, err := s.promptTarget(nb, ref.run)
	if err != nil {
		return "", err
	}
	if promptDigest(tg.Current) != sg.Base {
		return "", fmt.Errorf("%w: the prompt changed after this suggestion was made", ErrConflict)
	}
	if err := checkNewVariables(tg.Current, sg.Text, ref.run.Vars); err != nil {
		return "", err
	}
	switch tg.Kind {
	case targetSaved:
		p, err := nb.LoadPrompt(tg.PromptName)
		if err != nil {
			return "", err
		}
		p.Text = sg.Text
		return "", nb.SavePrompt(p)
	case targetSchedule:
		return "", setSchedulePrompt(nb, tg.ScheduleID, sg.Text)
	default:
		name, err := freePromptName(nb)
		if err != nil {
			return "", err
		}
		return name, nb.SavePrompt(notebook.Prompt{Name: name, Text: sg.Text})
	}
}

// checkNewVariables rejects variables the new prompt adds that the run does
// not set: every later run would fail to fill them.
func checkNewVariables(current, next string, vars map[string]string) error {
	have := map[string]bool{}
	for _, v := range executor.Variables(current) {
		have[v] = true
	}
	for _, v := range executor.Variables(next) {
		if _, set := vars[v]; !have[v] && !set {
			return invalidf("the suggested prompt adds {{%s}}, which its runs do not set", v)
		}
	}
	return nil
}

func setSchedulePrompt(nb *notebook.Notebook, scheduleID, text string) error {
	list, err := nb.Schedules()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID == scheduleID {
			list[i].Prompt = text
			return nb.SaveSchedules(list)
		}
	}
	return fmt.Errorf("schedule %s: %w", scheduleID, ErrNotFound)
}

// freePromptName returns the lowest refined-<n> not yet saved.
func freePromptName(nb *notebook.Notebook) (string, error) {
	names, err := nb.ListPrompts()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, n := range names {
		taken[n] = true
	}
	for i := 1; ; i++ {
		if name := fmt.Sprintf("refined-%d", i); !taken[name] {
			return name, nil
		}
	}
}

func (s *Service) auditApply(nb *notebook.Notebook, outputID string, sg notebook.Suggestion) error {
	actor := s.Actor
	if actor == "" {
		actor = defaultActor
	}
	l, err := audit.Open(s.AuditPath(nb), actor)
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	err = l.Append(audit.KindSuggestApply, outputID, map[string]any{
		"suggestion": sg.ID, "kind": sg.Kind, "text_sha256": audit.Digest(sg.Text),
	})
	return errors.Join(err, l.Close())
}
