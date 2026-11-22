package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputExport(t *testing.T) {
	h, dir, id := outputHarness(t)
	docx := filepath.Join(h.dir, "digest.docx")
	h.mustRun("output", "export", dir, id, "--format", "docx", "--out", docx)
	b, err := os.ReadFile(docx)
	if err != nil || len(b) < 4 || string(b[:2]) != "PK" {
		t.Fatalf("not a docx (zip): %v %d bytes", err, len(b))
	}
	md := filepath.Join(h.dir, "digest.md")
	h.mustRun("output", "export", dir, id, "--out", md)
	if b, _ := os.ReadFile(md); !strings.Contains(string(b), "Digest") {
		t.Fatalf("markdown: %q", b)
	}
	if code := h.run("output", "export", dir, id, "--format", "pdf"); code != ExitUsage {
		t.Fatalf("pdf: exit %d", code)
	}
}
