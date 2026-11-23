package cli

import (
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestSearchCommand(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	f, _ := config.Load(h.env.ConfigPath)
	if code := h.run("search"); code != ExitUsage {
		t.Fatalf("no words: exit %d", code)
	}
	if out := h.mustRun("search", "nothing"); !strings.Contains(out, "KIND") {
		t.Fatalf("empty search: %q", out)
	}
	_ = f
}
