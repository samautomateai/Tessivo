package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// queueModel answers from a queue of replies.
type queueModel struct {
	mu      sync.Mutex
	replies []llm.Response
}

func (m *queueModel) ChatStream(context.Context, llm.Request, func(string)) (llm.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.replies) == 0 {
		return llm.Result{}, errors.New("no reply queued")
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return llm.Result{Response: r, Target: "litellm/m"}, nil
}

func grepReply() llm.Response {
	return llm.Response{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "grep", Arguments: `{"pattern":"x"}`}}}
}

// agentNotebook makes a notebook with a code folder and a model that asks for
// grep, then finishes.
func agentNotebook(t *testing.T, h *harness) string {
	t.Helper()
	h.initConfig()
	m := &queueModel{replies: []llm.Response{grepReply(), {Content: "done"}, {Content: summary}}}
	h.env.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	nb := filepath.Join(h.dir, "nb")
	h.mustRun("new", nb, "--type", "summarize")
	src := filepath.Join(h.dir, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.mustRun("context", "add", nb, src)
	return nb
}

var idsInPause = regexp.MustCompile(`run (\S+) needs approval (\S+)`)

func pauseIDs(t *testing.T, stderr string) (string, string) {
	t.Helper()
	m := idsInPause.FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("no run and approval ids in: %s", stderr)
	}
	return m[1], m[2]
}

func TestRunAgentPausedExit4(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.StdinIsTTY = false
	if code := h.run("run", nb, "--mode", "agent", "--tool", "grep=ask"); code != ExitPaused {
		t.Fatalf("exit %d, stderr: %s", code, h.errOut)
	}
	run, aid := pauseIDs(t, h.errOut.String())
	if !strings.Contains(h.errOut.String(), "tess run approve "+run+" "+aid+" --notebook") {
		t.Fatalf("the pause message must give the exact command: %s", h.errOut)
	}
}

func TestRunApproveResumes(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.StdinIsTTY = false
	h.run("run", nb, "--mode", "agent", "--tool", "grep=ask")
	run, aid := pauseIDs(t, h.errOut.String())
	out := h.mustRun("run", "approve", run, aid, "--notebook", nb)
	if !strings.Contains(out, "# Digest") {
		t.Fatalf("approve must resume and print the output: %s", out)
	}
	if code := h.run("run", "approve", run, aid, "--notebook", nb); code == ExitOK {
		t.Fatal("approving a finished run must fail")
	}
}

func TestRunDenyAndCancel(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.StdinIsTTY = false
	h.run("run", nb, "--mode", "agent", "--tool", "grep=ask")
	run, _ := pauseIDs(t, h.errOut.String())
	h.mustRun("run", "cancel", run, "--notebook", nb)
	if code := h.run("run", "resume", run, "--notebook", nb); code == ExitOK {
		t.Fatal("resuming a cancelled run must fail")
	}
	h2 := newHarness(t)
	nb2 := agentNotebook(t, h2)
	h2.env.StdinIsTTY = false
	h2.run("run", nb2, "--mode", "agent", "--tool", "grep=ask")
	run2, aid2 := pauseIDs(t, h2.errOut.String())
	if out := h2.mustRun("run", "deny", run2, aid2, "--notebook", nb2); !strings.Contains(out, "# Digest") {
		t.Fatalf("deny must resume without the tool: %s", out)
	}
}

func TestRunPendingTable(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.StdinIsTTY = false
	h.run("run", nb, "--mode", "agent", "--tool", "grep=ask")
	run, aid := pauseIDs(t, h.errOut.String())
	out := h.mustRun("run", "pending", "--notebook", nb)
	for _, want := range []string{"RUN", "STATUS", "waiting_approval", run, aid, "grep"} {
		if !strings.Contains(out, want) {
			t.Fatalf("pending lacks %q: %s", want, out)
		}
	}
}

func TestRunInteractiveApproval(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("y\n"), true
	out := h.mustRun("run", nb, "--mode", "agent", "--tool", "grep=ask")
	if !strings.Contains(out, "# Digest") || !strings.Contains(h.errOut.String(), "allow grep") {
		t.Fatalf("stdout %s stderr %s", out, h.errOut)
	}
}

func TestRunToolFlag(t *testing.T) {
	h := newHarness(t)
	nb := agentNotebook(t, h)
	h.env.StdinIsTTY = false
	if out := h.mustRun("run", nb, "--mode", "agent", "--tool", "grep=allow"); !strings.Contains(out, "# Digest") {
		t.Fatalf("an allowed tool must not pause: %s", out)
	}
	if code := h.run("run", nb, "--tool", "grep"); code != ExitUsage {
		t.Fatalf("--tool without =rule: exit %d", code)
	}
}

func TestRunSubcommandPrecedence(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	if err := os.MkdirAll(filepath.Join(h.dir, "pending"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(h.dir)
	if out := h.mustRun("run", "pending"); !strings.Contains(out, "no approvals") {
		t.Fatalf("pending must run the subcommand: %s", out)
	}
}
