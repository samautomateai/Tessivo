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

// fakeAnthropic records the last request body and answers with reply.
type antReply func(w http.ResponseWriter, body map[string]any)

func fakeAnthropic(t *testing.T, reply antReply) (*httptest.Server, *map[string]any) {
	t.Helper()
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "sk-ant" ||
			r.Header.Get("anthropic-version") == "" || r.Header.Get("Authorization") != "" {
			t.Errorf("request %s %v", r.URL.Path, r.Header)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &last)
		reply(w, last)
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func TestAnthropicChatMapsMessagesToolsAndCaching(t *testing.T) {
	srv, last := fakeAnthropic(t, func(w http.ResponseWriter, _ map[string]any) {
		_, _ = w.Write([]byte(`{"model":"claude-x","stop_reason":"tool_use",
			"usage":{"input_tokens":10,"output_tokens":5},
			"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Reading."},
			{"type":"tool_use","id":"tu1","name":"read_file","input":{"path":"a.go"}}]}`))
	})
	c := NewAnthropic(srv.URL, "sk-ant", time.Minute)
	resp, err := c.Chat(context.Background(), Request{
		Model: "claude-x", ReasoningEffort: "medium",
		Messages: []Message{
			{Role: RoleSystem, Content: "You analyze code."},
			{Role: RoleUser, Content: "Look at a.go"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "tu0", Name: "list", Arguments: `{}`}}},
			{Role: RoleTool, ToolCallID: "tu0", Content: "a.go"},
		},
		Tools: []ToolDef{{Name: "read_file", Description: "Read", Parameters: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Reading." || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Arguments != `{"path":"a.go"}` ||
		resp.Usage.TotalTokens != 15 || resp.FinishReason != "tool_calls" {
		t.Fatalf("response %+v", resp)
	}
	b, _ := json.Marshal(*last)
	body := string(b)
	for _, want := range []string{
		`"cache_control":{"type":"ephemeral"}`, `"thinking":{"budget_tokens":`,
		`"type":"tool_result"`, `"tool_use_id":"tu0"`, `"type":"tool_use"`, `"input_schema":{"type":"object"}`,
		`"max_tokens":`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("request lacks %s: %s", want, body)
		}
	}
	if strings.Contains(body, `"role":"system"`) || strings.Contains(body, `"role":"tool"`) {
		t.Errorf("system and tool roles must be mapped: %s", body)
	}
}

func TestAnthropicStructuredOutputUsesAForcedTool(t *testing.T) {
	srv, last := fakeAnthropic(t, func(w http.ResponseWriter, _ map[string]any) {
		_, _ = w.Write([]byte(`{"stop_reason":"tool_use","content":[{"type":"tool_use","id":"x","name":"respond",
			"input":{"title":"T"}}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	c := NewAnthropic(srv.URL, "sk-ant", time.Minute)
	resp, err := c.Chat(context.Background(), Request{
		Model: "m", Messages: []Message{{Role: RoleUser, Content: "go"}}, ReasoningEffort: "high",
		Format: &ResponseFormat{Name: "summary", Schema: json.RawMessage(`{"type":"object"}`)},
	})
	if err != nil || resp.Content != `{"title":"T"}` || len(resp.ToolCalls) != 0 {
		t.Fatalf("%v %+v", err, resp)
	}
	b, _ := json.Marshal(*last)
	if !strings.Contains(string(b), `"tool_choice":{"name":"respond","type":"tool"}`) ||
		strings.Contains(string(b), `"thinking"`) {
		t.Fatalf("forced tool without thinking expected: %s", b)
	}
}

func TestAnthropicStream(t *testing.T) {
	srv, _ := fakeAnthropic(t, func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []string{
			`{"type":"message_start","message":{"model":"claude-x","usage":{"input_tokens":7}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"secret plan"}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Ship "}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"in March."}}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"t1","name":"grep"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"x\"}"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
			`{"type":"message_stop"}`,
		} {
			_, _ = w.Write([]byte("event: x\ndata: " + ev + "\n\n"))
		}
	})
	var got strings.Builder
	resp, err := NewAnthropic(srv.URL, "sk-ant", time.Minute).ChatStream(context.Background(),
		Request{Model: "m", Messages: []Message{{Role: RoleUser, Content: "q"}}}, func(s string) { got.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "Ship in March." || resp.Content != "Ship in March." || resp.Usage.TotalTokens != 11 ||
		len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Arguments != `{"q":"x"}` || resp.Model != "claude-x" {
		t.Fatalf("deltas %q, response %+v", got.String(), resp)
	}
}

func TestAnthropicErrorsClassify(t *testing.T) {
	srv, _ := fakeAnthropic(t, func(w http.ResponseWriter, _ map[string]any) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down sk-ant"}}`))
	})
	_, err := NewAnthropic(srv.URL, "sk-ant", time.Minute).Chat(context.Background(),
		Request{Model: "m", Messages: []Message{{Role: RoleUser, Content: "q"}}})
	var api *APIError
	if !errors.As(err, &api) || Classify(err) != config.TriggerRateLimit || strings.Contains(err.Error(), "sk-ant") {
		t.Fatalf("%v", err)
	}
}
