package cli

import (
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

// addFake runs `tess mcp add` for the fake server in mode.
func (h *harness) addFake(name, mode string) {
	h.t.Helper()
	prog, args, env := mcptest.Command(h.t, mode)
	cmd := []string{"mcp", "add", name, "--command", prog}
	for _, a := range args {
		cmd = append(cmd, "--arg="+a)
	}
	for k, v := range env {
		cmd = append(cmd, "--env", k+"="+v)
	}
	h.mustRun(cmd...)
}

func TestMCPAddListTestRemove(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	h.addFake("fake", mcptest.ModeOK)
	if out := h.mustRun("mcp", "list"); !strings.Contains(out, "fake") || !strings.Contains(out, "stdio") {
		t.Fatalf("list: %q", out)
	}
	out := h.mustRun("mcp", "test", "fake")
	for _, want := range []string{"2 tools", "echo", "1 resource", "file:///notes.md", "1 prompt", "review"} {
		if !strings.Contains(out, want) {
			t.Errorf("test output lacks %q:\n%s", want, out)
		}
	}
	h.mustRun("mcp", "remove", "fake")
	if out := h.mustRun("mcp", "list"); strings.Contains(out, "fake") {
		t.Fatalf("not removed: %q", out)
	}
}

func TestMCPAddRejectsLiteralToken(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	code := h.run("mcp", "add", "gh", "--url", "https://x.com/mcp", "--header", "Authorization=Bearer ghp_"+
		strings.Repeat("a", 36))
	if code != ExitUsage || strings.Contains(h.errOut.String(), "ghp_") {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	h.mustRun("mcp", "add", "gh", "--url", "https://x.com/mcp", "--header", "Authorization=Bearer ${secret:GH}")
}

func TestMCPUsage(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	for _, args := range [][]string{
		{"mcp"},
		{"mcp", "bogus"},
		{"mcp", "add"},
		{"mcp", "add", "x"},
		{"mcp", "test"},
		{"mcp", "remove"},
		{"mcp", "add", "x", "--command", "a", "--env", "noequals"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage exit, got %d (%s)", args, code, h.errOut)
		}
	}
	if code := h.run("mcp", "remove", "missing"); code != ExitError {
		t.Errorf("remove missing: %d", code)
	}
}
