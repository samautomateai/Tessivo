package cli

import (
	"strings"
	"testing"
)

func TestIntegrationCLI(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	out := h.mustRun("integration", "add", "gh", "--kind", "github", "--project", "acme/shop",
		"--token-ref", "GH_TOKEN")
	if !strings.Contains(out, "tess secret set GH_TOKEN") {
		t.Fatalf("add tells how to store the token: %q", out)
	}
	h.mustRun("integration", "add", "share", "--kind", "folder", "--path", h.dir)
	got := h.mustRun("integration", "list")
	if !strings.Contains(got, "gh") || !strings.Contains(got, "no token") || !strings.Contains(got, "token set") {
		t.Fatalf("list: %q", got)
	}
	h.mustRun("integration", "remove", "gh")
	if got := h.mustRun("integration", "list"); strings.Contains(got, "acme/shop") {
		t.Fatalf("after remove: %q", got)
	}
	for _, args := range [][]string{
		{"integration"},
		{"integration", "nope"},
		{"integration", "remove"},
		{"integration", "add", "x", "--kind", "trello"},
		{"integration", "add"},
	} {
		if code := h.run(args...); code == ExitOK {
			t.Errorf("%v: want an error", args)
		}
	}
}
