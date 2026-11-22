package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestUsageCommandAndWarning(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	f, _ := config.Load(h.env.ConfigPath)
	f.Limits.MaxTokensPerDay = 100
	f.WorkspaceDir = filepath.Join(h.dir, "ws") // its own usage.json
	_ = f.Save(h.env.ConfigPath)
	h.model.tokens = 90
	dir := filepath.Join(h.dir, "ops")
	h.mustRun("new", dir, "--type", "summarize")
	h.mustRun("run", dir, "--prompt", "Summarize")
	if !strings.Contains(h.errOut.String(), "warning: 80% of the daily token limit used") {
		t.Fatalf("no warning: %q", h.errOut.String())
	}
	out := h.mustRun("usage")
	if !strings.Contains(out, "90 tokens used (90%)") || !strings.Contains(out, "day limit: 100") {
		t.Fatalf("usage: %q", out)
	}
	if code := h.run("run", dir, "--prompt", "Again"); code != ExitOK {
		t.Fatalf("still under the limit: exit %d (%s)", code, h.errOut)
	}
	if code := h.run("run", dir, "--prompt", "Over"); code != ExitError ||
		!strings.Contains(h.errOut.String(), "max_tokens_per_day") {
		t.Fatalf("over the limit: exit %d (%s)", code, h.errOut)
	}
}
