package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultGeminiURL is Google's Gemini API.
const DefaultGeminiURL = "https://generativelanguage.googleapis.com"

// Gemini talks to the Gemini generateContent API: system instructions,
// function calls and responses in Gemini's own format, thinking budgets and
// JSON-schema output. The key travels in the x-goog-api-key header.
type Gemini struct {
	BaseURL string
	APIKey  string
	Headers map[string]string
	HTTP    *http.Client
	Timeout time.Duration
}

// NewGemini returns a client for baseURL ("" is DefaultGeminiURL).
func NewGemini(baseURL, apiKey string, timeout time.Duration) *Gemini {
	if timeout <= 0 {
		timeout = defaultProviderWait
	}
	if baseURL == "" {
		baseURL = DefaultGeminiURL
	}
	return &Gemini{
		BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: newHTTPClient(timeout),
		Timeout: timeout,
	}
}

type gemCall struct {
	Args json.RawMessage `json:"args"`
	Name string          `json:"name"`
	ID   string          `json:"id,omitempty"`
}

type gemFuncResponse struct {
	Name     string         `json:"name"`
	ID       string         `json:"id,omitempty"`
	Response map[string]any `json:"response"`
}

type gemPart struct {
	Text             string           `json:"text,omitempty"`
	Thought          bool             `json:"thought,omitempty"`
	FunctionCall     *gemCall         `json:"functionCall,omitempty"`
	FunctionResponse *gemFuncResponse `json:"functionResponse,omitempty"`
}

type gemContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []gemPart `json:"parts"`
}

type gemFunc struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description,omitempty"`
	ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

type gemTools struct {
	FunctionDeclarations []gemFunc `json:"functionDeclarations"`
}

type gemToolConfig struct {
	FunctionCallingConfig struct {
		Mode string `json:"mode"`
	} `json:"functionCallingConfig"`
}

type gemThinking struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type gemGeneration struct {
	Temperature        *float64        `json:"temperature,omitempty"`
	MaxOutputTokens    *int            `json:"maxOutputTokens,omitempty"`
	ResponseMimeType   string          `json:"responseMimeType,omitempty"`
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
	ThinkingConfig     *gemThinking    `json:"thinkingConfig,omitempty"`
}

type gemRequest struct {
	Contents          []gemContent   `json:"contents"`
	SystemInstruction *gemContent    `json:"systemInstruction,omitempty"`
	Tools             []gemTools     `json:"tools,omitempty"`
	ToolConfig        *gemToolConfig `json:"toolConfig,omitempty"`
	GenerationConfig  gemGeneration  `json:"generationConfig"`
}

type gemResponse struct {
	ModelVersion string `json:"modelVersion"`
	Candidates   []struct {
		Content      gemContent `json:"content"`
		FinishReason string     `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (g *Gemini) toWire(req Request) gemRequest {
	w := gemRequest{GenerationConfig: gemGeneration{Temperature: req.Temperature, MaxOutputTokens: req.MaxTokens}}
	w.SystemInstruction, w.Contents = gemContents(req.Messages)
	if len(req.Tools) > 0 {
		decl := gemTools{}
		for _, t := range req.Tools {
			decl.FunctionDeclarations = append(decl.FunctionDeclarations, gemFunc{
				Name: t.Name, Description: t.Description, ParametersJSONSchema: schemaOrObject(t.Parameters),
			})
		}
		w.Tools = []gemTools{decl}
	}
	modes := map[string]string{choiceAuto: "AUTO", choiceNone: "NONE", "required": "ANY"}
	if mode := modes[req.ToolChoice]; mode != "" {
		w.ToolConfig = &gemToolConfig{}
		w.ToolConfig.FunctionCallingConfig.Mode = mode
	}
	if req.Format != nil {
		w.GenerationConfig.ResponseMimeType = "application/json"
		w.GenerationConfig.ResponseJSONSchema = schemaOrObject(req.Format.Schema)
	}
	if b := thinkingBudget(req.ReasoningEffort); b > 0 {
		w.GenerationConfig.ThinkingConfig = &gemThinking{ThinkingBudget: b}
	}
	return w
}

// gemContents splits out the system instruction, maps tool calls and results
// (a result names its function, found from the call it answers) and merges
// same-role neighbours.
func gemContents(msgs []Message) (*gemContent, []gemContent) {
	var system *gemContent
	var out []gemContent
	names := map[string]string{}
	add := func(role string, parts ...gemPart) {
		if len(parts) == 0 {
			return
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Parts = append(out[n-1].Parts, parts...)
			return
		}
		out = append(out, gemContent{Role: role, Parts: parts})
	}
	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			if system == nil {
				system = &gemContent{}
			}
			system.Parts = append(system.Parts, gemPart{Text: m.Content})
		case RoleAssistant:
			var parts []gemPart
			if m.Content != "" {
				parts = append(parts, gemPart{Text: m.Content})
			}
			for _, c := range m.ToolCalls {
				names[c.ID] = c.Name
				parts = append(parts, gemPart{FunctionCall: &gemCall{Args: schemaArgs(c.Arguments), Name: c.Name}})
			}
			add("model", parts...)
		case RoleTool:
			add("user", gemPart{FunctionResponse: &gemFuncResponse{
				Name: names[m.ToolCallID], Response: map[string]any{"content": m.Content},
			}})
		default:
			if m.Content != "" {
				add("user", gemPart{Text: m.Content})
			}
		}
	}
	return system, out
}

func (g *Gemini) headers() map[string]string {
	h := map[string]string{"x-goog-api-key": g.APIKey}
	for k, v := range g.Headers {
		h[k] = v
	}
	return h
}

func (g *Gemini) url(model, method string) string {
	return g.BaseURL + "/v1beta/models/" + url.PathEscape(model) + ":" + method
}

// Chat sends a non-streaming request.
func (g *Gemini) Chat(ctx context.Context, req Request) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()
	resp, err := sendJSON(ctx, g.HTTP, http.MethodPost, g.url(req.Model, "generateContent"), g.headers(),
		g.toWire(req), g.APIKey)
	if err != nil {
		return Response{}, toolsError(req, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var w gemResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&w); err != nil {
		return Response{}, fmt.Errorf("decode gemini response: %w", err)
	}
	acc := &gemAccumulator{}
	if err := acc.add(w, nil); err != nil {
		return Response{}, err
	}
	return acc.response(), nil
}

// ChatStream sends a streaming request; onDelta gets answer text (never thoughts).
func (g *Gemini) ChatStream(ctx context.Context, req Request, onDelta func(string)) (Response, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := sendJSON(ctx, g.HTTP, http.MethodPost, g.url(req.Model, "streamGenerateContent")+"?alt=sse",
		g.headers(), g.toWire(req), g.APIKey)
	if err != nil {
		return Response{}, toolsError(req, err)
	}
	resp.Body = newIdleReader(resp.Body, g.Timeout, cancel)
	defer func() { _ = resp.Body.Close() }()
	acc := &gemAccumulator{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "data:")
		if !ok {
			continue
		}
		var w gemResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &w); err != nil {
			return Response{}, fmt.Errorf("decode stream chunk: %w", err)
		}
		if err := acc.add(w, onDelta); err != nil {
			return Response{}, err
		}
	}
	if err := sc.Err(); err != nil {
		if errors.Is(context.Cause(ctx), ErrIdleTimeout) {
			return Response{}, &TransportError{Err: fmt.Errorf("%w after %s: %w", ErrIdleTimeout, g.Timeout,
				context.DeadlineExceeded)}
		}
		return Response{}, &TransportError{Err: err}
	}
	return acc.response(), nil
}

// gemAccumulator assembles one reply from one or more response chunks.
type gemAccumulator struct {
	out    Response
	text   strings.Builder
	finish string
}

func (a *gemAccumulator) add(w gemResponse, onDelta func(string)) error {
	if w.ModelVersion != "" {
		a.out.Model = w.ModelVersion
	}
	if u := w.UsageMetadata; u.TotalTokenCount > 0 {
		a.out.Usage = Usage{
			PromptTokens: u.PromptTokenCount, CompletionTokens: u.TotalTokenCount - u.PromptTokenCount,
			TotalTokens: u.TotalTokenCount,
		}
	}
	if len(w.Candidates) == 0 {
		return nil
	}
	c := w.Candidates[0]
	if c.FinishReason != "" {
		a.finish = c.FinishReason
	}
	for _, p := range c.Content.Parts {
		switch {
		case p.FunctionCall != nil:
			if len(a.out.ToolCalls) >= MaxToolCalls {
				return fmt.Errorf("%w: model asked for more than %d tool calls", ErrToolCallLimit, MaxToolCalls)
			}
			if len(p.FunctionCall.Args) > MaxToolCallArgs {
				return errTooLargeArgs(p.FunctionCall.Name)
			}
			id := p.FunctionCall.ID
			if id == "" {
				id = fmt.Sprintf("call-%d", len(a.out.ToolCalls)+1)
			}
			a.out.ToolCalls = append(a.out.ToolCalls, ToolCall{
				ID: id, Name: p.FunctionCall.Name,
				Arguments: string(schemaArgs(string(p.FunctionCall.Args))),
			})
		case p.Text != "" && !p.Thought:
			a.text.WriteString(p.Text)
			if onDelta != nil {
				onDelta(p.Text)
			}
		}
	}
	return nil
}

func (a *gemAccumulator) response() Response {
	a.out.Content = a.text.String()
	switch {
	case len(a.out.ToolCalls) > 0:
		a.out.FinishReason = "tool_calls"
	case a.finish == "MAX_TOKENS":
		a.out.FinishReason = "length"
	case a.finish == "STOP" || a.finish == "":
		a.out.FinishReason = finishStop
	default:
		a.out.FinishReason = "content_filter"
	}
	return a.out
}

// Models lists the models the key can use.
func (g *Gemini) Models(ctx context.Context) ([]Model, error) {
	ctx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()
	resp, err := sendJSON(ctx, g.HTTP, http.MethodGet, g.BaseURL+"/v1beta/models?pageSize=1000", g.headers(), nil,
		g.APIKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var w struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&w); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	out := make([]Model, 0, len(w.Models))
	for _, m := range w.Models {
		out = append(out, Model{ID: strings.TrimPrefix(m.Name, "models/"), OwnedBy: "google"})
	}
	return out, nil
}
