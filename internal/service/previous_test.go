package service

import (
	"context"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/worktype"
)

func TestPreviousFillsTheLatestOutput(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	nb := mustOpen(t, s, info.ID)
	ctx := context.Background()
	spec, err := s.buildSpec(ctx, RunRequest{Notebook: nb, Prompt: "What changed since {{previous}}?"})
	if err != nil || spec.Vars["previous"] != noPrevious {
		t.Fatalf("no outputs yet: %v %q", err, spec.Vars["previous"])
	}
	_ = nb.SaveOutput(notebook.Output{
		ID: "20261101T080000-aaaaaa", Title: "Week 1",
		CreatedAt: "2026-11-01T08:00:00Z",
	}, "# Week 1")
	_ = nb.SaveOutput(notebook.Output{
		ID: "20261108T080000-bbbbbb", Title: "Week 2",
		CreatedAt: "2026-11-08T08:00:00Z",
	}, "# Week 2")
	spec, err = s.buildSpec(ctx, RunRequest{Notebook: nb, Prompt: "Compare with {{previous}}"})
	if err != nil || spec.Vars["previous"] != "# Week 2" {
		t.Fatalf("newest output: %v %q", err, spec.Vars["previous"])
	}
	mine := map[string]string{"previous": "mine"}
	spec, _ = s.buildSpec(ctx, RunRequest{Notebook: nb, Prompt: "{{previous}}", Vars: mine})
	if spec.Vars["previous"] != "mine" {
		t.Fatal("a value the run passes wins")
	}
	_ = nb.SaveOutput(notebook.Output{ID: "20261115T080000-cccccc", Title: "Big", CreatedAt: "2026-11-15T08:00:00Z"},
		strings.Repeat("x", maxPreviousChars+100))
	spec, _ = s.buildSpec(ctx, RunRequest{Notebook: nb, Prompt: "{{previous}}"})
	if got := spec.Vars["previous"]; len(got) > maxPreviousChars+200 || !strings.Contains(got, "cut") {
		t.Fatalf("a long output is cut with a note: %d", len(got))
	}
	spec, _ = s.buildSpec(ctx, RunRequest{Notebook: nb, Prompt: "No variable here"})
	if _, ok := spec.Vars["previous"]; ok {
		t.Fatal("only prompts that use {{previous}} read an output")
	}
}

func TestLibraryHasChangesSinceLastTime(t *testing.T) {
	for _, p := range worktype.Library {
		if p.ID == "changes-since-last-time" && strings.Contains(p.Text, "{{previous}}") {
			return
		}
	}
	t.Fatal("the starter prompt is missing")
}
