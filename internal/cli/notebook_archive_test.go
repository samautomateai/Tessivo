package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestNotebookExportImportAndRelink(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	ws := filepath.Join(h.dir, "ws")
	f, _ := os.ReadFile(h.env.ConfigPath)
	var cfg map[string]any
	_ = json.Unmarshal(f, &cfg)
	cfg["workspace_dir"] = ws
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(h.env.ConfigPath, b, 0o600)

	src := filepath.Join(h.dir, "ops")
	h.mustRun("new", src, "--type", "summarize")
	folder := filepath.Join(h.dir, "code")
	_ = os.MkdirAll(folder, 0o700)
	_ = os.WriteFile(filepath.Join(folder, "a.go"), []byte("package a"), 0o600)
	h.mustRun("context", "add", src, folder)

	zip := filepath.Join(h.dir, "ops.tessivo.zip")
	if out := h.mustRun("notebook", "export", src, "-o", zip); !strings.Contains(out, zip) {
		t.Fatalf("export: %q", out)
	}
	out := h.mustRun("notebook", "import", zip)
	if !strings.Contains(out, "1 source needs relinking") {
		t.Fatalf("import: %q", out)
	}
	imported := filepath.Join(ws, "ops")
	nb, err := notebook.Open(imported)
	if err != nil || len(nb.Manifest.Context) != 1 || !nb.Manifest.Context[0].NeedsRelink {
		t.Fatalf("%v %+v", err, nb)
	}
	h.mustRun("context", "relink", imported, nb.Manifest.Context[0].ID, folder)
	nb, _ = notebook.Open(imported)
	if c := nb.Manifest.Context[0]; c.NeedsRelink || c.Path != folder {
		t.Fatalf("after relink: %+v", c)
	}
	for _, args := range [][]string{{"notebook", "export"}, {"notebook", "import"}, {"context", "relink", imported}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
