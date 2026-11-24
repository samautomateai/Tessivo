package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/executor/agent"
	"github.com/samautomateai/tessivo/internal/llm"
)

// Chat tool limits.
const (
	maxChatToolSteps  = 8        // model turns that may call tools before a final answer
	maxChatToolResult = 32 << 10 // bytes of one tool result sent back to the model
	chatToolTimeout   = 2 * time.Minute
)

// ChatToolEvent reports a tool call in a chat: call, result, denied or blocked.
// A result says who allowed the call: "rule" (an allow rule) or "you" (asked).
type ChatToolEvent struct {
	Kind    string `json:"kind"`
	Tool    string `json:"tool"`
	Args    string `json:"args,omitempty"`
	Summary string `json:"summary,omitempty"`
	By      string `json:"by,omitempty"`
}

// ChatToolRequest is a tool call waiting for the user's answer.
type ChatToolRequest struct {
	ID   string `json:"id"`
	Tool string `json:"tool"`
	Args string `json:"args"`
}

// ChatToolAnswer is the user's answer; Remember keeps it as this chat's rule.
type ChatToolAnswer struct {
	Allow    bool `json:"allow"`
	Remember bool `json:"remember"`
}

// chatTools runs one chat reply with the session's MCP tools: up to
// maxChatToolSteps model turns may call tools, then a last turn answers
// without them. It returns the reply and the tools it used.
type chatTools struct {
	s        *Service
	chatID   string
	in       ChatSend
	policy   agent.Policy
	reg      map[string]agenttools.Tool
	remember func(tool, rule string)
	used     []string
}

func (s *Service) chatWithTools(ctx context.Context, alias config.Alias, req llm.Request, sess chat.Session,
	in ChatSend, remember func(tool, rule string), onDelta func(string),
) (chat.Message, []string, error) {
	reply := chat.Message{ID: chat.NewMessageID(), Role: chat.RoleAssistant, CreatedAt: nowRFC3339()}
	model, err := s.aliasModel(alias)
	if err != nil {
		reply.Stopped, reply.Error = true, errModelSetup
		return reply, nil, err
	}
	tools, closeAll, err := s.mcpToolsFor(ctx, sess.MCPServers)
	if err != nil {
		reply.Stopped, reply.Error = true, s.redactor()(err.Error())
		return reply, nil, err
	}
	defer closeAll()
	ct := &chatTools{
		s: s, chatID: sess.ID, in: in, remember: remember, reg: map[string]agenttools.Tool{},
		policy: agent.Policy{Run: sess.ToolRules, App: s.cfg().Agent.Tools},
	}
	var defs []llm.ToolDef
	for _, t := range tools {
		ct.reg[t.Def().Name] = t
		defs = append(defs, t.Def())
	}
	res, text, err := ct.loop(ctx, model, req, defs, onDelta)
	reply.Model, reply.Content, reply.Tokens = res.Target, text, res.Usage.CompletionTokens
	return reply, ct.used, finishReply(&reply, err)
}

func (ct *chatTools) loop(ctx context.Context, model executor.Model, req llm.Request, defs []llm.ToolDef,
	onDelta func(string),
) (llm.Result, string, error) {
	for step := 0; ; step++ {
		r := req
		if step < maxChatToolSteps {
			r.Tools = defs
		}
		res, text, err := streamText(ctx, model, r, 0, onDelta)
		if err != nil || len(res.ToolCalls) == 0 || step >= maxChatToolSteps {
			return res, text, err
		}
		asked := llm.Message{Role: llm.RoleAssistant, Content: text, ToolCalls: res.ToolCalls}
		req.Messages = append(req.Messages, asked)
		for _, call := range res.ToolCalls {
			content := ct.call(ctx, call)
			req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: content})
		}
	}
}

// call decides and, if allowed, runs one tool call; it returns what the model reads.
func (ct *chatTools) call(ctx context.Context, call llm.ToolCall) string {
	tool, ok := ct.reg[call.Name]
	if !ok {
		return fmt.Sprintf("There is no tool named %q.", call.Name)
	}
	args := capText(call.Arguments, 2000)
	ct.emit(ChatToolEvent{Kind: "call", Tool: call.Name, Args: args})
	decision, by := ct.policy.Decide(call.Name), "rule"
	if decision == agent.Ask {
		decision, by = ct.ask(call), "you"
	}
	ct.s.auditChatTool(ct.chatID, call, string(decision))
	switch decision {
	case agent.Block:
		ct.emit(ChatToolEvent{Kind: "blocked", Tool: call.Name})
		return fmt.Sprintf("Tool %s is blocked by a rule. Do not call it again; answer without it.", call.Name)
	case agent.Ask:
		ct.emit(ChatToolEvent{Kind: "denied", Tool: call.Name})
		return fmt.Sprintf("The user has not approved %s. Answer without it, or say what you would use it for.",
			call.Name)
	case agent.Allow:
	}
	ct.used = append(ct.used, call.Name)
	runCtx, cancel := context.WithTimeout(ctx, chatToolTimeout)
	defer cancel()
	out, err := tool.Run(runCtx, json.RawMessage(schemaArgsOrEmpty(call.Arguments)))
	if err != nil {
		out = "The tool failed: " + ct.s.redactor()(err.Error())
	}
	out = capText(out, maxChatToolResult)
	ct.emit(ChatToolEvent{Kind: "result", Tool: call.Name, Summary: capText(firstLine(out), 200), By: by})
	return out
}

// ask returns Allow or Block from the user's answer, or Ask when nobody can answer.
func (ct *chatTools) ask(call llm.ToolCall) agent.Decision {
	if ct.in.Approve == nil {
		return agent.Ask
	}
	ans := ct.in.Approve(ChatToolRequest{ID: call.ID, Tool: call.Name, Args: capText(call.Arguments, 4000)})
	rule := config.RuleBlock
	if ans.Allow {
		rule = config.RuleAllow
	}
	if ans.Remember && ct.remember != nil {
		ct.remember(call.Name, rule)
	}
	if ans.Allow {
		return agent.Allow
	}
	return agent.Ask
}

func (ct *chatTools) emit(e ChatToolEvent) {
	if ct.in.OnTool != nil {
		ct.in.OnTool(e)
	}
}

// auditChatTool records a chat tool call by digest, like agent runs.
func (s *Service) auditChatTool(chatID string, call llm.ToolCall, decision string) {
	if s.UserAuditPath == "" {
		return
	}
	l, err := audit.Open(s.UserAuditPath, s.actor())
	if err != nil {
		return
	}
	_ = errors.Join(l.Append(audit.KindToolCall, chatID, map[string]any{
		auditKeyTool: call.Name, "args_sha256": audit.Digest(call.Arguments), "decision": decision, "chat": true,
	}), l.Close())
}

func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n[… cut]"
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func schemaArgsOrEmpty(args string) string {
	if strings.TrimSpace(args) == "" || !json.Valid([]byte(args)) {
		return "{}"
	}
	return args
}

func (s *Service) actor() string {
	if s.Actor == "" {
		return defaultActor
	}
	return s.Actor
}

// rememberChatRule keeps an answer as the chat's rule for that tool.
func (s *Service) rememberChatRule(st *chat.Store, id, tool, rule string) {
	_, _ = st.Update(id, func(sess *chat.Session) error {
		if sess.ToolRules == nil {
			sess.ToolRules = map[string]string{}
		}
		sess.ToolRules[tool] = rule
		return nil
	})
}

// toolNote records the tools a reply used (notes are never sent to the model).
func toolNote(used []string) chat.Message {
	seen := map[string]bool{}
	var names []string
	for _, u := range used {
		if !seen[u] {
			seen[u] = true
			names = append(names, u)
		}
	}
	return chat.Message{
		ID: chat.NewMessageID(), Role: chat.RoleNote, CreatedAt: nowRFC3339(),
		Content: "Used tools: " + strings.Join(names, ", "),
	}
}

// auditKeyTool names the tool in audit entries.
const auditKeyTool = "tool"
