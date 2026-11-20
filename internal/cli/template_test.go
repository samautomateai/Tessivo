package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateAndMemoryCommands(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	list := h.mustRun("template", "list", "--type", "report")
	if !strings.Contains(list, "weekly-ops-report") || strings.Contains(list, "change-risk-review") {
		t.Fatal(list)
	}
	if out := h.mustRun("template", "show", "contract-digest"); !strings.Contains(out, "auto-renewal") {
		t.Fatal(out)
	}
	nbDir := filepath.Join(h.dir, "ops")
	out := h.mustRun("template", "use", "weekly-ops-report", nbDir, "--name", "Ops")
	if !strings.Contains(out, "Tickets (spreadsheet, required)") ||
		!strings.Contains(out, "schedule was added disabled") {
		t.Fatal(out)
	}
	h.mustRun("memory", "add", nbDir, "Week", "starts", "Monday")
	h.mustRun("memory", "add", nbDir, "Team is ops")
	if out := h.mustRun("memory", "list", nbDir); !strings.Contains(out, "1. Week starts Monday") {
		t.Fatal(out)
	}
	h.mustRun("memory", "remove", nbDir, "2")
	h.model.reply = `{"title":"W47","period":"wk 47","highlights":[],"issues":[],"trends":[],"next_steps":[]}`
	h.mustRun("run", nbDir, "--prompt-name", "weekly", "--quiet")
	if user := h.model.reqs[0].Messages[1].Content; !strings.Contains(user, "Week starts Monday") ||
		strings.Contains(user, "Team is ops") {
		t.Fatalf("memory not applied to the run: %s", user)
	}
	if out := h.mustRun("template", "save", nbDir, "--name", "Our ops"); !strings.Contains(out, "our-ops v1") {
		t.Fatal(out)
	}
	if out := h.mustRun("template", "extend", "our-ops", "--name", "Our ops EU"); !strings.Contains(out, "our-ops@1") {
		t.Fatal(out)
	}
	h.mustRun("template", "save", nbDir, "--name", "Our ops") // bumps our-ops to v2
	if list := h.mustRun("template", "list"); !strings.Contains(list, "parent updated to v2") {
		t.Fatal(list)
	}
	h.mustRun("template", "delete", "our-ops-eu")
	bad := [][]string{
		{"template"},
		{"template", "zap"},
		{"template", "show"},
		{"template", "use", "x"},
		{"template", "delete"},
		{"template", "save", nbDir},
		{"template", "extend", "our-ops"},
		{"memory", "x"},
		{"memory", "zap", nbDir},
		{"memory", "add", nbDir},
		{"memory", "remove", nbDir, "9"},
		{"template", "save", nbDir, "--name", "x", "--id", "Bad ID"},
	}
	for _, args := range bad {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage error, got %d (%s)", args, code, h.errOut)
		}
	}
	for _, args := range [][]string{
		{"template", "show", "nope"},
		{"template", "use", "nope", nbDir + "2"},
		{"template", "delete", "change-risk-review"},
		{"memory", "list", filepath.Join(h.dir, "nope")},
		{"template", "use", "weekly-ops-report", nbDir},
		{"template", "extend", "nope", "--name", "x"},
	} {
		if code := h.run(args...); code != ExitError {
			t.Errorf("%v: want error, got %d", args, code)
		}
	}
}
