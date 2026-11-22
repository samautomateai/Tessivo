package cli

import (
	"archive/zip"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsCommand(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	out := filepath.Join(h.dir, "diag.zip")
	if got := h.mustRun("diagnostics", "--out", out); !strings.Contains(got, out) {
		t.Fatalf("output: %q", got)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if len(r.File) != 4 {
		t.Fatalf("files: %d", len(r.File))
	}
}
