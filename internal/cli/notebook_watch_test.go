package cli

import (
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
