package service

import (
	"context"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
)

func TestNotebookCommandAllowList(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Build", "analyze")
	rules := []config.CommandRule{{Program: "go", Args: []string{"test", "*"}}}
	got, err := s.SetNotebookCommands(info.ID, rules)
	if err != nil || len(got.Commands) != 1 || got.Commands[0].Args[1] != "*" {
		t.Fatalf("%v %+v", err, got.Commands)
	}
	if again, _ := s.OpenNotebook(info.ID); len(again.Manifest.Agent.Commands) != 1 {
		t.Fatal("the allow-list must be saved")
	}
	if _, err := s.SetNotebookCommands(info.ID, []config.CommandRule{{Program: ""}}); !IsInvalid(err) {
		t.Fatalf("an empty program: %v", err)
	}
	if got, _ := s.SetNotebookCommands(info.ID, nil); len(got.Commands) != 0 {
		t.Fatal("nil clears the allow-list")
	}
}

func TestCommandEnvLeavesOutKeys(t *testing.T) {
	s := newSvc(t, summary)
	t.Setenv("TESSIVO_LITELLM_KEY", "sk-1")
	t.Setenv("TESSIVO_TEST_PLAIN", "fine")
	env := strings.Join(s.commandEnv(), "\n")
	if strings.Contains(env, "sk-1") || !strings.Contains(env, "TESSIVO_TEST_PLAIN=fine") {
		t.Fatalf("env: keys must be removed, other variables kept")
	}
}

func TestAgentRunsAnAllowListedCommand(t *testing.T) {
	call := llm.Response{ToolCalls: []llm.ToolCall{{
		ID: "c1", Name: agenttools.RunCommandName, Arguments: `{"program":"go","args":["version"]}`,
	}}}
	e := newAgentEnv(t, call, llm.Response{Content: "done"}, llm.Response{Content: summary})
	rules := []config.CommandRule{{Program: "go", Args: []string{"version"}}}
	if _, err := e.svc.SetNotebookCommands(e.nbID, rules); err != nil {
		t.Fatal(err)
	}
	out := e.start(t, context.Background())
	if len(out.Paused) != 0 || !out.Result.Valid {
		t.Fatalf("an allow-listed command must run without asking: %+v", out.Paused)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	run, err := nb.LoadRun(out.RunID)
	if err != nil || len(run.ToolCalls) != 1 {
		t.Fatalf("%v %+v", err, run.ToolCalls)
	}
	if tc := run.ToolCalls[0]; tc.Tool != agenttools.RunCommandName || !strings.Contains(tc.Result, "exit code 0") {
		t.Fatalf("tool call %+v", tc)
	}
}
