package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/service"
)

func TestAskToolAnswers(t *testing.T) {
	var errOut bytes.Buffer
	env := Env{Stderr: &errOut}
	for line, want := range map[string]service.ChatToolAnswer{
		"y":      {Allow: true},
		"always": {Allow: true, Remember: true},
		"d":      {Remember: true},
		"n":      {},
		"":       {},
	} {
		got := askTool(env, func() (string, bool) { return line + "\n", true })(service.ChatToolRequest{Tool: "t"})
		if got != want {
			t.Errorf("%q: %+v, want %+v", line, got, want)
		}
	}
	if got := askTool(env, nil)(service.ChatToolRequest{Tool: "t"}); got.Allow ||
		!strings.Contains(errOut.String(), "needs approval") {
		t.Fatalf("no terminal must deny and explain: %+v %q", got, errOut.String())
	}
	toolLine(env)(service.ChatToolEvent{Kind: "call", Tool: "mcp__x__y", Args: "{}"})
	if !strings.Contains(errOut.String(), "[tool] mcp__x__y") {
		t.Fatalf("tool line: %q", errOut.String())
	}
}

func TestChatMCPFlagNeedsConfiguredServers(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	if code := h.run("chat", "--mcp", "nope", "hello"); code != ExitUsage {
		t.Fatalf("unknown server: exit %d (%s)", code, h.errOut)
	}
}
