package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunsAreAudited(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	nbDir := filepath.Join(h.dir, "nb")
	h.mustRun("new", nbDir, "--type", "summarize")
	h.mustRun("run", nbDir, "--prompt", "secret plan for Q3", "--quiet")

	logPath := filepath.Join(nbDir, AuditFile)
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret plan") {
		t.Fatal("prompt text must not be stored in the audit log")
	}
	for _, kind := range []string{"run.start", "model.attempt", "run.finish", "output.saved", "prompt_sha256"} {
		if !strings.Contains(string(raw), kind) {
			t.Errorf("audit log missing %q", kind)
		}
	}
	if out := h.mustRun("audit", "verify", nbDir); !strings.Contains(out, "ok: 5 entries") {
		t.Fatal(out)
	}
	if out := h.mustRun("audit", "show", nbDir, "--last", "2"); strings.Count(out, "\n") != 2 ||
		!strings.Contains(out, "output.saved") {
		t.Fatal(out)
	}
	tampered := strings.Replace(string(raw), `"valid":true`, `"valid":false`, 1)
	_ = os.WriteFile(logPath, []byte(tampered), 0o600)
	code := h.run("audit", "verify", logPath)
	if code != ExitError || !strings.Contains(h.errOut.String(), "modified") {
		t.Fatalf("want tamper detection, got %d %s", code, h.errOut)
	}
}

func TestNotebookLessRunUsesUserAuditLog(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	h.mustRun("run", "--type", "summarize", "--quiet")
	if out := h.mustRun("audit", "verify"); !strings.Contains(out, filepath.Join(h.dir, AuditFile)) {
		t.Fatal(out)
	}
	if code := h.run("audit"); code != ExitUsage {
		t.Fatal("want usage error")
	}
	if code := h.run("audit", "zap"); code != ExitUsage {
		t.Fatal("want usage error")
	}
	if code := h.run("audit", "show", filepath.Join(h.dir, "missing.jsonl")); code != ExitError {
		t.Fatal("want missing file error")
	}
}

func TestAuditOpenFailureStopsRun(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	nbDir := filepath.Join(h.dir, "nb")
	h.mustRun("new", nbDir, "--type", "summarize")
	_ = os.WriteFile(filepath.Join(nbDir, AuditFile), []byte("{corrupt\n"), 0o600)
	if code := h.run("run", nbDir, "--quiet"); code != ExitError || !strings.Contains(h.errOut.String(), "audit") {
		t.Fatalf("want audit error, got %d %s", code, h.errOut)
	}
}
