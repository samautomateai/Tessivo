package mcp

// The Streamable HTTP transport follows Zero internal/mcp/network_client.go
// (commit 8288ae81bd6a, MIT; see doc.go): each message is a POST; a request's
// answer is JSON or an SSE stream holding it; the session ID is echoed.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/llm"
)

const sessionHeader = "Mcp-Session-Id"

type httpConn struct {
	url     string
	headers map[string]string
	client  *http.Client

	mu      sync.Mutex
	session string
}

func dialHTTP(s Server) (*httpConn, error) {
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", s.URL)
	}
	if u.User != nil {
		return nil, errors.New("URLs with credentials are refused; use a header")
	}
	if u.Scheme == "http" && !s.AllowInsecureHTTP && !loopback(u.Hostname()) {
		return nil, fmt.Errorf("plain http to %s is refused: use https or set allow_insecure_http", u.Hostname())
	}
	return &httpConn{url: s.URL, headers: s.Headers, client: llm.NewHTTPClient(requestTimeout)}, nil
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *httpConn) post(ctx context.Context, m message) (*http.Response, error) {
	body, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp: request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: post: %w", err)
	}
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("mcp: server answered %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if s := resp.Header.Get(sessionHeader); s != "" {
		c.mu.Lock()
		c.session = s
		c.mu.Unlock()
	}
	return resp, nil
}

func (c *httpConn) setHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	c.mu.Lock()
	if c.session != "" {
		req.Header.Set(sessionHeader, c.session)
	}
	c.mu.Unlock()
}

func (c *httpConn) roundTrip(ctx context.Context, m message) (message, error) {
	resp, err := c.post(ctx, m)
	if err != nil {
		return message{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt == "text/event-stream" {
		return readSSE(resp.Body, m.ID)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxMessage+1))
	if err != nil {
		return message{}, fmt.Errorf("mcp: read answer: %w", err)
	}
	if len(b) > MaxMessage {
		return message{}, ErrTooLarge
	}
	return decode(b)
}

// readSSE returns the response with id from an SSE stream, skipping other
// messages (notifications, progress). Each event is bounded by MaxMessage.
func readSSE(body io.Reader, id json.RawMessage) (message, error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), MaxMessage)
	var data strings.Builder
	flush := func() (message, bool, error) {
		defer data.Reset()
		if data.Len() == 0 {
			return message{}, false, nil
		}
		m, err := decode([]byte(data.String()))
		if err != nil {
			return message{}, false, err
		}
		return m, m.Method == "" && bytes.Equal(m.ID, id), nil
	}
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len()+len(v) > MaxMessage {
				return message{}, ErrTooLarge
			}
			data.WriteString(strings.TrimPrefix(v, " "))
			continue
		}
		if line == "" {
			if m, done, err := flush(); err != nil || done {
				return m, err
			}
		}
	}
	if m, done, err := flush(); err != nil || done {
		return m, err
	}
	if err := sc.Err(); errors.Is(err, bufio.ErrTooLong) {
		return message{}, ErrTooLarge
	}
	return message{}, errors.New("mcp: stream ended without an answer")
}

func (c *httpConn) notify(ctx context.Context, m message) error {
	resp, err := c.post(ctx, m)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("mcp: close answer: %w", err)
	}
	return nil
}

// close ends the session (best effort).
func (c *httpConn) close() error {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	if session == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.url, nil)
	if err != nil {
		return nil //nolint:nilerr // best effort: the session also expires on the server
	}
	c.setHeaders(req)
	if resp, err := c.client.Do(req); err == nil {
		_ = resp.Body.Close()
	}
	return nil
}
