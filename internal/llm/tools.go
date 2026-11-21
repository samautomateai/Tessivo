package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RoleTool marks a message that carries a tool's result.
const RoleTool = "tool"

// Bounds on what a model may ask for in one reply.
const (
	MaxToolCalls    = 16
	MaxToolCallArgs = 64 << 10
)

// ErrToolsUnsupported means the provider rejected a request because of its tools.
var ErrToolsUnsupported = errors.New("model does not support tool calling")

// ErrToolCallLimit means the model asked for more tool calls, or larger
// arguments, than MaxToolCalls and MaxToolCallArgs allow.
var ErrToolCallLimit = errors.New("tool call limit exceeded")

// ToolDef describes a tool the model may call. Parameters is a JSON schema.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToolCall is one call the model asked for; Arguments is the raw JSON text.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type wireToolCall struct {
	Index    *int             `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function wireCallFunction `json:"function"`
}

type wireCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

// wireMessage is a Message in the OpenAI chat format.
type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

func toWireMessages(msgs []Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, c := range m.ToolCalls {
			out[i].ToolCalls = append(out[i].ToolCalls, wireToolCall{
				ID: c.ID, Type: "function", Function: wireCallFunction{Name: c.Name, Arguments: c.Arguments},
			})
		}
	}
	return out
}

func toWireTools(defs []ToolDef) []wireTool {
	out := make([]wireTool, 0, len(defs))
	for _, d := range defs {
		out = append(out, wireTool{Type: "function", Function: wireFunction(d)})
	}
	return out
}

func fromWireCalls(calls []wireToolCall) ([]ToolCall, error) {
	if len(calls) > MaxToolCalls {
		return nil, fmt.Errorf("%w: model asked for %d tool calls; at most %d are allowed", ErrToolCallLimit,
			len(calls),
			MaxToolCalls)
	}
	out := make([]ToolCall, 0, len(calls))
	for _, c := range calls {
		if len(c.Function.Arguments) > MaxToolCallArgs {
			return nil, errTooLargeArgs(c.Function.Name)
		}
		out = append(out, ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: c.Function.Arguments})
	}
	return out, nil
}

func errTooLargeArgs(name string) error {
	return fmt.Errorf("%w: arguments for tool %q exceed 64 KiB", ErrToolCallLimit, name)
}

// toolAccumulator assembles streamed tool-call fragments keyed by index.
type toolAccumulator struct {
	order []int
	parts map[int]*ToolCall
}

func (a *toolAccumulator) add(d wireToolCall) error {
	idx := 0
	if d.Index != nil {
		idx = *d.Index
	}
	if a.parts == nil {
		a.parts = map[int]*ToolCall{}
	}
	c, ok := a.parts[idx]
	if !ok {
		if len(a.order) >= MaxToolCalls {
			return fmt.Errorf("%w: model asked for more than %d tool calls", ErrToolCallLimit, MaxToolCalls)
		}
		c = &ToolCall{}
		a.parts[idx] = c
		a.order = append(a.order, idx)
	}
	if d.ID != "" {
		c.ID = d.ID
	}
	if d.Function.Name != "" {
		c.Name = d.Function.Name
	}
	if len(c.Arguments)+len(d.Function.Arguments) > MaxToolCallArgs {
		return errTooLargeArgs(c.Name)
	}
	c.Arguments += d.Function.Arguments
	return nil
}

func (a *toolAccumulator) calls() []ToolCall {
	if len(a.order) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(a.order))
	for _, idx := range a.order {
		out = append(out, *a.parts[idx])
	}
	return out
}

// toolsError marks a provider rejection of a request that carried tools.
func toolsError(req Request, err error) error {
	var api *APIError
	if len(req.Tools) == 0 || !errors.As(err, &api) || api.Status != 400 {
		return err
	}
	body := strings.ToLower(api.Body)
	if strings.Contains(body, "tool") || strings.Contains(body, "function") {
		return fmt.Errorf("%w: %w", ErrToolsUnsupported, err)
	}
	return err
}
