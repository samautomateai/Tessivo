package service

import (
	"context"
	"errors"
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// addSuggestion stores one open suggestion for the fixture's output, with the
// current target prompt's digest for prompt suggestions.
func (f threadFixture) addSuggestion(t *testing.T, kind, text string) notebook.Suggestion {
	t.Helper()
	ref, err := f.s.loadOutput(f.nbID, f.output)
	if err != nil {
		t.Fatal(err)
	}
	sg := notebook.Suggestion{ID: notebook.NewID(), Kind: kind, Text: text, Status: notebook.SuggestOpen}
	if kind == notebook.SuggestPrompt {
		tg, err := f.s.currentTarget(ref)
		if err != nil {
			t.Fatal(err)
		}
		sg.Base = promptDigest(tg.Current)
	}
	if err := ref.nb.UpdateSuggestions(f.output, func(l []notebook.Suggestion) ([]notebook.Suggestion, error) {
		return append(l, sg), nil
	}); err != nil {
		t.Fatal(err)
	}
	return sg
}

// setRun rewrites the fixture output's run record.
func (f threadFixture) setRun(t *testing.T, edit func(*notebook.Run)) {
	t.Helper()
	ref, err := f.s.loadOutput(f.nbID, f.output)
	if err != nil {
		t.Fatal(err)
	}
	edit(&ref.run)
	if err := ref.nb.SaveRun(ref.run); err != nil {
		t.Fatal(err)
	}
}

func (f threadFixture) reopen(t *testing.T) *notebook.Notebook {
	t.Helper()
	nb, err := f.s.OpenNotebook(f.nbID)
	if err != nil {
		t.Fatal(err)
	}
	return nb
}

func TestApplyMemory(t *testing.T) {
	f := newThreadFixture(t)
	sg := f.addSuggestion(t, notebook.SuggestMemory, "cite file paths")
	res, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID)
	if err != nil || res.Suggestion.Status != notebook.SuggestApplied || res.Suggestion.DecidedAt == "" {
		t.Fatalf("%v %+v", err, res)
	}
	mem := f.reopen(t).Manifest.Memory
	if mem[len(mem)-1] != "cite file paths" {
		t.Fatalf("memory %v", mem)
	}
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("apply twice: %v", err)
	}
}

func TestApplyMemoryFullIs409(t *testing.T) {
	f := newThreadFixture(t)
	full := make([]string, 50)
	for i := range full {
		full[i] = "fact"
	}
	if _, err := f.s.SetMemory(f.nbID, full); err != nil {
		t.Fatal(err)
	}
	sg := f.addSuggestion(t, notebook.SuggestMemory, "one more")
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("full memory: %v", err)
	}
}

func TestApplyPromptSavedUpdatesSharedPrompt(t *testing.T) {
	f := newThreadFixture(t)
	nb := f.reopen(t)
	if err := nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "old {{team}}"}); err != nil {
		t.Fatal(err)
	}
	f.setRun(t, func(r *notebook.Run) { r.PromptName, r.Vars = "weekly", map[string]string{"team": "ops"} })
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "new {{team}}, cite files")
	list, err := f.s.ListSuggestions(f.nbID, f.output)
	if err != nil || len(list) != 1 || list[0].Target == nil || list[0].Target.PromptName != "weekly" {
		t.Fatalf("list: %v %+v", err, list)
	}
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); err != nil {
		t.Fatal(err)
	}
	p, _ := nb.LoadPrompt("weekly")
	if p.Text != "new {{team}}, cite files" {
		t.Fatalf("prompt %q", p.Text)
	}
}

func TestApplyPromptScheduleInline(t *testing.T) {
	f := newThreadFixture(t)
	nb := f.reopen(t)
	if err := nb.SaveSchedules([]notebook.Schedule{
		{ID: "c", Cron: "@daily", Prompt: "Summarize the board.", Missed: notebook.MissedSkip, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	f.setRun(t, func(r *notebook.Run) { r.ScheduleID = "c" })
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board. Cite files.")
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := nb.Schedules()
	if list[0].Prompt != "Summarize the board. Cite files." || !list[0].Enabled || list[0].Cron != "@daily" {
		t.Fatalf("schedule %+v", list[0])
	}
}

func TestApplyPromptNewName(t *testing.T) {
	f := newThreadFixture(t)
	if err := f.reopen(t).SavePrompt(notebook.Prompt{Name: "refined-1", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board. Cite files.")
	res, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID)
	if err != nil || res.PromptName != "refined-2" {
		t.Fatalf("%v %+v", err, res)
	}
	if p, err := f.reopen(t).LoadPrompt("refined-2"); err != nil || p.Text != sg.Text {
		t.Fatalf("%v %+v", err, p)
	}
}

func TestApplyStaleIs409(t *testing.T) {
	f := newThreadFixture(t)
	nb := f.reopen(t)
	_ = nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "old"})
	f.setRun(t, func(r *notebook.Run) { r.PromptName = "weekly" })
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "suggested")
	_ = nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "edited by hand"})
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale: %v", err)
	}
	if p, _ := nb.LoadPrompt("weekly"); p.Text != "edited by hand" {
		t.Fatalf("prompt changed: %q", p.Text)
	}
}

func TestApplyNewVariableIs422(t *testing.T) {
	f := newThreadFixture(t)
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board for {{team}}.")
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); !IsInvalid(err) {
		t.Fatalf("new variable: %v", err)
	}
	list, _ := f.s.ListSuggestions(f.nbID, f.output)
	if list[0].Status != notebook.SuggestOpen {
		t.Fatalf("status %q", list[0].Status)
	}
}

func TestDismiss(t *testing.T) {
	f := newThreadFixture(t)
	sg := f.addSuggestion(t, notebook.SuggestMemory, "x")
	got, err := f.s.DismissSuggestion(f.nbID, f.output, sg.ID)
	if err != nil || got.Status != notebook.SuggestDismissed {
		t.Fatalf("%v %+v", err, got)
	}
	if _, err := f.s.DismissSuggestion(f.nbID, f.output, sg.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("dismiss twice: %v", err)
	}
	if _, err := f.s.DismissSuggestion(f.nbID, f.output, notebook.NewID()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if len(f.reopen(t).Manifest.Memory) != 1 {
		t.Fatal("dismiss changed memory")
	}
}

func TestGenerateSuggestionsStoresValid(t *testing.T) {
	f := newThreadFixture(t)
	if _, _, err := f.send(t, "always cite files"); err != nil {
		t.Fatal(err)
	}
	f.model.steps = []step{{text: `{"suggestions":[{"kind":"memory","text":"cite files","reason":"asked"},` +
		`{"kind":"prompt","text":"Summarize the board. Cite files."}]}`}}
	list, err := f.s.GenerateSuggestions(context.Background(), f.nbID, f.output)
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %+v", err, list)
	}
	reqs := f.model.requests()
	last := reqs[len(reqs)-1]
	if last.Format == nil || len(last.Tools) != 0 {
		t.Fatalf("want a structured call without tools: %+v", last.Format)
	}
	if list[1].Target == nil || list[1].Base == "" {
		t.Fatalf("prompt suggestion lacks target or base: %+v", list[1])
	}
}

func TestGenerateSuggestionsInvalidJSON(t *testing.T) {
	f := newThreadFixture(t)
	if _, _, err := f.send(t, "q"); err != nil {
		t.Fatal(err)
	}
	f.model.steps = []step{{text: `{"nope":1}`}}
	_, err := f.s.GenerateSuggestions(context.Background(), f.nbID, f.output)
	if !errors.Is(err, llm.ErrInvalidOutput) {
		t.Fatalf("invalid: %v", err)
	}
	if list, _ := f.s.ListSuggestions(f.nbID, f.output); len(list) != 0 {
		t.Fatalf("stored %d", len(list))
	}
}

func TestGenerateSuggestionsEmptyThread(t *testing.T) {
	f := newThreadFixture(t)
	if _, err := f.s.GenerateSuggestions(context.Background(), f.nbID, f.output); !IsInvalid(err) {
		t.Fatalf("empty thread: %v", err)
	}
}
