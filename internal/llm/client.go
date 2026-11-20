// Package llm is Tessivo's client for OpenAI-compatible APIs such as a LiteLLM
// proxy: model listing, chat completions, streaming, structured output and
// reasoning effort, plus alias fallback chains (see Router).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat requests JSON output that matches Schema.
type ResponseFormat struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

// Request is a provider-neutral chat request.
type Request struct {
	Model           string
	Messages        []Message
	Temperature     *float64
	MaxTokens       *int
	ReasoningEffort string // "", "low", "medium", "high"; "off" sends nothing
	Format          *ResponseFormat
}

// Usage is token accounting reported by the provider.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response is a completed chat answer.
type Response struct {
	Model        string
	Content      string
	FinishReason string
	Usage        Usage
}

// Model is one entry from the provider's /models list.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// Chatter is anything that can answer chat requests, such as Client.
type Chatter interface {
	Chat(ctx context.Context, req Request) (Response, error)
	ChatStream(ctx context.Context, req Request, onDelta func(string)) (Response, error)
}

// Client talks to one OpenAI-compatible base URL.
type Client struct {
	BaseURL string
	APIKey  string
	Headers map[string]string
	HTTP    *http.Client
	// Timeout bounds a non-streaming call and the gap between stream chunks.
	Timeout time.Duration
}

// NewClient returns a client with a sensible timeout.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    newHTTPClient(timeout),
		Timeout: timeout,
	}
}

type wireRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Temperature     *float64        `json:"temperature,omitempty"`
	MaxTokens       *int            `json:"max_tokens,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	ResponseFormat  *wireFormat     `json:"response_format,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	StreamOptions   *wireStreamOpts `json:"stream_options,omitempty"`
}

type wireFormat struct {
	Type       string          `json:"type"`
	JSONSchema *wireJSONSchema `json:"json_schema,omitempty"`
}

type wireJSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type wireStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

func toWire(req Request, stream bool) wireRequest {
	w := wireRequest{
		Model: req.Model, Messages: req.Messages, Temperature: req.Temperature, MaxTokens: req.MaxTokens,
		Stream: stream,
	}
	if req.ReasoningEffort != "" && req.ReasoningEffort != "off" {
		w.ReasoningEffort = req.ReasoningEffort
	}
	if req.Format != nil {
		w.ResponseFormat = &wireFormat{Type: "json_schema", JSONSchema: &wireJSONSchema{
			Name: req.Format.Name, Schema: req.Format.Schema, Strict: true,
		}}
	}
	if stream {
		w.StreamOptions = &wireStreamOpts{IncludeUsage: true}
	}
	return w
}

// Chat sends a non-streaming chat completion request.
func (c *Client) Chat(ctx context.Context, req Request) (Response, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	resp, err := c.do(ctx, http.MethodPost, "/chat/completions", toWire(req, false))
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var w wireResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&w); err != nil {
		return Response{}, fmt.Errorf("decode chat response: %w", err)
	}
	if len(w.Choices) == 0 {
		return Response{}, fmt.Errorf("chat response has no choices")
	}
	return Response{
		Model: w.Model, Content: w.Choices[0].Message.Content,
		FinishReason: w.Choices[0].FinishReason, Usage: w.Usage,
	}, nil
}

// Models lists the models the provider exposes.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	resp, err := c.do(ctx, http.MethodGet, "/models", nil)
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

const maxBody = 32 << 20 // 32 MiB

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, &TransportError{Err: err}
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, &APIError{Status: resp.StatusCode, Body: redactKey(string(b), c.APIKey)}
	}
	return resp, nil
}

func redactKey(s, key string) string {
	if len(key) >= 4 {
		return strings.ReplaceAll(s, key, "[REDACTED]")
	}
	return s
}
