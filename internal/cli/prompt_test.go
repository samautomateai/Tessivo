package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptCommands(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(h.dir, "ops")
	h.mustRun("new", dir, "--type", "summarize")
	h.mustRun("prompt", "save", dir, "weekly", "--temperature", "0.2", "Summarize", "{{week}}")
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("Daily digest of {{day}}"), false
	h.mustRun("prompt", "save", dir, "daily")
	h.env.StdinIsTTY = true
	out := h.mustRun("prompt", "list", dir)
	if !strings.Contains(out, "weekly") || !strings.Contains(out, "daily") {
		t.Fatalf("list: %q", out)
	}
	if out = h.mustRun("prompt", "show", dir, "weekly"); !strings.Contains(out, "Summarize {{week}}") ||
		!strings.Contains(out, "temperature 0.2") {
		t.Fatalf("show: %q", out)
	}
	h.mustRun("prompt", "rm", dir, "daily")
	if out = h.mustRun("prompt", "list", dir); strings.Contains(out, "daily") {
		t.Fatalf("not removed: %q", out)
	}
	for _, args := range [][]string{{"prompt"}, {"prompt", "save", dir}, {"prompt", "bogus", dir}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := h.run("prompt", "save", dir, "empty"); code != ExitUsage {
		t.Errorf("save without text: exit %d", code)
	}
}
