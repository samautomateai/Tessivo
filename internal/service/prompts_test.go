package service

import (
	"errors"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestPromptLibrary(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	temp := 0.2
	p, err := s.SavePrompt(info.ID, notebook.Prompt{Name: "weekly", Text: "Summarize {{week}}"})
	if err != nil || p.Name != "weekly" {
		t.Fatalf("%v %+v", err, p)
	}
	p.Settings.Temperature = &temp
	if _, err := s.SavePrompt(info.ID, p); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListPrompts(info.ID)
	if err != nil || len(list) != 1 || *list[0].Settings.Temperature != 0.2 {
		t.Fatalf("%v %+v", err, list)
	}
	if _, err := s.SavePrompt(info.ID, notebook.Prompt{Name: "bad name", Text: "x"}); !IsInvalid(err) {
		t.Fatalf("bad name: %v", err)
	}
	if _, err := s.AddSchedule(info.ID, ScheduleInput{Cron: "0 8 * * 1", PromptName: "weekly"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePrompt(info.ID, "weekly"); !errors.Is(err, ErrConflict) {
		t.Fatalf("a prompt a schedule uses was deleted: %v", err)
	}
	if err := s.DeletePrompt(info.ID, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
