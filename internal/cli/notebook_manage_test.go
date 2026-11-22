package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotebookManageCommands(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(h.dir, "ops")
	h.mustRun("new", dir, "--type", "summarize")
	if out := h.mustRun("notebook", "rename", dir, "Weekly", "ops"); !strings.Contains(out, `"Weekly ops"`) {
		t.Fatalf("rename: %q", out)
	}
	if out := h.mustRun("notebook", "archive", dir); !strings.Contains(out, "archived") {
		t.Fatalf("archive: %q", out)
	}
	h.mustRun("notebook", "unarchive", dir)
	for _, args := range [][]string{{"notebook"}, {"notebook", "bogus", dir}, {"notebook", "rename", dir}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := h.run("notebook", "delete", dir); code != ExitUsage {
		t.Fatalf("delete without --yes: exit %d", code)
	}
	h.mustRun("notebook", "delete", "--yes", dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("not deleted: %v", err)
	}
	if code := h.run("notebook", "delete", "--yes", h.dir); code != ExitError {
		t.Fatalf("deleting a folder that is not a notebook: exit %d", code)
	}
}
