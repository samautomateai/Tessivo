package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// Server is a resolved server definition: placeholders already expanded.
// Command starts a stdio server; URL dials a Streamable HTTP server.
type Server struct {
	Name              string
	Command           string
	Args              []string
	Env               map[string]string
	URL               string
	Headers           map[string]string
	AllowInsecureHTTP bool
}

// Tool is a tool a server offers.
type (
	Tool struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	}
	// Resource is a resource a server offers.
	Resource struct {
		URI         string `json:"uri"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		MimeType    string `json:"mimeType,omitempty"`
	}
	// Prompt is a prompt template a server offers.
	Prompt struct {
		Name        string           `json:"name"`
		Description string           `json:"description,omitempty"`
		Arguments   []PromptArgument `json:"arguments,omitempty"`
	}
	// PromptArgument is one argument of a Prompt.
	PromptArgument struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Required    bool   `json:"required,omitempty"`
	}
)

// CallResult is a tool's answer: its text blocks joined, other blocks named.
type CallResult struct {
	Text    string
	IsError bool
}

// List caps and the largest resource text accepted (bytes).
const (
	MaxTools        = 500
	MaxResources    = 500
	MaxPrompts      = 200
	MaxResourceText = 2 << 20
	maxPages        = 50
)

// Timeouts; variables so tests can shorten them.
var (
	handshakeTimeout = 30 * time.Second
	requestTimeout   = 30 * time.Second
	callTimeout      = 120 * time.Second
	closeGrace       = 2 * time.Second
)

// Errors callers can match.
var (
	ErrServerExited     = errors.New("mcp: server exited")
	ErrResourceTooLarge = errors.New("mcp: resource exceeds 2 MiB")
	errClosed           = errors.New("mcp: client closed")
)

// conn is one transport: a request gets its response; a notification none.
type conn interface {
	roundTrip(ctx context.Context, m message) (message, error)
	notify(ctx context.Context, m message) error
	close() error
}

// Client talks to one MCP server. It is safe for concurrent use.
type Client struct {
	name   string
	conn   conn
	nextID atomic.Int64
}

// Connect starts or dials the server and completes the MCP handshake.
func Connect(ctx context.Context, s Server) (*Client, error) {
	var (
		c   conn
		err error
	)
	switch {
	case s.Command != "" && s.URL == "":
		c, err = startStdio(ctx, s)
	case s.URL != "" && s.Command == "":
		c, err = dialHTTP(s)
	default:
		err = errors.New("give exactly one of a command or a URL")
	}
	if err != nil {
		return nil, fmt.Errorf("mcp %s: %w", s.Name, err)
	}
	cl := &Client{name: s.Name, conn: c}
	if err := cl.handshake(ctx); err != nil {
		closeErr := c.close()
		if st, ok := c.(*stdioConn); ok {
			err = st.withStderr(err)
		}
		return nil, errors.Join(fmt.Errorf("mcp %s: %w", s.Name, err), closeErr)
	}
	return cl, nil
}

func (c *Client) handshake(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	params := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "tessivo", "version": "1"},
	}
	if err := c.call(ctx, "initialize", params, nil); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if err := c.conn.notify(ctx, message{JSONRPC: jsonrpcVersion, Method: "notifications/initialized"}); err != nil {
		return fmt.Errorf("initialized: %w", err)
	}
	return nil
}

// call sends method and decodes the result into out (nil to discard).
func (c *Client) call(ctx context.Context, method string, params, out any) error {
	m := request(c.nextID.Add(1), method, params)
	resp, err := c.conn.roundTrip(ctx, m)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("mcp: %s result: %w", method, err)
	}
	return nil
}

func (c *Client) timed(ctx context.Context, d time.Duration, method string, params, out any) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	if err := c.call(ctx, method, params, out); err != nil {
		return fmt.Errorf("mcp %s: %s: %w", c.name, method, err)
	}
	return nil
}

// list follows nextCursor pages, keeping at most limit items.
func list[T any](ctx context.Context, c *Client, method, field string, limit int) ([]T, error) {
	var out []T
	cursor := ""
	for range maxPages {
		params := map[string]string{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var page map[string]json.RawMessage
		if err := c.timed(ctx, requestTimeout, method, params, &page); err != nil {
			return nil, err
		}
		var items []T
		if raw, ok := page[field]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, fmt.Errorf("mcp %s: %s: %w", c.name, method, err)
			}
		}
		out = append(out, items...)
		if len(out) >= limit {
			return out[:limit], nil
		}
		cursor = ""
		if raw, ok := page["nextCursor"]; ok {
			_ = json.Unmarshal(raw, &cursor)
		}
		if cursor == "" {
			break
		}
	}
	return out, nil
}

// ListTools lists up to MaxTools tools.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	return list[Tool](ctx, c, "tools/list", "tools", MaxTools)
}

// ListResources lists up to MaxResources resources.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	return list[Resource](ctx, c, "resources/list", "resources", MaxResources)
}

// ListPrompts lists up to MaxPrompts prompts.
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	return list[Prompt](ctx, c, "prompts/list", "prompts", MaxPrompts)
}

// CallTool calls a tool with JSON object arguments (nil for none).
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	var res struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			MimeType string `json:"mimeType"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	params := map[string]any{"name": name, "arguments": args}
	if err := c.timed(ctx, callTimeout, "tools/call", params, &res); err != nil {
		return CallResult{}, err
	}
	parts := make([]string, 0, len(res.Content))
	for _, b := range res.Content {
		if b.Type == "text" {
			parts = append(parts, b.Text)
			continue
		}
		label := b.Type
		if b.MimeType != "" {
			label += ": " + b.MimeType
		}
		parts = append(parts, "["+label+"]")
	}
	return CallResult{Text: strings.Join(parts, "\n"), IsError: res.IsError}, nil
}

// ReadResource returns a resource's text (≤ MaxResourceText). Binary content
// is refused unless its MIME type is text or JSON.
func (c *Client) ReadResource(ctx context.Context, uri string) (text, mimeType string, err error) {
	var res struct {
		Contents []struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Blob     string `json:"blob"`
		} `json:"contents"`
	}
	if err := c.timed(ctx, requestTimeout, "resources/read", map[string]string{"uri": uri}, &res); err != nil {
		return "", "", err
	}
	var sb strings.Builder
	for _, part := range res.Contents {
		mimeType = part.MimeType
		chunk := part.Text
		if part.Blob != "" {
			if !textual(part.MimeType) {
				return "", "", fmt.Errorf("mcp %s: %s: binary resource not supported (%s)", c.name, uri, part.MimeType)
			}
			b, err := base64.StdEncoding.DecodeString(part.Blob)
			if err != nil {
				return "", "", fmt.Errorf("mcp %s: %s: %w", c.name, uri, err)
			}
			chunk = string(b)
		}
		if sb.Len()+len(chunk) > MaxResourceText {
			return "", "", fmt.Errorf("mcp %s: %s: %w", c.name, uri, ErrResourceTooLarge)
		}
		sb.WriteString(chunk)
	}
	return sb.String(), mimeType, nil
}

func textual(mime string) bool {
	return strings.HasPrefix(mime, "text/") || mime == "application/json" || strings.HasSuffix(mime, "+json")
}

// Close ends the session; a stdio server is stopped.
func (c *Client) Close() error { return c.conn.close() }
