package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/llm"
)

// MCPCaller calls a tool on one MCP server. The service adapts its MCP
// client to it, so MCP's own types stay out of the agent.
type MCPCaller interface {
	CallTool(ctx context.Context, name string, args json.RawMessage) (text string, isError bool, err error)
}

// MCPToolSpec is a tool a server lists.
type MCPToolSpec struct {
	Server, Name, Description string
	Schema                    json.RawMessage
}

// Limits on what a server's tool list may put in front of the model.
const (
	maxToolName       = 64
	maxToolSchema     = 16 << 10
	maxToolDescribe   = 1024
	defaultToolSchema = `{"type":"object"}`
)

// SanitizeToolName is mcp__<server>__<tool>, lower-cased, every character
// outside [a-z0-9_] replaced with '_', cut to 64 characters.
func SanitizeToolName(server, tool string) string {
	raw := strings.ToLower("mcp__" + server + "__" + tool)
	var sb strings.Builder
	for _, r := range raw {
		if sb.Len() >= maxToolName {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// MCPTools turns a server's tools into agent tools named
// mcp__<server>__<tool>; names that collide after sanitizing get _2, _3, …
func MCPTools(server string, specs []MCPToolSpec, c MCPCaller) []Tool {
	seen := map[string]bool{}
	out := make([]Tool, 0, len(specs))
	for _, s := range specs {
		base := SanitizeToolName(server, s.Name)
		name := base
		for i := 2; seen[name]; i++ {
			suffix := fmt.Sprintf("_%d", i)
			name = base[:min(len(base), maxToolName-len(suffix))] + suffix
		}
		seen[name] = true
		out = append(out, &mcpTool{remote: s.Name, caller: c, def: llm.ToolDef{
			Name:        name,
			Description: describeMCPTool(server, s.Description),
			Parameters:  toolSchema(s.Schema),
		}})
	}
	return out
}

func describeMCPTool(server, desc string) string {
	d := "[MCP server " + server + "] " + strings.TrimSpace(desc)
	if len(d) > maxToolDescribe {
		d = d[:maxToolDescribe]
		for !utf8.ValidString(d) {
			d = d[:len(d)-1]
		}
	}
	return d
}

// toolSchema passes a server's schema through when it is a JSON object of at
// most 16 KiB; anything else becomes an empty object schema.
func toolSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || len(raw) > maxToolSchema {
		return json.RawMessage(defaultToolSchema)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return json.RawMessage(defaultToolSchema)
	}
	return raw
}

type mcpTool struct {
	def    llm.ToolDef
	remote string
	caller MCPCaller
}

func (t *mcpTool) Def() llm.ToolDef { return t.def }

// Run calls the remote tool. A result the server marks as an error goes back
// to the model as "error: <text>"; a failed call is an error.
func (t *mcpTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	text, isError, err := t.caller.CallTool(ctx, t.remote, args)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.def.Name, err)
	}
	if isError {
		return "error: " + text, nil
	}
	return text, nil
}
