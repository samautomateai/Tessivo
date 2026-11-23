package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultAnthropicURL is Anthropic's API.
const DefaultAnthropicURL = "https://api.anthropic.com"

const (
	anthropicVersion    = "2023-06-01"
	anthropicMaxTokens  = 8192 // max_tokens is required; used when the request sets none
	formatTool          = "respond"
	thinkingHeadroom    = 4096 // answer tokens left after the thinking budget
	defaultProviderWait = 120 * time.Second
)

// thinkingBudget maps a reasoning effort to a thinking token budget (0: off).
func thinkingBudget(effort string) int {
	switch effort {
	case "low":
		return 2048
	case "medium":
		return 8192
	case "high":
		return 24576
	default:
		return 0
	}
}

// Anthropic talks to the Anthropic Messages API: system prompts and tool
// results in Anthropic's own format, prompt caching on the system prompt and
// the earlier turns, thinking budgets, and structured output via a forced tool.
type Anthropic struct {
	BaseURL string
	APIKey  string
	Headers map[string]string
	HTTP    *http.Client
	Timeout time.Duration
}

// NewAnthropic returns a client for baseURL ("" is DefaultAnthropicURL).
func NewAnthropic(baseURL, apiKey string, timeout time.Duration) *Anthropic {
	if timeout <= 0 {
		timeout = defaultProviderWait
	}
	if baseURL == "" {
		baseURL = DefaultAnthropicURL
	}
	return &Anthropic{
		BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: newHTTPClient(timeout),
		Timeout: timeout,
	}
}

type antCache struct {
	Type string `json:"type"`
}

type antBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      string          `json:"content,omitempty"`
	CacheControl *antCache       `json:"cache_control,omitempty"`
}

type antMessage struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

type antTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type antChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type antThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type antRequest struct {
	Model       string       `json:"model"`
	MaxTokens   int          `json:"max_tokens"`
	System      []antBlock   `json:"system,omitempty"`
	Messages    []antMessage `json:"messages"`
	Temperature *float64     `json:"temperature,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
	Tools       []antTool    `json:"tools,omitempty"`
	ToolChoice  *antChoice   `json:"tool_choice,omitempty"`
	Thinking    *antThinking `json:"thinking,omitempty"`
}

type antUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}

func (u antUsage) prompt() int { return u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens }

type antResponse struct {
	Model      string     `json:"model"`
	StopReason string     `json:"stop_reason"`
	Content    []antBlock `json:"content"`
	Usage      antUsage   `json:"usage"`
}

func (a *Anthropic) toWire(req Request, stream bool) antRequest {
	w := antRequest{Model: req.Model, MaxTokens: anthropicMaxTokens, Stream: stream, Temperature: req.Temperature}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		w.MaxTokens = *req.MaxTokens
	}
	w.System, w.Messages = antMessages(req.Messages)
	for _, t := range req.Tools {
		tool := antTool{Name: t.Name, Description: t.Description, InputSchema: schemaOrObject(t.Parameters)}
		w.Tools = append(w.Tools, tool)
	}
	switch req.ToolChoice {
	case choiceAuto:
		w.ToolChoice = &antChoice{Type: choiceAuto}
	case choiceNone:
		w.ToolChoice = &antChoice{Type: choiceNone}
	case "required":
		w.ToolChoice = &antChoice{Type: "any"}
	}
	if req.Format != nil { // structured output: the model must call one tool whose input is the result
		w.Tools = append(w.Tools, antTool{
			Name: formatTool, Description: "Respond with the result.",
			InputSchema: schemaOrObject(req.Format.Schema),
		})
		w.ToolChoice = &antChoice{Type: "tool", Name: formatTool}
		return w // a forced tool cannot be combined with thinking
	}
	if b := thinkingBudget(req.ReasoningEffort); b > 0 {
		w.Thinking = &antThinking{Type: "enabled", BudgetTokens: b}
		w.MaxTokens = max(w.MaxTokens, b+thinkingHeadroom)
		w.Temperature = nil // thinking requires the default temperature
	}
	return w
}

func schemaOrObject(s json.RawMessage) json.RawMessage {
	if len(s) == 0 || !json.Valid(s) {
		return json.RawMessage(`{"type":"object"}`)
	}
	return s
}

// antMessages splits out the system prompt, maps tool calls and results to
// blocks, merges same-role neighbours (the API needs alternating roles) and
// marks cache breakpoints on the system prompt and the turn before the last.
func antMessages(msgs []Message) ([]antBlock, []antMessage) {
	var system []antBlock
	var out []antMessage
	add := func(role string, blocks ...antBlock) {
		if len(blocks) == 0 {
			return
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Content = append(out[n-1].Content, blocks...)
			return
		}
		out = append(out, antMessage{Role: role, Content: blocks})
	}
	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			if m.Content != "" {
				system = append(system, antBlock{Type: blockText, Text: m.Content})
			}
		case RoleTool:
			add("user", antBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: nonEmpty(m.Content)})
		case RoleAssistant:
			add("assistant", assistantBlocks(m)...)
		default:
			if m.Content != "" {
				add("user", antBlock{Type: blockText, Text: m.Content})
			}
		}
	}
	if n := len(system); n > 0 {
		system[n-1].CacheControl = &antCache{Type: "ephemeral"}
	}
	if n := len(out); n >= 2 {
		blocks := out[n-2].Content
		blocks[len(blocks)-1].CacheControl = &antCache{Type: "ephemeral"}
	}
	return system, out
}

// assistantBlocks is an assistant turn as text and tool_use blocks.
func assistantBlocks(m Message) []antBlock {
	var blocks []antBlock
	if m.Content != "" {
		blocks = append(blocks, antBlock{Type: blockText, Text: m.Content})
	}
	for _, c := range m.ToolCalls {
		blocks = append(blocks, antBlock{Type: blockToolUse, ID: c.ID, Name: c.Name, Input: schemaArgs(c.Arguments)})
	}
	return blocks
}

func nonEmpty(s string) string {
	if s == "" {
		return "(no output)"
	}
	return s
}

func schemaArgs(args string) json.RawMessage {
	if args == "" || !json.Valid([]byte(args)) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(args)
}

func (a *Anthropic) headers() map[string]string {
	h := map[string]string{"x-api-key": a.APIKey, "anthropic-version": anthropicVersion}
	for k, v := range a.Headers {
		h[k] = v
	}
	return h
}

// Chat sends a non-streaming request.
func (a *Anthropic) Chat(ctx context.Context, req Request) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()
	resp, err := sendJSON(ctx, a.HTTP, http.MethodPost, a.BaseURL+"/v1/messages", a.headers(), a.toWire(req, false),
		a.APIKey)
	if err != nil {
		return Response{}, toolsError(req, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var w antResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&w); err != nil {
		return Response{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	return antResult(req, w)
}

// antResult turns a finished reply into a Response.
func antResult(req Request, w antResponse) (Response, error) {
	out := Response{Model: w.Model, FinishReason: antStop(w.StopReason, req.Format != nil), Usage: Usage{
		PromptTokens: w.Usage.prompt(), CompletionTokens: w.Usage.OutputTokens,
		TotalTokens: w.Usage.prompt() + w.Usage.OutputTokens,
	}}
	var text strings.Builder
	for _, b := range w.Content {
		switch {
		case b.Type == blockText:
			text.WriteString(b.Text)
		case b.Type == blockToolUse && req.Format != nil && b.Name == formatTool:
			out.Content = string(b.Input)
		case b.Type == blockToolUse:
			if len(out.ToolCalls) >= MaxToolCalls {
				return Response{}, errTooManyCalls()
			}
			if len(b.Input) > MaxToolCallArgs {
				return Response{}, errTooLargeArgs(b.Name)
			}
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: b.ID, Name: b.Name, Arguments: string(b.Input)})
		}
	}
	if req.Format == nil {
		out.Content = text.String()
	}
	return out, nil
}

func antStop(reason string, format bool) string {
	switch reason {
	case "max_tokens":
		return "length"
	case blockToolUse:
		if format {
			return finishStop
		}
		return "tool_calls"
	default:
		return finishStop
	}
}

// Models lists the models the key can use.
func (a *Anthropic) Models(ctx context.Context) ([]Model, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()
	resp, err := sendJSON(ctx, a.HTTP, http.MethodGet, a.BaseURL+"/v1/models?limit=1000", a.headers(), nil, a.APIKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var w struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&w); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	return w.Data, nil
}

func errTooManyCalls() error {
	return fmt.Errorf("%w: model asked for more than %d tool calls", ErrToolCallLimit, MaxToolCalls)
}
