package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/pack"
)

func TestPackCommands(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	dir := filepath.Join(h.dir, "team-pack")
	if out := h.mustRun("pack", "init", dir, "--id", "team", "--name", "Team pack"); !strings.Contains(out, dir) {
		t.Fatalf("init: %q", out)
	}
	if out := h.mustRun("pack", "add", dir); !strings.Contains(out, `added "Team pack" (team)`) {
		t.Fatalf("add: %q", out)
	}
	if out := h.mustRun("pack", "list"); !strings.Contains(out, "team") || !strings.Contains(out, "folder") {
		t.Fatalf("list: %q", out)
	}
	h.mustRun("pack", "refresh")
	h.mustRun("pack", "remove", "team")
	if out := h.mustRun("pack", "list"); strings.Contains(out, "team") {
		t.Fatalf("after remove: %q", out)
	}

	nb := filepath.Join(h.dir, "ops")
	h.mustRun("new", nb, "--type", "summarize")
	h.mustRun("prompt", "save", nb, "triage", "Triage these issues.")
	out := filepath.Join(h.dir, "exported")
	h.mustRun("pack", "export", out, "--id", "ops", "--name", "Ops pack", "--prompts", nb)
	p, err := pack.Load(out)
	if err != nil || len(p.Prompts) != 1 || p.Prompts[0].Name != "triage" {
		t.Fatalf("export: %v %+v", err, p)
	}
	for _, args := range [][]string{
		{"pack"},
		{"pack", "add"},
		{"pack", "remove"},
		{"pack", "init", dir},
		{"pack", "bogus"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
