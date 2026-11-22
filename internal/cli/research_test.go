package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResearchCLIFlow(t *testing.T) {
	h := chatHarness(t)
	h.model.reply = "A short summary."
	out := h.mustRun("research", "new", "Board", "papers", "--description", "Q3 board")
	if !strings.Contains(out, "board-papers") {
		t.Fatalf("new: %q", out)
	}
	src := filepath.Join(h.dir, "minutes.md")
	_ = os.WriteFile(src, []byte("Decided to ship in March."), 0o600)
	if out = h.mustRun("research", "source", "add", "board-papers", src); !strings.Contains(out, "summary ready") {
		t.Fatalf("add: %q", out)
	}
	if out = h.mustRun("research", "source", "list", "board-papers"); !strings.Contains(out, "summary") ||
		!strings.Contains(out, "minutes.md") {
		t.Fatalf("list: %q", out)
	}
	if out = h.mustRun("research", "list"); !strings.Contains(out, "board-papers") {
		t.Fatalf("research list: %q", out)
	}
	h.model.reply = "Ship in March [1]."
	out = h.mustRun("research", "ask", "board-papers", "When", "do", "we", "ship?")
	if !strings.Contains(out, "Ship in March [1].") || !strings.Contains(out, "References:") ||
		!strings.Contains(out, "[1] minutes.md (summary)") {
		t.Fatalf("ask: %q", out)
	}
	h.mustRun("research", "note", "add", "board-papers", "--title", "Mine", "Remember", "the", "audit")
	if out = h.mustRun("research", "note", "list", "board-papers"); !strings.Contains(out, "Mine") {
		t.Fatalf("notes: %q", out)
	}
	if out = h.mustRun("research", "show", "board-papers"); !strings.Contains(out, "Q3 board") {
		t.Fatalf("show: %q", out)
	}
	h.mustRun("research", "delete", "board-papers")
	if out = h.mustRun("research", "list"); strings.Contains(out, "board-papers") {
		t.Fatalf("not deleted: %q", out)
	}
}

func TestResearchCLIUsage(t *testing.T) {
	h := chatHarness(t)
	for _, args := range [][]string{
		{"research"},
		{"research", "bogus"},
		{"research", "new"},
		{"research", "source"},
		{"research", "source", "add", "x"},
		{"research", "ask", "x"},
		{"research", "note", "add", "x"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage exit, got %d (%s)", args, code, h.errOut)
		}
	}
}
