package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScheduleCommands(t *testing.T) {
	h := newHarness(t)
	nbDir := filepath.Join(h.dir, "ops")
	h.mustRun("new", nbDir, "--type", "report")
	out := h.mustRun("schedule", "add", nbDir, "--cron", "0 8 * * MON", "--missed", "run_once", "--var", "team=ops")
	if !strings.Contains(out, "added schedule") {
		t.Fatal(out)
	}
	id := strings.Fields(out)[2]
	list := h.mustRun("schedule", "list", nbDir)
	if !strings.Contains(list, "0 8 * * MON") || !strings.Contains(list, "run_once") || !strings.Contains(list, id) {
		t.Fatal(list)
	}
	h.mustRun("schedule", "disable", nbDir, id)
	if list := h.mustRun("schedule", "list", nbDir); !strings.Contains(list, "false") {
		t.Fatal(list)
	}
	h.mustRun("schedule", "enable", nbDir, id)
	if out := h.mustRun("schedule", "remove", nbDir, id); !strings.Contains(out, "removed") {
		t.Fatal(out)
	}
	if code := h.run("schedule", "remove", nbDir, id); code != ExitError {
		t.Fatal("want missing schedule")
	}
	next := h.mustRun("schedule", "next", "@daily", "--count", "3")
	if strings.Count(next, "\n") != 3 || !strings.Contains(next, "00:00") {
		t.Fatal(next)
	}
	bad := [][]string{
		{"schedule"},
		{"schedule", "zap", nbDir},
		{"schedule", "add", nbDir, "--cron", "nope"},
		{"schedule", "add", nbDir, "--cron", "* * * * *", "--missed", "always"},
		{"schedule", "add", nbDir, "--cron", "* * * * *", "--var", "x"},
		{"schedule", "remove", nbDir},
		{"schedule", "next", "nope"},
		{"schedule", "next", "@daily", "--count", "99"},
	}
	for _, args := range bad {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage error, got %d (%s)", args, code, h.errOut)
		}
	}
	if code := h.run("schedule", "list", filepath.Join(h.dir, "missing")); code != ExitError {
		t.Fatal("want missing notebook")
	}
}
