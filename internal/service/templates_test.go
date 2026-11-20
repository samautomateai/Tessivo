package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/template"
)

func TestTemplatesThroughService(t *testing.T) {
	s := newSvc(t, summary)
	s.Templates = template.Library{Dir: filepath.Join(t.TempDir(), "templates")}
	all, err := s.ListTemplates("")
	if err != nil || len(all) != 12 {
		t.Fatalf("%d %v", len(all), err)
	}
	if reports, _ := s.ListTemplates("report"); len(reports) != 1 {
		t.Fatalf("want 1 report template, got %d", len(reports))
	}
	nb, err := s.CreateFromTemplate("weekly-ops-report", "")
	if err != nil || nb.Name != "Weekly ops report" || nb.Template != "weekly-ops-report@1" {
		t.Fatalf("%v %+v", err, nb)
	}
	scheds, _ := s.ListSchedules(nb.ID)
	if len(scheds) != 1 || scheds[0].Enabled {
		t.Fatalf("template schedule should start disabled: %+v", scheds)
	}
	if _, err := s.CreateFromTemplate("nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}

	if _, err := s.SetMemory(nb.ID, []string{" Fiscal week starts Monday ", "", "Team: ops"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetMemory(nb.ID, []string{strings.Repeat("a", MaxMemoryItem+1)}); !IsInvalid(err) {
		t.Fatal("want too long")
	}
	if _, err := s.SetMemory(nb.ID, make([]string, template.MaxMemory+1)); !IsInvalid(err) {
		t.Fatal("want too many")
	}
	d, _ := s.Describe(nb.ID)
	if len(d.Memory) != 2 || d.Memory[0] != "Fiscal week starts Monday" {
		t.Fatalf("memory: %v", d.Memory)
	}

	saved, err := s.SaveAsTemplate(nb.ID, "", "Our ops report", "Weekly, with our metrics")
	if err != nil || saved.ID != "our-ops-report" || len(saved.Memory) != 2 {
		t.Fatalf("%v %+v", err, saved)
	}
	ext, err := s.ExtendTemplate("change-risk-review", "", "Payments risk")
	if err != nil || ext.Parent != "change-risk-review@1" {
		t.Fatalf("%v %+v", err, ext)
	}
	if _, err := s.ExtendTemplate("change-risk-review", "Bad ID", "x"); !IsInvalid(err) {
		t.Fatal("want invalid id")
	}
	if _, err := s.ExtendTemplate("nope", "", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("want not found")
	}
	if got, err := s.GetTemplate("payments-risk"); err != nil || got.Source != template.SourceUser {
		t.Fatalf("%v %+v", err, got)
	}
	if _, err := s.GetTemplate("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal("want not found")
	}
	if err := s.DeleteTemplate("payments-risk"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTemplate("change-risk-review"); !IsInvalid(err) {
		t.Fatal("built-ins cannot be deleted")
	}
	if _, err := s.SaveAsTemplate("missing", "", "x", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal("want missing notebook")
	}
}

func TestScheduleServiceOps(t *testing.T) {
	s := newSvc(t, summary)
	nb, _ := s.CreateNotebook("Ops", "report")
	info, err := s.AddSchedule(nb.ID, ScheduleInput{Cron: "0 8 * * MON"})
	if err != nil || info.Missed != "skip" || !info.Enabled || info.NextRun == "" {
		t.Fatalf("%v %+v", err, info)
	}
	for _, bad := range []ScheduleInput{
		{Cron: "nope"},
		{Cron: "@daily", Missed: "always"},
		{Cron: "@daily", PromptName: "missing"},
	} {
		if _, err := s.AddSchedule(nb.ID, bad); !IsInvalid(err) {
			t.Errorf("%+v: want invalid, got %v", bad, err)
		}
	}
	off, err := s.SetScheduleEnabled(nb.ID, info.ID, false)
	if err != nil || off.Enabled || off.NextRun != "" {
		t.Fatalf("%v %+v", err, off)
	}
	if list, _ := s.ListSchedules(nb.ID); len(list) != 1 {
		t.Fatal("list")
	}
	if err := s.DeleteSchedule(nb.ID, info.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSchedule(nb.ID, info.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("want not found")
	}
	if _, err := s.ListSchedules("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal("want missing notebook")
	}
	if _, err := s.AddSchedule("missing", ScheduleInput{Cron: "@daily"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("want missing notebook")
	}
}
