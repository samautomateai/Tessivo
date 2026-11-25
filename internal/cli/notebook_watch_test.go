package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestNotebookWatch(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	dir := filepath.Join(h.dir, "ops")
	h.mustRun("new", dir, "--type", "summarize")
	if out := h.mustRun("notebook", "watch", dir, "run"); !strings.Contains(out, "run") {
		t.Fatalf("watch run: %q", out)
	}
	if nb, _ := notebook.Open(dir); nb.Manifest.Watch != notebook.WatchRun {
		t.Fatalf("saved: %q", nb.Manifest.Watch)
	}
	h.mustRun("notebook", "watch", dir, "off")
	if nb, _ := notebook.Open(dir); nb.Manifest.Watch != "" {
		t.Fatalf("off: %q", nb.Manifest.Watch)
	}
	for _, args := range [][]string{{"notebook", "watch", dir}, {"notebook", "watch", dir, "sometimes"}} {
		if code := h.run(args...); code == ExitOK {
			t.Errorf("%v: want an error", args)
		}
	}
}

func TestNotebookWorkType(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	dir := filepath.Join(h.dir, "pay")
	h.mustRun("new", dir, "--type", "summarize")
	patch := filepath.Join(h.dir, "fix.patch")
	_ = os.WriteFile(patch, []byte("diff --git a b"), 0o600)
	h.mustRun("context", "add", dir, patch)
	if out := h.mustRun("notebook", "worktype", dir); !strings.Contains(out, "suggested: analyze") {
		t.Fatalf("suggestion: %q", out)
	}
	h.mustRun("notebook", "worktype", dir, "analyze")
	if out := h.mustRun("notebook", "worktype", dir); strings.Contains(out, "suggested") ||
		!strings.Contains(out, "analyze") {
		t.Fatalf("after switching: %q", out)
	}
	for _, args := range [][]string{{"notebook", "worktype"}, {"notebook", "worktype", dir, "poem"}} {
		if code := h.run(args...); code == ExitOK {
			t.Errorf("%v: want an error", args)
		}
	}
}
