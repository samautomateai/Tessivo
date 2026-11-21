package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/testutil"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestBuiltinsAreValid(t *testing.T) {
	all := Builtins()
	if len(all) != 12 {
		t.Fatalf("want 12 built-ins, got %d", len(all))
	}
	for _, tpl := range all {
		if err := tpl.Validate(); err != nil {
			t.Errorf("%s: %v", tpl.ID, err)
		}
		if tpl.Source != SourceBuiltin || tpl.Version != 1 {
			t.Errorf("%s: source/version", tpl.ID)
		}
	}
}

func TestLibrarySaveGetDelete(t *testing.T) {
	lib := Library{Dir: filepath.Join(t.TempDir(), "templates")}
	if list, err := lib.List(); err != nil || len(list) != 12 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	base, _ := lib.Get("change-risk-review")
	mine := Extend(base, "payments-risk", "Payments risk review")
	mine.Memory = append(mine.Memory, "Payments must be idempotent.")
	saved, err := lib.Save(mine)
	if err != nil || saved.Version != 1 || saved.Parent != "change-risk-review@1" {
		t.Fatalf("save: %v %+v", err, saved)
	}
	testutil.AssertPerm(t, filepath.Join(lib.Dir, "payments-risk.json"), 0o600)
	again, _ := lib.Save(saved)
	if again.Version != 2 {
		t.Fatalf("want version bump, got %d", again.Version)
	}
	got, err := lib.Get("payments-risk")
	if err != nil || got.Source != SourceUser || len(got.Memory) != 1 {
		t.Fatalf("get: %v %+v", err, got)
	}
	list, _ := lib.List()
	if len(list) != 13 {
		t.Fatalf("want 13 templates, got %d", len(list))
	}
	if _, err := lib.Save(base); err == nil {
		t.Fatal("built-ins cannot be overwritten")
	}
	if err := lib.Delete("change-risk-review"); err == nil {
		t.Fatal("built-ins cannot be deleted")
	}
	if err := lib.Delete("payments-risk"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"payments-risk", "../etc", "Nope"} {
		if _, err := lib.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: want not found, got %v", id, err)
		}
		if err := lib.Delete(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("delete %q: want not found, got %v", id, err)
		}
	}
	_ = os.WriteFile(filepath.Join(lib.Dir, "broken.json"), []byte("{"), 0o600)
	if list, err := lib.List(); err != nil || len(list) != 12 {
		t.Fatal("a broken user template must not break the list")
	}
}

func TestValidationRejectsSecretsAndBadParts(t *testing.T) {
	ok := Template{ID: "x", Name: "X", WorkType: "summarize", Prompts: []Prompt{{Name: "p", Text: "Summarize"}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []func(*Template){
		func(t *Template) { t.ID = "Bad ID" },
		func(t *Template) { t.Name = " " },
		func(t *Template) { t.WorkType = "poem" },
		func(t *Template) { t.Prompts = nil },
		func(t *Template) { t.Prompts[0].Name = "../p" },
		func(t *Template) { t.Prompts[0].Text = "use api_key=sk-1234567890abcdef" },
		func(t *Template) { t.Memory = []string{"token: abcdefghijklmn"} },
		func(t *Template) { t.Prompts[0].Text = strings.Repeat("a", MaxTextBytes+1) },
		func(t *Template) { t.Settings = config.RunSettings{Mode: "warp"} },
		func(t *Template) { t.Schedule = &Schedule{Cron: "nope", Missed: "skip"} },
		func(t *Template) { t.Schedule = &Schedule{Cron: "@daily", Missed: "always"} },
		func(t *Template) { t.Schedule = &Schedule{Cron: "@daily", Missed: "skip", PromptName: "missing"} },
	}
	for i, mutate := range bad {
		tpl := ok
		tpl.Prompts = append([]Prompt(nil), ok.Prompts...)
		mutate(&tpl)
		if tpl.Validate() == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestApplyAndFromNotebook(t *testing.T) {
	lib := Library{Dir: t.TempDir()}
	weekly, _ := lib.Get("weekly-ops-report")
	nb, err := notebook.Create(filepath.Join(t.TempDir(), "ops"), "Ops", "report")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(nb, weekly); err != nil {
		t.Fatal(err)
	}
	if nb.Manifest.Template != "weekly-ops-report@1" {
		t.Fatalf("template ref: %q", nb.Manifest.Template)
	}
	if p, err := nb.LoadPrompt("weekly"); err != nil || !strings.Contains(p.Text, "ops report") {
		t.Fatalf("prompt: %v %+v", err, p)
	}
	scheds, _ := nb.Schedules()
	if len(scheds) != 1 || scheds[0].Enabled || scheds[0].Missed != "run_once" {
		t.Fatalf("schedule should be created disabled: %+v", scheds)
	}
	other, _ := notebook.Create(filepath.Join(t.TempDir(), "x"), "X", "analyze")
	if err := Apply(other, weekly); err == nil {
		t.Fatal("work type mismatch must fail")
	}

	src := filepath.Join(t.TempDir(), "tickets.csv")
	_ = os.WriteFile(src, []byte("id\n1\n"), 0o600)
	_, _ = nb.AddContext(src)
	_, _ = nb.AddContext(t.TempDir())
	nb.Manifest.Memory = []string{"Fiscal week starts Monday"}
	captured, err := FromNotebook(nb, "my-ops", "My ops", "Our weekly report")
	if err != nil {
		t.Fatal(err)
	}
	if err := captured.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(captured.Slots) != 2 || captured.Slots[0].Name != "tickets" || captured.Slots[1].Kind != "code base" {
		t.Fatalf("slots: %+v", captured.Slots)
	}
	if captured.Schedule == nil || len(captured.Memory) != 1 {
		t.Fatalf("captured: %+v", captured)
	}
	empty, _ := notebook.Create(filepath.Join(t.TempDir(), "e"), "E", "summarize")
	if c, _ := FromNotebook(empty, "e", "E", ""); len(c.Prompts) != 1 {
		t.Fatal("a notebook without prompts gets a default prompt")
	}
}

func TestParentUpdate(t *testing.T) {
	lib := Library{Dir: t.TempDir()}
	parent := Template{
		ID: "team-base", Name: "Team base", WorkType: "summarize",
		Prompts: []Prompt{{Name: "a", Text: "v1"}, {Name: "b", Text: "same"}},
	}
	parent, _ = lib.Save(parent)
	child, _ := lib.Save(Extend(parent, "team-child", "Team child"))
	if n, _, err := lib.ParentUpdate(child); err != nil || n != 0 {
		t.Fatalf("no update yet: %d %v", n, err)
	}
	parent.Prompts[0].Text = "v2"
	if _, err := lib.Save(parent); err != nil {
		t.Fatal(err)
	}
	n, changed, err := lib.ParentUpdate(child)
	if err != nil || n != 2 || len(changed) != 1 || changed[0] != "a" {
		t.Fatalf("update: %d %v %v", n, changed, err)
	}
	if n, _, _ := lib.ParentUpdate(parent); n != 0 {
		t.Fatal("templates without a parent have no updates")
	}
	child.Parent = "nope"
	if _, _, err := lib.ParentUpdate(child); err == nil {
		t.Fatal("want bad reference error")
	}
	child.Parent = "gone@1"
	if _, _, err := lib.ParentUpdate(child); err == nil {
		t.Fatal("want missing parent error")
	}
}

func TestOversizedTemplateFile(t *testing.T) {
	lib := Library{Dir: t.TempDir()}
	big := []byte(`{"id":"` + strings.Repeat("a", MaxFileBytes) + `"}`)
	_ = os.WriteFile(filepath.Join(lib.Dir, "big.json"), big, 0o600)
	if _, err := lib.Get("big"); err == nil {
		t.Fatal("want size error")
	}
}
