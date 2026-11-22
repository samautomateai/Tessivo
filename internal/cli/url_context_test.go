package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestContextAddURLAndText(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(h.dir, "ops")
	h.mustRun("new", dir, "--type", "summarize")
	if out := h.mustRun("context", "add", dir, "https://example.com/status"); !strings.Contains(out, "web page") {
		t.Fatalf("url: %q", out)
	}
	out := h.mustRun("context", "add", dir, "--text", "Budget is 10k", "--label", "Notes")
	if !strings.Contains(out, "Notes") {
		t.Fatalf("text: %q", out)
	}
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("From a pipe"), false
	h.mustRun("context", "add", dir, "--text", "-")
	out = h.mustRun("context", "list", dir)
	if !strings.Contains(out, "example.com/status") || !strings.Contains(out, "Notes") {
		t.Fatalf("list: %q", out)
	}
	if code := h.run("context", "add", dir, "https://user:pw@example.com/"); code != ExitUsage {
		t.Fatalf("credentials in URL: exit %d", code)
	}
}
