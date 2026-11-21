package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// A tool call over the llm limits must not fail the reply: it is retried
// without tools.
func TestSendThreadToolCallLimitRetriesWithoutTools(t *testing.T) {
	f := newThreadFixture(t, step{err: fmt.Errorf("stream: %w", llm.ErrToolCallLimit)}, step{text: "plain"})
	r, _, err := f.send(t, "q")
	if err != nil || r.Assistant.Content != "plain" || r.Assistant.Error != "" {
		t.Fatalf("%v %+v", err, r.Assistant)
	}
	if reqs := f.model.requests(); len(reqs) != 2 || len(reqs[1].Tools) != 0 {
		t.Fatalf("retry without tools: %d requests", len(reqs))
	}
}

// The follow-up round keeps the tool definitions (some providers require them
// when the history holds tool calls) but forbids new calls.
func TestFollowUpKeepsToolsWithChoiceNone(t *testing.T) {
	f := newThreadFixture(t,
		step{text: "Noted.", calls: []llm.ToolCall{call("suggest_memory", `{"text":"cite files"}`)}},
		step{text: "Done."})
	r, _, err := f.send(t, "q")
	if err != nil {
		t.Fatal(err)
	}
	follow := f.model.requests()[1]
	if len(follow.Tools) != 2 || follow.ToolChoice != "none" {
		t.Fatalf("follow-up tools %d choice %q", len(follow.Tools), follow.ToolChoice)
	}
	if r.Assistant.Content != "Noted.\n\nDone." {
		t.Fatalf("rounds glued: %q", r.Assistant.Content)
	}
}

func TestGenerateSuggestionsOverCapIs409(t *testing.T) {
	f := newThreadFixture(t)
	for range notebook.MaxOpenSuggestions {
		f.addSuggestion(t, notebook.SuggestMemory, "x")
	}
	if _, _, err := f.send(t, "q"); err != nil {
		t.Fatal(err)
	}
	f.model.steps = []step{{text: `{"suggestions":[{"kind":"memory","text":"cite files"}]}`}}
	_, err := f.s.GenerateSuggestions(context.Background(), f.nbID, f.output)
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "dismiss") {
		t.Fatalf("over cap: %v", err)
	}
}

// After a prompt suggestion created a new saved prompt for an ad-hoc run,
// later suggestions and versions of that output use it.
func TestAppliedNewPromptIsSticky(t *testing.T) {
	f := newThreadFixture(t)
	first := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board. Cite files.")
	res, err := f.s.ApplySuggestion(f.nbID, f.output, first.ID)
	if err != nil || res.PromptName != "refined-1" {
		t.Fatalf("%v %+v", err, res)
	}
	list, _ := f.s.ListSuggestions(f.nbID, f.output)
	if tg := list[0].Target; tg == nil || tg.Kind != targetSaved || tg.PromptName != "refined-1" {
		t.Fatalf("target after apply: %+v", tg)
	}
	second := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board. Cite files and lines.")
	if res, err = f.s.ApplySuggestion(f.nbID, f.output, second.ID); err != nil || res.PromptName != "" {
		t.Fatalf("second apply: %v %+v", err, res)
	}
	p, _ := f.reopen(t).LoadPrompt("refined-1")
	if p.Text != second.Text {
		t.Fatalf("refined-1 = %q", p.Text)
	}
	if _, err := f.reopen(t).LoadPrompt("refined-2"); err == nil {
		t.Fatal("a second prompt was created")
	}
}

// Older scheduled runs recorded neither PromptName nor ScheduleID: a unique
// exact text match finds their schedule.
func TestPromptTargetMatchesOlderRunsByText(t *testing.T) {
	f := newThreadFixture(t)
	if err := f.nb.SaveSchedules([]notebook.Schedule{
		{ID: "c", Cron: "@daily", Prompt: "Summarize the board.", Missed: notebook.MissedSkip},
	}); err != nil {
		t.Fatal(err)
	}
	tg, err := f.s.promptTarget(f.nb, notebook.Run{Prompt: "Summarize the board."})
	if err != nil || tg.Kind != targetSchedule || tg.ScheduleID != "c" {
		t.Fatalf("schedule by text: %v %+v", err, tg)
	}
	_ = f.nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "Summarize the board."})
	if tg, _ = f.s.promptTarget(f.nb, notebook.Run{Prompt: "Summarize the board."}); tg.Kind != targetNew {
		t.Fatalf("ambiguous match must not guess: %+v", tg)
	}
}

func TestPromptTargetMissingSavedPromptFallsBack(t *testing.T) {
	f := newThreadFixture(t)
	tg, err := f.s.promptTarget(f.nb, notebook.Run{PromptName: "gone", Prompt: "old text"})
	if err != nil || tg.Kind != targetNew || tg.Current != "old text" {
		t.Fatalf("%v %+v", err, tg)
	}
}

func TestApplyWithUnknownBaseIsAccepted(t *testing.T) {
	f := newThreadFixture(t)
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize the board. Cite files.")
	if err := f.nb.UpdateSuggestions(f.output, func(l []notebook.Suggestion) ([]notebook.Suggestion, error) {
		l[0].Base = ""
		return l, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); err != nil {
		t.Fatalf("unknown base: %v", err)
	}
}

// A saved prompt shared by schedules: a new variable must be set by every
// schedule it reaches, not only by the run.
func TestApplyNewVariableUnsetBySharedScheduleIsRejected(t *testing.T) {
	f := newThreadFixture(t)
	_ = f.nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "Summarize."})
	_ = f.nb.SaveSchedules([]notebook.Schedule{
		{
			ID: "a", Cron: "@daily", PromptName: "weekly", Missed: notebook.MissedSkip,
			Vars: map[string]string{"team": "x"},
		},
		{ID: "b", Cron: "@daily", PromptName: "weekly", Missed: notebook.MissedSkip},
	})
	f.setRun(t, func(r *notebook.Run) { r.PromptName, r.Vars = "weekly", map[string]string{"team": "ops"} })
	sg := f.addSuggestion(t, notebook.SuggestPrompt, "Summarize for {{team}}.")
	if _, err := f.s.ApplySuggestion(f.nbID, f.output, sg.ID); !IsInvalid(err) || !strings.Contains(err.Error(), "b") {
		t.Fatalf("schedule b lacks team: %v", err)
	}
}

// The note in the parent thread is cosmetic: failing to write it must not
// turn a saved version into an error.
func TestNewVersionSavedEvenIfNoteFails(t *testing.T) {
	f := newThreadFixture(t, step{text: "ok"}, step{text: revised})
	if _, _, err := f.send(t, "change it"); err != nil {
		t.Fatal(err)
	}
	noteFail = func() error { return errors.New("disk full") }
	t.Cleanup(func() { noteFail = nil })
	out, err := f.s.NewVersion(context.Background(), f.nbID, f.output, "")
	if err != nil || out.OutputID == "" {
		t.Fatalf("%v %+v", err, out)
	}
}

// A version of an output made from a saved prompt uses that prompt's settings.
func TestNewVersionUsesSavedPromptSettings(t *testing.T) {
	f := newThreadFixture(t, step{text: "ok"}, step{text: revised})
	effort := "high"
	p := notebook.Prompt{Name: "weekly", Text: "Summarize the board."}
	p.Settings.ThinkingEffort = effort
	_ = f.nb.SavePrompt(p)
	f.setRun(t, func(r *notebook.Run) { r.PromptName = "weekly" })
	if _, _, err := f.send(t, "change it"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, ""); err != nil {
		t.Fatal(err)
	}
	reqs := f.model.requests()
	if got := reqs[len(reqs)-1].ReasoningEffort; got != effort {
		t.Fatalf("reasoning effort %q", got)
	}
}

func TestSuggestInputFallsBackToData(t *testing.T) {
	f := newThreadFixture(t)
	ref, _ := f.s.loadOutput(f.nbID, f.output)
	_ = os.Remove(filepath.Join(f.nb.Dir, notebook.OutputsDir, f.output+".md"))
	ref.md = ""
	if in := f.s.suggestInput(ref, "user: q"); !strings.Contains(in, "Board digest") {
		t.Fatalf("no output in suggest input:\n%s", in)
	}
}
