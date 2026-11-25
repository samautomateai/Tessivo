package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotebookTagsPinsAndList(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	ws := filepath.Join(h.dir, "ws")
	f, _ := os.ReadFile(h.env.ConfigPath)
	var cfg map[string]any
	_ = json.Unmarshal(f, &cfg)
	cfg["workspace_dir"] = ws
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(h.env.ConfigPath, b, 0o600)
	ops, board := filepath.Join(ws, "ops"), filepath.Join(ws, "board")
	h.mustRun("new", ops, "--type", "summarize")
	h.mustRun("new", board, "--type", "summarize")

	if out := h.mustRun("notebook", "tag", ops, "add", "Infra"); !strings.Contains(out, "Infra") {
		t.Fatalf("tag add: %q", out)
	}
	h.mustRun("notebook", "pin", board)
	out := h.mustRun("notebook", "list")
	if strings.Index(out, "board") > strings.Index(out, "ops") || !strings.Contains(out, "[Infra]") {
		t.Fatalf("list (pinned first, tags shown): %q", out)
	}
	out = h.mustRun("notebook", "list", "--tag", "infra")
	if strings.Contains(out, "board") || !strings.Contains(out, "ops") {
		t.Fatalf("list --tag: %q", out)
	}
	h.mustRun("notebook", "tag", ops, "remove", "infra")
	h.mustRun("notebook", "unpin", board)
	if out := h.mustRun("notebook", "list", "--tag", "infra"); strings.Contains(out, "ops") {
		t.Fatalf("after remove: %q", out)
	}
	for _, args := range [][]string{{"notebook", "tag", ops}, {"notebook", "tag", ops, "add", "a/b"}} {
		if code := h.run(args...); code == ExitOK {
			t.Errorf("%v: want an error", args)
		}
	}
}
