package service

import (
	"context"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// recording returns the summary and records every request.
type recording struct{ reqs []llm.Request }

func (r *recording) ChatStream(_ context.Context, req llm.Request, _ func(string)) (llm.Result, error) {
	r.reqs = append(r.reqs, req)
	return llm.Result{Response: llm.Response{Content: summary}, Target: "litellm/m"}, nil
}

func mcpContextSvc(t *testing.T, mode string) (*Service, string, *recording) {
	t.Helper()
	s := mcpSvc(t)
	m := &recording{}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	if _, err := s.AddMCPServer(fakeServer(t, "fake", mode)); err != nil {
		t.Fatal(err)
	}
	info, err := s.CreateNotebook("Board", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	return s, info.ID, m
}

func (s *Service) runNB(t *testing.T, id string) (RunOutcome, error) {
	t.Helper()
	nb, err := s.OpenNotebook(id)
	if err != nil {
		t.Fatal(err)
	}
	return s.Run(context.Background(), RunRequest{Notebook: nb, Save: true}, nil)
}

func TestPipelineRunReadsMCPResource(t *testing.T) {
	s, id, m := mcpContextSvc(t, mcptest.ModeOK)
	info, err := s.AddMCPContext(id, "fake", "file:///notes.md", "")
	if err != nil {
		t.Fatal(err)
	}
	if c := info.Context[0]; c.Kind != "mcp-resource" || c.Label != "file:///notes.md" || c.EstimatedTokens != 0 {
		t.Fatalf("context item: %+v", c)
	}
	out, err := s.runNB(t, id)
	if err != nil {
		t.Fatal(err)
	}
	if user := m.reqs[0].Messages[1].Content; !strings.Contains(user, "Ship in March") {
		t.Fatalf("resource text not in the request:\n%s", user)
	}
	nb, _ := s.OpenNotebook(id)
	run, _ := nb.LoadRun(out.RunID)
	if len(run.Context) != 1 || run.Context[0] != "mcp:fake/file:///notes.md" {
		t.Fatalf("run context: %v", run.Context)
	}
}

func TestMCPResourceErrorFailsRun(t *testing.T) {
	s, id, _ := mcpContextSvc(t, mcptest.ModeFail)
	if _, err := s.AddMCPContext(id, "fake", "file:///notes.md", "Notes"); err != nil {
		t.Fatal(err)
	}
	_, err := s.runNB(t, id)
	if err == nil || !strings.Contains(err.Error(), "fake") || !strings.Contains(err.Error(), "file:///notes.md") {
		t.Fatalf("want an error naming server and URI, got %v", err)
	}
}

func TestMCPResourceTooLarge(t *testing.T) {
	s, id, _ := mcpContextSvc(t, mcptest.ModeOK)
	if _, err := s.AddMCPContext(id, "fake", "file:///big", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runNB(t, id); err == nil || !strings.Contains(err.Error(), "2 MiB") {
		t.Fatalf("too large: %v", err)
	}
}

func TestAddMCPContextValidates(t *testing.T) {
	s, id, _ := mcpContextSvc(t, mcptest.ModeOK)
	if _, err := s.AddMCPContext(id, "nope", "file:///x", ""); !IsInvalid(err) {
		t.Fatalf("unknown server: %v", err)
	}
	if _, err := s.AddMCPContext(id, "fake", strings.Repeat("u", 2049), ""); !IsInvalid(err) {
		t.Fatalf("long uri: %v", err)
	}
	if _, err := s.AddMCPContext(id, "fake", "", ""); !IsInvalid(err) {
		t.Fatalf("empty uri: %v", err)
	}
}

func TestDisabledMCPResourceNotRead(t *testing.T) {
	counts := countMCP(t)
	s, id, _ := mcpContextSvc(t, mcptest.ModeOK)
	info, _ := s.AddMCPContext(id, "fake", "file:///notes.md", "")
	if _, err := s.SetContextEnabled(id, info.Context[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runNB(t, id); err != nil {
		t.Fatal(err)
	}
	if o, _ := counts(); o != 0 {
		t.Fatalf("a disabled resource connected %d times", o)
	}
}
