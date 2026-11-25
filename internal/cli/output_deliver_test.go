package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
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

func TestOutputPushAndPublish(t *testing.T) {
	h, dir, out := analyzeHarness(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"number":9,"html_url":"https://gh.test/9"}`)
	}))
	defer ts.Close()
	h.env.Secrets = secrets.Map{"TESSIVO_LITELLM_KEY": "k", "GH": "t"}
	h.mustRun("integration", "add", "gh", "--kind", "github", "--url", ts.URL, "--project", "a/b", "--token-ref", "GH")
	share := t.TempDir()
	h.mustRun("integration", "add", "share", "--kind", "folder", "--path", share)
	if got := h.mustRun("output", "push", dir, out, "--to", "gh", "--only", "1"); got != "1. https://gh.test/9\n" {
		t.Fatalf("push: %q", got)
	}
	h.mustRun("output", "push", dir, out, "--to", "gh")
	if !strings.Contains(h.errOut.String(), "skipped 1 finding") {
		t.Fatalf("skipped note: %q", h.errOut)
	}
	if got := h.mustRun("output", "publish", dir, out, "--to", "share"); !strings.HasPrefix(got, "file:") {
		t.Fatalf("publish: %q", got)
	}
	if got := h.mustRun("integration", "test", "share"); got != "share: ok\n" {
		t.Fatalf("test: %q", got)
	}
	for _, args := range [][]string{
		{"output", "push", dir, out},
		{"output", "publish", dir, out},
		{"output", "push", dir, out, "--to", "gh", "--only", "x"},
		{"integration", "test"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
