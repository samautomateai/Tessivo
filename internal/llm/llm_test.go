package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

func newServer(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/", "sk-test-key", 5*time.Second)
}

func TestChatSendsWireFormat(t *testing.T) {
	var got map[string]any
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test-key" || r.Header.Get("X-Team") != "ops" {
			t.Errorf("headers: %v", r.Header)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = fmt.Fprint(w, `{"model":"m1","choices":[{"message":{"role":"assistant","content":"hi"},`+
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
	})
	c.Headers = map[string]string{"X-Team": "ops"}
	resp, err := c.Chat(context.Background(), Request{
		Model: "m1", Messages: []Message{{"user", "hello"}}, Temperature: config.Ptr(0.1),
		ReasoningEffort: "high", Format: &ResponseFormat{Name: "report", Schema: json.RawMessage(`{"type":"object"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi" || resp.Usage.TotalTokens != 4 || resp.FinishReason != "stop" {
		t.Fatalf("resp: %+v", resp)
	}
	if got["reasoning_effort"] != "high" || got["response_format"].(map[string]any)["type"] != "json_schema" {
		t.Fatalf("wire: %v", got)
	}
}

func TestReasoningOffIsOmitted(t *testing.T) {
	w := toWire(Request{Model: "m", ReasoningEffort: "off"}, true)
	if w.ReasoningEffort != "" || w.StreamOptions == nil || !w.StreamOptions.IncludeUsage {
		t.Fatalf("wire: %+v", w)
	}
}

func TestChatErrors(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"maximum context length exceeded for key sk-test-key"}`)
	})
	_, err := c.Chat(context.Background(), Request{Model: "m"})
	var api *APIError
	if !errors.As(err, &api) || api.Status != 400 {
		t.Fatalf("want APIError, got %v", err)
	}
	if strings.Contains(err.Error(), "sk-test-key") {
		t.Fatal("api key leaked into error")
	}
	if Classify(err) != config.TriggerContextOverflow {
		t.Fatalf("classify: %q", Classify(err))
	}
	empty := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `{"choices":[]}`) })
	if _, err := empty.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("want no-choices error")
	}
	garbage := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `not json`) })
	if _, err := garbage.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("want decode error")
	}
	if _, err := garbage.Models(context.Background()); err == nil {
		t.Fatal("want decode error")
	}
	dead := NewClient("http://127.0.0.1:1", "", 0)
	if _, err := dead.Chat(context.Background(), Request{}); Classify(err) != config.TriggerServerError {
		t.Fatalf("want transport error classified as server_error, got %v", err)
	}
	if _, err := dead.Models(context.Background()); err == nil {
		t.Fatal("want transport error")
	}
	bad := &Client{BaseURL: "http://bad host"}
	if _, err := bad.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("want build error")
	}
}

func TestModels(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"data":[{"id":"a"},{"id":"b","owned_by":"x"}]}`)
	})
	ms, err := c.Models(context.Background())
	if err != nil || len(ms) != 2 || ms[1].OwnedBy != "x" {
		t.Fatalf("models: %v %+v", err, ms)
	}
}

func TestChatStream(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, ": keepalive\n\n"+
			`data: {"model":"m1","choices":[{"delta":{"content":"Hel"}}]}`+"\n\n"+
			`data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}]}`+"\n\n"+
			`data: {"choices":[],"usage":{"total_tokens":9}}`+"\n\n"+
			"data: [DONE]\n\n")
	})
	var deltas []string
	resp, err := c.ChatStream(context.Background(), Request{Model: "m1"}, func(s string) { deltas = append(deltas, s) })
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello" || len(deltas) != 2 || resp.Usage.TotalTokens != 9 || resp.FinishReason != "stop" {
		t.Fatalf("resp %+v deltas %v", resp, deltas)
	}
	bad := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "data: {oops\n\n") })
	if _, err := bad.ChatStream(context.Background(), Request{}, nil); err == nil {
		t.Fatal("want chunk decode error")
	}
	fail := newServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	if _, err := fail.ChatStream(context.Background(), Request{}, nil); Classify(err) != config.TriggerRateLimit {
		t.Fatalf("want rate limit, got %v", err)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]error{
		"":                            nil,
		config.TriggerTimeout:         context.DeadlineExceeded,
		config.TriggerServerError:     &APIError{Status: 502},
		config.TriggerInvalidOutput:   fmt.Errorf("x: %w", ErrInvalidOutput),
		config.TriggerContextOverflow: &APIError{Status: 400, Body: "context window too small"},
	}
	for want, err := range cases {
		if got := Classify(err); got != want {
			t.Errorf("Classify(%v) = %q, want %q", err, got, want)
		}
	}
	if Classify(&APIError{Status: 504}) != config.TriggerTimeout || Classify(&APIError{Status: 401}) != "" {
		t.Fatal("status mapping")
	}
	if Classify(errors.New("other")) != "" {
		t.Fatal("unknown errors have no trigger")
	}
	long := &APIError{Status: 500, Body: strings.Repeat("x", 400)}
	if !strings.HasSuffix(long.Error(), "…") {
		t.Fatal("long bodies are truncated")
	}
}

func TestNewRouterFromConfig(t *testing.T) {
	f := config.Default()
	f.Providers = append(f.Providers, config.Provider{Name: "open", BaseURL: "http://o"})
	alias := config.Alias{Name: "smart", MaxRetries: 2,
		Chain: []config.ModelRef{{Provider: "litellm", Model: "a"}, {Provider: "litellm", Model: "b"},
			{Provider: "open", Model: "c"}}}
	r, err := NewRouter(f, alias, secrets.Map{"TESSIVO_LITELLM_KEY": "k123"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Chain) != 3 || r.Chain[0].Client != r.Chain[1].Client || r.Chain[2].Label != "open/c" || r.MaxRetries != 2 {
		t.Fatalf("router: %+v", r)
	}
	if _, err := NewRouter(f, alias, secrets.Map{}); err == nil {
		t.Fatal("want missing key error")
	}
	alias.Chain = append(alias.Chain, config.ModelRef{Provider: "ghost", Model: "x"})
	if _, err := NewRouter(f, alias, secrets.Map{"TESSIVO_LITELLM_KEY": "k123"}); err == nil {
		t.Fatal("want unknown provider error")
	}
}
