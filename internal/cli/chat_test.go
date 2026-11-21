package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

// chatHarness is a harness with a config whose workspace (and so the chat
// store) lives in the test's temp dir.
func chatHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.initConfig()
	f, _ := config.Load(h.env.ConfigPath)
	f.WorkspaceDir = filepath.Join(h.dir, "ws")
	if err := f.Save(h.env.ConfigPath); err != nil {
		t.Fatal(err)
	}
	return h
}

var sessionLine = regexp.MustCompile(`session ([0-9a-f]{16})`)

func (h *harness) newChat(args ...string) string {
	h.t.Helper()
	out := h.mustRun(append([]string{"chat"}, args...)...)
	m := sessionLine.FindStringSubmatch(h.errOut.String())
	if m == nil {
		h.t.Fatalf("no session id in %q (stdout %q)", h.errOut, out)
	}
	return m[1]
}

func TestChatSendListShow(t *testing.T) {
	h := chatHarness(t)
	h.model.reply = "Rotate keys monthly."
	doc := filepath.Join(h.dir, "runbook.md")
	_ = os.WriteFile(doc, []byte("# Runbook"), 0o600)
	id := h.newChat("--thinking", "low", "--attach", doc, "How", "often", "do", "we", "rotate?")
	if !strings.Contains(h.out.String(), "Rotate keys monthly.") {
		t.Fatalf("reply not printed: %q", h.out)
	}
	if req := h.model.reqs[0]; req.ReasoningEffort != "low" ||
		!strings.Contains(req.Messages[len(req.Messages)-1].Content, "# Runbook") {
		t.Fatalf("request %+v", req)
	}
	h.env.StdinIsTTY = false
	h.env.Stdin = strings.NewReader("And who approves?")
	h.mustRun("chat", "--session", id)
	h.env.StdinIsTTY = true

	if out := h.mustRun("chat", "list"); !strings.Contains(out, id) || !strings.Contains(out, "4 msgs") {
		t.Fatalf("list: %q", out)
	}
	if out := h.mustRun("chat", "list", "--search", "approves"); !strings.Contains(out, id) {
		t.Fatalf("search: %q", out)
	}
	out := h.mustRun("chat", "show", id)
	if !strings.Contains(out, "# How often do we rotate?") || !strings.Contains(out, "[path: runbook.md") {
		t.Fatalf("show: %q", out)
	}
	h.mustRun("chat", "rename", id, "Key", "rotation")
	h.mustRun("chat", "pin", id)
	if out := h.mustRun("chat", "list"); !strings.Contains(out, "* "+id+"  Key rotation") {
		t.Fatalf("pinned rename: %q", out)
	}
	h.mustRun("chat", "archive", id)
	if out := h.mustRun("chat", "list"); strings.Contains(out, id) {
		t.Fatalf("archived still listed: %q", out)
	}
	if out := h.mustRun("chat", "list", "--archived"); !strings.Contains(out, id) {
		t.Fatalf("archived list: %q", out)
	}
	h.mustRun("chat", "unarchive", id)
	h.mustRun("chat", "unpin", id)
	h.mustRun("chat", "delete", id)
	if code := h.run("chat", "show", id); code != ExitError {
		t.Fatalf("show deleted: %d", code)
	}
}

func TestChatInteractiveLoop(t *testing.T) {
	h := chatHarness(t)
	h.model.reply = "ok"
	h.env.Stdin = strings.NewReader("first\n\nsecond\n/quit\nnever sent\n")
	h.newChat()
	if len(h.model.reqs) != 2 || strings.Count(h.out.String(), "ok\n") != 2 {
		t.Fatalf("loop: %d requests, out %q", len(h.model.reqs), h.out)
	}
	h.model.err = errFake
	h.env.Stdin = strings.NewReader("fails\n")
	h.newChat()
	if !strings.Contains(h.errOut.String(), "tess:") {
		t.Fatalf("error not reported: %q", h.errOut)
	}
}

func TestChatMemoryRepeatAndSave(t *testing.T) {
	h := chatHarness(t)
	h.mustRun("chat", "memory", "add", "I", "lead", "payments")
	h.mustRun("chat", "memory", "add", "Prefer tables")
	h.mustRun("chat", "memory", "remove", "1")
	if out := h.mustRun("chat", "memory", "list"); out != "1. Prefer tables\n" {
		t.Fatalf("memory: %q", out)
	}
	id := h.newChat("Summarize", "the", "board", "pack")
	if !strings.Contains(h.model.reqs[0].Messages[0].Content, "Prefer tables") {
		t.Fatal("chat memory not sent")
	}
	out := h.mustRun("chat", "repeat", id, "--name", "Board pack")
	if !strings.Contains(out, "created notebook board-pack (summarize)") {
		t.Fatalf("repeat: %q", out)
	}
	out = h.mustRun("chat", "save", id, "last", "--type", "summarize")
	if !strings.Contains(out, `saved "Digest" as output`) || !strings.Contains(out, "chat-summarize") {
		t.Fatalf("save: %q", out)
	}
}

func TestChatUsageErrors(t *testing.T) {
	h := chatHarness(t)
	id := h.newChat("hello")
	for _, args := range [][]string{
		{"chat", "show"},
		{"chat", "rename", id},
		{"chat", "pin"},
		{"chat", "delete"},
		{"chat", "memory"},
		{"chat", "memory", "add"},
		{"chat", "memory", "remove", "9"},
		{"chat", "memory", "bogus"},
		{"chat", "repeat"},
		{"chat", "repeat", id, "--type", "poetry"},
		{"chat", "save", id},
		{"chat", "save", id, "last", "--type", "poetry"},
		{"chat", "--model", "nope", "hi"},
		{"chat", "--bogus"},
		{"chat", "list", "--bogus"},
		{"chat", strings.Repeat("x", 70<<10)},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d (%s)", args[:min(len(args), 3)], code, h.errOut)
		}
	}
	if code := h.run("chat", "save", id, "nope", "--type", "summarize"); code != ExitError {
		t.Errorf("missing message: %d", code)
	}
	if out := h.mustRun("chat", "-h"); out != "" || !strings.Contains(h.errOut.String(), "tess chat repeat") {
		t.Errorf("help: %q %q", out, h.errOut)
	}
}

func TestChatNeedsConfig(t *testing.T) {
	h := newHarness(t)
	if code := h.run("chat", "list"); code != ExitError {
		t.Fatalf("no config: %d", code)
	}
}

var errFake = fakeErr("provider down")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
