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
	pdf := filepath.Join(h.dir, "digest.pdf")
	h.mustRun("output", "export", dir, id, "--format", "pdf", "--out", pdf)
	if b, _ := os.ReadFile(pdf); !strings.HasPrefix(string(b), "%PDF-") {
		t.Fatal("not a PDF")
	}
	if code := h.run("output", "export", dir, id, "--format", "rtf"); code != ExitUsage {
		t.Fatalf("rtf: exit %d", code)
	}
}

func TestOutputRerun(t *testing.T) {
	h, dir, id := outputHarness(t)
	if out := h.mustRun("output", "rerun", dir, id, "--model", "litellm/m"); !strings.Contains(out, "Digest") {
		t.Fatalf("rerun: %q", out)
	}
	if !strings.Contains(h.errOut.String(), "saved output") {
		t.Fatalf("stderr: %q", h.errOut.String())
	}
	if code := h.run("output", "rerun", dir, "nope"); code == ExitOK {
		t.Fatal("rerun of a missing output succeeded")
	}
}

func TestOutputDiff(t *testing.T) {
	h, dir, id := outputHarness(t)
	if code := h.run("output", "diff", dir, id); code != ExitUsage {
		t.Fatalf("diff of an original: exit %d (%s)", code, h.errOut)
	}
	if out := h.mustRun("output", "diff", dir, id, "--against", id); !strings.Contains(out, "  ") {
		t.Fatalf("diff with itself: %q", out)
	}
	if !strings.Contains(h.errOut.String(), "0 added, 0 removed") {
		t.Fatalf("summary: %q", h.errOut.String())
	}
}
