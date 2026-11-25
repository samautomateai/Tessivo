package cli

import (
	"strings"
	"testing"
)

func TestProfileCLI(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	if out := h.mustRun("profile", "save", "work"); !strings.Contains(out, "* work") {
		t.Fatalf("save: %q", out)
	}
	h.mustRun("profile", "save", "home")
	out := h.mustRun("profile", "use", "work")
	if !strings.Contains(out, "* work") || !strings.Contains(out, "  home") {
		t.Fatalf("use: %q", out)
	}
	if out := h.mustRun("profile", "delete", "home"); strings.Contains(out, "home") {
		t.Fatalf("delete: %q", out)
	}
	h.mustRun("profile", "list")
	for _, args := range [][]string{
		{"profile"}, {"profile", "use"}, {"profile", "list", "x"}, {"profile", "zap", "x"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := h.run("profile", "use", "nope"); code == ExitOK {
		t.Fatal("unknown profile")
	}
}
