package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
)

func fakeGemini(t *testing.T, reply func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *string) {
	t.Helper()
	var last string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "g-key" || r.URL.Query().Get("key") != "" {
			t.Errorf("the key must travel in a header: %v %s", r.Header, r.URL)
		}
		b, _ := io.ReadAll(r.Body)
		last = r.URL.Path + " " + string(b)
		reply(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func TestGeminiChatMapsRolesToolsAndThinking(t *testing.T) {
	srv, last := fakeGemini(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"modelVersion":"gemini-x",
			"candidates":[{"finishReason":"STOP","content":{"role":"model",
			"parts":[{"text":"Reading.","thought":false},{"text":"internal","thought":true},
			{"functionCall":{"name":"read_file","args":{"path":"a.go"}}}]}}],
			"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`))
	})
	c := NewGemini(srv.URL, "g-key", time.Minute)
	resp, err := c.Chat(context.Background(), Request{
		Model: "gemini-x", ReasoningEffort: "low",
		Messages: []Message{
			{Role: RoleSystem, Content: "You analyze code."},
			{Role: RoleUser, Content: "Look"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "list", Arguments: `{"dir":"."}`}}},
			{Role: RoleTool, ToolCallID: "c1", Content: "a.go"},
		},
		Tools: []ToolDef{{Name: "read_file", Parameters: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Reading." || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Arguments != `{"path":"a.go"}` ||
		resp.ToolCalls[0].ID == "" || resp.Usage.TotalTokens != 15 || resp.FinishReason != "tool_calls" {
		t.Fatalf("response %+v", resp)
	}
	for _, want := range []string{
		"/v1beta/models/gemini-x:generateContent", `"systemInstruction"`, `"role":"model"`,
		`"functionResponse":{"name":"list"`, `"functionCall":{"args":{"dir":"."},"name":"list"}`,
		`"thinkingBudget":2048`, `"parametersJsonSchema":{"type":"object"}`,
	} {
		if !strings.Contains(*last, want) {
			t.Errorf("request lacks %s: %s", want, *last)
		}
	}
}

func TestGeminiStructuredOutputAndStream(t *testing.T) {
	srv, last := fakeGemini(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("stream path %s", r.URL)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ch := range []string{
			`{"candidates":[{"content":{"parts":[{"text":"{\"title\":"}]}}]}`,
			`{"candidates":[{"content":{"parts":[{"text":"\"T\"}"}]},"finishReason":"STOP"}],` +
				`"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`,
		} {
			_, _ = w.Write([]byte("data: " + ch + "\r\n\r\n"))
		}
	})
	var got strings.Builder
	resp, err := NewGemini(srv.URL, "g-key", time.Minute).ChatStream(context.Background(), Request{
		Model: "m", Messages: []Message{{Role: RoleUser, Content: "go"}},
		Format: &ResponseFormat{Name: "s", Schema: json.RawMessage(`{"type":"object"}`)},
	}, func(s string) { got.WriteString(s) })
	if err != nil || resp.Content != `{"title":"T"}` || got.String() != resp.Content || resp.Usage.TotalTokens != 5 {
		t.Fatalf("%v %q %+v", err, got.String(), resp)
	}
	if !strings.Contains(*last, `"responseMimeType":"application/json"`) ||
		!strings.Contains(*last, `"responseJsonSchema":{"type":"object"}`) {
		t.Fatalf("request %s", *last)
	}
}

func TestGeminiErrorsClassify(t *testing.T) {
	srv, _ := fakeGemini(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"overloaded"}}`))
	})
	_, err := NewGemini(srv.URL, "g-key", time.Minute).Chat(context.Background(),
		Request{Model: "m", Messages: []Message{{Role: RoleUser, Content: "q"}}})
	var api *APIError
	if !errors.As(err, &api) || Classify(err) != config.TriggerServerError {
		t.Fatalf("%v", err)
	}
}
