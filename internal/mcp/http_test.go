package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// streamable is a minimal Streamable HTTP MCP server: initialize answers JSON
// and sets a session; tools/list answers as an SSE stream; notifications get
// 202. It records the session header of every request after initialize.
type streamable struct {
	mu       sync.Mutex
	sessions []string
	auth     []string
}

func (s *streamable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var m message
	_ = json.Unmarshal(body, &m)
	s.mu.Lock()
	s.auth = append(s.auth, r.Header.Get("Authorization"))
	if m.Method != "initialize" {
		s.sessions = append(s.sessions, r.Header.Get("Mcp-Session-Id"))
	}
	s.mu.Unlock()
	switch {
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusOK)
	case m.ID == nil:
		w.WriteHeader(http.StatusAccepted)
	case m.Method == "initialize":
		w.Header().Set("Mcp-Session-Id", "sess-1")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-06-18"}}`, m.ID)
	default:
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n")
		_, _ = fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%s,\n"+
			"data: \"result\":{\"tools\":[{\"name\":\"search\"}]}}\n\n", m.ID)
	}
}

func TestHTTPJSONAndSSEReplies(t *testing.T) {
	h := &streamable{}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := connect(t, Server{Name: "remote", URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer t"}})
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools: %v %+v", err, tools)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, s := range h.sessions {
		if s != "sess-1" {
			t.Fatalf("request %d sent session %q", i, s)
		}
	}
	if len(h.sessions) < 2 || h.auth[0] != "Bearer t" {
		t.Fatalf("sessions %v auth %v", h.sessions, h.auth)
	}
}

func TestHTTPRefusesRedirectAndPlainRemote(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/mcp", http.StatusFound)
	}))
	t.Cleanup(redirect.Close)
	if _, err := Connect(context.Background(), Server{Name: "r", URL: redirect.URL}); err == nil ||
		!strings.Contains(err.Error(), "302") {
		t.Fatalf("redirect: %v", err)
	}
	_, err := Connect(context.Background(), Server{Name: "p", URL: "http://example.com/mcp"})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("plain remote: %v", err)
	}
}

func TestHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	if _, err := Connect(context.Background(), Server{Name: "u", URL: srv.URL}); err == nil ||
		!strings.Contains(err.Error(), "401") {
		t.Fatalf("401: %v", err)
	}
}
