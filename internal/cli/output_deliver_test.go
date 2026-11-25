package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

const cliAnalysis = `{"title":"Review","summary":"s","risk_score":7,"risk_level":"high","findings":[{"title":` +
	`"Retries double charge","severity":"high","detail":"d","steps":["a"],"labels":["payments"]}],` +
	`"impacted_areas":[],"evidence":[],"mitigations":[],"open_questions":[]}`

// analyzeHarness has an Analyze notebook folder with one saved output.
func analyzeHarness(t *testing.T) (h *harness, dir, outputID string) {
	t.Helper()
	h = newHarness(t)
	h.initConfig()
	h.model.reply = cliAnalysis
	dir = filepath.Join(h.dir, "pay")
	h.mustRun("new", dir, "--type", "analyze")
	h.mustRun("run", dir, "--prompt", "Review the change.")
	nb, err := notebook.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := nb.ListOutputs()
	if err != nil || len(outs) != 1 {
		t.Fatalf("seed output: %v %d", err, len(outs))
	}
	return h, dir, outs[0].ID
}

func TestOutputIssues(t *testing.T) {
	h, dir, out := analyzeHarness(t)
	got := h.mustRun("output", "issues", dir, out)
	if !strings.Contains(got, "1. [high] Retries double charge") || !strings.Contains(got, "payments, severity:high") {
		t.Fatalf("issues: %q", got)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(h.mustRun("output", "issues", dir, out, "--json")), &issues); err != nil ||
		len(issues) != 1 || issues[0]["title"] != "Retries double charge" {
		t.Fatalf("--json: %v %v", err, issues)
	}
}
