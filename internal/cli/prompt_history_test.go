package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptHistoryCommands(t *testing.T) {
	h := newHarness(t)
	nb := filepath.Join(h.dir, "ops")
	h.mustRun("new", nb, "--type", "summarize")
	h.mustRun("prompt", "save", nb, "weekly", "first")
	h.mustRun("prompt", "save", nb, "weekly", "second")
	out := h.mustRun("prompt", "history", nb, "weekly")
	if !strings.Contains(out, "v2") || !strings.Contains(out, "v1") || !strings.Contains(out, "first") {
		t.Fatalf("history: %q", out)
	}
	if out := h.mustRun("prompt", "restore", nb, "weekly", "1"); !strings.Contains(out, "v3") {
		t.Fatalf("restore: %q", out)
	}
	if out := h.mustRun("prompt", "show", nb, "weekly"); !strings.Contains(out, "first") {
		t.Fatalf("after restore: %q", out)
	}
	for _, args := range [][]string{{"prompt", "restore", nb, "weekly"}, {"prompt", "restore", nb, "weekly", "x"}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
