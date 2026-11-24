package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// chatScript answers with the next scripted response and records requests.
type chatScript struct {
	mu    sync.Mutex
	steps []llm.Response
	reqs  []llm.Request
}

func (m *chatScript) ChatStream(_ context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs = append(m.reqs, req)
	resp := llm.Response{Content: "Done."}
	if len(m.steps) > 0 {
		resp, m.steps = m.steps[0], m.steps[1:]
	}
	if onDelta != nil && resp.Content != "" {
		onDelta(resp.Content)
	}
	return llm.Result{Response: resp, Target: "litellm/m"}, nil
}

func chatToolsEnv(t *testing.T, steps ...llm.Response) (*Service, *chatScript, string) {
	t.Helper()
	s := mcpSvc(t)
	m := &chatScript{steps: steps}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	if _, err := s.AddMCPServer(fakeServer(t, "fake", mcptest.ModeOK)); err != nil {
		t.Fatal(err)
	}
	sess, err := s.CreateChat(ChatInput{Title: "Tools"})
	if err != nil {
		t.Fatal(err)
	}
	servers := []string{"fake"}
	if _, err := s.UpdateChat(sess.ID, ChatPatch{MCPServers: &servers}); err != nil {
		t.Fatal(err)
	}
	return s, m, sess.ID
}

func TestChatUsesAllowedMCPTools(t *testing.T) {
	s, m, id := chatToolsEnv(t, echoCall("hello"), llm.Response{Content: "The tool said hello."})
	s.Config.Agent.Tools = map[string]string{"mcp__fake__echo": config.RuleAllow}
	var events []ChatToolEvent
	reply, err := s.SendChat(context.Background(), id, ChatSend{
		Text: "Echo hello", OnTool: func(e ChatToolEvent) { events = append(events, e) },
	}, nil)
	if err != nil || reply.Assistant.Content != "The tool said hello." {
		t.Fatalf("%v %+v", err, reply.Assistant)
	}
	if len(m.reqs) != 2 || len(m.reqs[0].Tools) == 0 {
		t.Fatalf("requests %d, tools %d", len(m.reqs), len(m.reqs[0].Tools))
	}
	last := m.reqs[1].Messages
	if r := last[len(last)-1]; r.Role != llm.RoleTool || !strings.Contains(r.Content, "hello") {
		t.Fatalf("tool result not sent back: %+v", r)
	}
	if len(events) != 2 || events[0].Kind != "call" || events[1].Kind != "result" || events[1].By != "rule" {
		t.Fatalf("events %+v", events)
	}
	sess, _ := s.GetChat(id)
	note := sess.Messages[len(sess.Messages)-1]
	if note.Role != "note" || !strings.Contains(note.Content, "mcp__fake__echo") {
		t.Fatalf("no note of the tools used: %+v", sess.Messages)
	}
}

func TestChatAsksAndRemembers(t *testing.T) {
	s, m, id := chatToolsEnv(t, echoCall("a"), llm.Response{Content: "ok"}, echoCall("b"), llm.Response{Content: "ok"})
	asked := 0
	approve := func(ChatToolRequest) ChatToolAnswer { asked++; return ChatToolAnswer{Allow: true, Remember: true} }
	if _, err := s.SendChat(context.Background(), id, ChatSend{Text: "one", Approve: approve}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendChat(context.Background(), id, ChatSend{Text: "two", Approve: approve}, nil); err != nil {
		t.Fatal(err)
	}
	if asked != 1 {
		t.Fatalf("asked %d times; the second call should use the remembered rule", asked)
	}
	if sess, _ := s.GetChat(id); sess.ToolRules["mcp__fake__echo"] != config.RuleAllow {
		t.Fatalf("rule not remembered: %+v", sess.ToolRules)
	}
	_ = m
}

func TestChatDeniesWithoutAnApproverAndBlocks(t *testing.T) {
	s, m, id := chatToolsEnv(t, echoCall("x"), llm.Response{Content: "I could not use it."})
	if _, err := s.SendChat(context.Background(), id, ChatSend{Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	msgs := m.reqs[1].Messages
	if r := msgs[len(msgs)-1]; !strings.Contains(r.Content, "not approved") {
		t.Fatalf("denied call must tell the model: %+v", r)
	}
	s2, m2, id2 := chatToolsEnv(t, echoCall("x"), llm.Response{Content: "blocked"})
	s2.Config.Agent.Tools = map[string]string{"mcp__fake__*": config.RuleBlock}
	if _, err := s2.SendChat(context.Background(), id2, ChatSend{Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	msgs = m2.reqs[1].Messages
	if r := msgs[len(msgs)-1]; !strings.Contains(r.Content, "blocked") {
		t.Fatalf("blocked call: %+v", r)
	}
}

func TestChatToolStepsAreBounded(t *testing.T) {
	steps := make([]llm.Response, 20)
	for i := range steps {
		steps[i] = echoCall("again")
	}
	s, m, id := chatToolsEnv(t, steps...)
	s.Config.Agent.Tools = map[string]string{"mcp__fake__echo": config.RuleAllow}
	if _, err := s.SendChat(context.Background(), id, ChatSend{Text: "loop"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(m.reqs) != maxChatToolSteps+1 || len(m.reqs[maxChatToolSteps].Tools) != 0 {
		t.Fatalf("%d requests; the last must offer no tools", len(m.reqs))
	}
}

func TestChatMCPServersMustBeConfigured(t *testing.T) {
	s, _, id := chatToolsEnv(t)
	bad := []string{"nope"}
	if _, err := s.UpdateChat(id, ChatPatch{MCPServers: &bad}); !IsInvalid(err) {
		t.Fatalf("unknown server: %v", err)
	}
}

func TestChatResultSaysWhoAllowedIt(t *testing.T) {
	s, _, id := chatToolsEnv(t, echoCall("a"), llm.Response{Content: "ok"})
	var events []ChatToolEvent
	approve := func(ChatToolRequest) ChatToolAnswer { return ChatToolAnswer{Allow: true} }
	send := ChatSend{Text: "one", Approve: approve, OnTool: func(e ChatToolEvent) { events = append(events, e) }}
	if _, err := s.SendChat(context.Background(), id, send, nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Kind != "result" || events[1].By != "you" {
		t.Fatalf("events %+v", events)
	}
}
