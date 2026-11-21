package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

func echoCall(text string) llm.Response {
	call := llm.ToolCall{ID: "m1", Name: "mcp__fake__echo", Arguments: `{"text":"` + text + `"}`}
	return llm.Response{ToolCalls: []llm.ToolCall{call}}
}

// attachFake configures the fake server in mode and attaches it to e's notebook.
func attachFake(t *testing.T, e *agentEnv, mode string) {
	t.Helper()
	if _, err := e.svc.AddMCPServer(fakeServer(t, "fake", mode)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetNotebookMCP(e.nbID, []string{"fake"}); err != nil {
		t.Fatal(err)
	}
}

func (e *agentEnv) run(t *testing.T, rules map[string]string) (RunOutcome, error) {
	t.Helper()
	nb, err := e.svc.OpenNotebook(e.nbID)
	if err != nil {
		t.Fatal(err)
	}
	return e.svc.Run(context.Background(), RunRequest{
		Notebook: nb, Save: true, ToolRules: rules, Overrides: config.RunSettings{Mode: config.ModeAgent},
	}, nil)
}

// countMCP counts connections opened and closed through the test hook.
func countMCP(t *testing.T) func() (opened, closed int) {
	t.Helper()
	var mu sync.Mutex
	o, c := 0, 0
	mcpHook = func(event string) {
		mu.Lock()
		defer mu.Unlock()
		if event == "open" {
			o++
		} else {
			c++
		}
	}
	t.Cleanup(func() { mcpHook = nil })
	return func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		return o, c
	}
}

func TestAgentRunCallsAttachedMCPTool(t *testing.T) {
	e := newAgentEnv(t, echoCall("from-mcp"), llm.Response{Content: "done"}, llm.Response{Content: summary})
	attachFake(t, e, mcptest.ModeOK)
	out, err := e.run(t, map[string]string{"mcp__fake__echo": "allow"})
	if err != nil || out.OutputID == "" {
		t.Fatalf("%v %+v", err, out)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	run, err := nb.LoadRun(out.RunID)
	if err != nil || len(run.ToolCalls) != 1 || run.ToolCalls[0].Tool != "mcp__fake__echo" ||
		!strings.Contains(run.ToolCalls[0].Result, "from-mcp") {
		t.Fatalf("run record: %+v %v", run.ToolCalls, err)
	}
	log, _ := os.ReadFile(filepath.Join(nb.Dir, AuditFile))
	if !strings.Contains(string(log), `"tool.call"`) || !strings.Contains(string(log), "mcp__fake__echo") {
		t.Fatal("tool call not audited")
	}
}

func TestAgentMCPDefaultsToAskAndReconnectsOnResume(t *testing.T) {
	counts := countMCP(t)
	e := newAgentEnv(t, echoCall("after-resume"), llm.Response{Content: "done"}, llm.Response{Content: summary})
	attachFake(t, e, mcptest.ModeOK)
	out, err := e.run(t, nil)
	if err != nil || len(out.Paused) != 1 || out.Paused[0].Tool != "mcp__fake__echo" {
		t.Fatalf("want a pause on the MCP tool: %v %+v", err, out.Paused)
	}
	if o, c := counts(); o != 1 || c != 1 {
		t.Fatalf("after pause: opened %d closed %d", o, c)
	}
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", ""); err != nil {
		t.Fatal(err)
	}
	done, err := e.svc.Resume(context.Background(), out.RunID, nil)
	if err != nil || done.OutputID == "" {
		t.Fatalf("resume: %v %+v", err, done)
	}
	if o, c := counts(); o != 2 || c != 2 {
		t.Fatalf("after resume: opened %d closed %d", o, c)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	run, _ := nb.LoadRun(out.RunID)
	if len(run.ToolCalls) != 1 || !strings.Contains(run.ToolCalls[0].Result, "after-resume") {
		t.Fatalf("tool did not run after resume: %+v", run.ToolCalls)
	}
}

func TestAgentAttachedServerDownFailsRun(t *testing.T) {
	e := newAgentEnv(t, llm.Response{Content: "done"})
	attachFake(t, e, mcptest.ModeFail)
	if _, err := e.run(t, nil); err == nil || !strings.Contains(err.Error(), "fake") {
		t.Fatalf("want an error naming the server, got %v", err)
	}
}

func TestSetNotebookMCP(t *testing.T) {
	e := newAgentEnv(t)
	if _, err := e.svc.SetNotebookMCP(e.nbID, []string{"nope"}); !IsInvalid(err) {
		t.Fatalf("unknown server: %v", err)
	}
	if _, err := e.svc.AddMCPServer(fakeServer(t, "fake", mcptest.ModeOK)); err != nil {
		t.Fatal(err)
	}
	info, err := e.svc.SetNotebookMCP(e.nbID, []string{"fake", "fake"})
	if err != nil || len(info.MCPServers) != 1 {
		t.Fatalf("%v %+v", err, info.MCPServers)
	}
	if info, err = e.svc.SetNotebookMCP(e.nbID, nil); err != nil || len(info.MCPServers) != 0 {
		t.Fatalf("detach all: %v %+v", err, info.MCPServers)
	}
}
