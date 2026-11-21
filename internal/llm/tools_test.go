package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func readTool() ToolDef {
	return ToolDef{
		Name: "read_file", Description: "Read a file",
		Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}
}

func TestToWireTools(t *testing.T) {
	req := Request{
		Model: "m", Tools: []ToolDef{readTool()}, ToolChoice: "auto",
		Messages: []Message{
			{Role: RoleUser, Content: "go"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read_file", Arguments: `{"path":"a"}`}}},
			{Role: RoleTool, ToolCallID: "c1", Content: "text"},
		},
	}
	b, err := json.Marshal(toWire(req, false))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ToolChoice string `json:"tool_choice"`
		Tools      []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string         `json:"name"`
				Parameters map[string]any `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ToolChoice != "auto" || len(got.Tools) != 1 || got.Tools[0].Type != "function" ||
		got.Tools[0].Function.Name != "read_file" || got.Tools[0].Function.Parameters["type"] != "object" {
		t.Fatalf("tools on the wire: %s", b)
	}
	calls, ok := got.Messages[1]["tool_calls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("assistant tool_calls: %s", b)
	}
	call := calls[0].(map[string]any)
	fn := call["function"].(map[string]any)
	if call["id"] != "c1" || call["type"] != "function" || fn["name"] != "read_file" ||
		fn["arguments"] != `{"path":"a"}` {
		t.Fatalf("tool call: %v", call)
	}
	if got.Messages[2]["role"] != "tool" || got.Messages[2]["tool_call_id"] != "c1" {
		t.Fatalf("tool message: %v", got.Messages[2])
	}
	if _, has := got.Messages[0]["tool_calls"]; has {
		t.Fatal("plain message must not carry tool_calls")
	}
}

func TestChatParsesToolCalls(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"model":"m","choices":[{"message":{"role":"assistant","content":"",`+
			`"tool_calls":[{"id":"a","type":"function",`+
			`"function":{"name":"read_file","arguments":"{\"path\":\"x\"}"}},`+
			`{"id":"b","type":"function","function":{"name":"grep","arguments":"{}"}}]},`+
			`"finish_reason":"tool_calls"}]}`)
	})
	resp, err := c.Chat(context.Background(), Request{Model: "m", Tools: []ToolDef{readTool()}})
	if err != nil {
		t.Fatal(err)
	}
	want := wantCalls()
	if resp.FinishReason != "tool_calls" || len(resp.ToolCalls) != 2 || resp.ToolCalls[0] != want[0] ||
		resp.ToolCalls[1] != want[1] {
		t.Fatalf("tool calls: %+v", resp)
	}
}

func wantCalls() []ToolCall {
	return []ToolCall{
		{ID: "a", Name: "read_file", Arguments: `{"path":"x"}`},
		{ID: "b", Name: "grep", Arguments: "{}"},
	}
}

// callDelta is one streamed tool-call fragment as a full SSE chunk.
func callDelta(body string) string {
	return `{"choices":[{"delta":{"tool_calls":[` + body + `]}}]}`
}

func sse(chunks ...string) string {
	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString("data: " + c + "\n\n")
	}
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

func TestStreamAssemblesToolCalls(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sse(
			callDelta(`{"index":0,"id":"a","function":{"name":"read_file","arguments":"{\"pa"}}`),
			callDelta(`{"index":0,"function":{"arguments":"th\":"}}`),
			callDelta(`{"index":1,"id":"b","function":{"name":"grep","arguments":"{}"}}`),
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]},`+
				`"finish_reason":"tool_calls"}]}`,
		))
	})
	var deltas []string
	resp, err := c.ChatStream(context.Background(), Request{Model: "m"}, func(s string) { deltas = append(deltas, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Fatalf("argument fragments reached onDelta: %q", deltas)
	}
	want := wantCalls()
	if len(resp.ToolCalls) != 2 || resp.ToolCalls[0] != want[0] || resp.ToolCalls[1] != want[1] {
		t.Fatalf("tool calls: %+v", resp.ToolCalls)
	}
}

func TestStreamToolCallLimits(t *testing.T) {
	tooMany := make([]string, 0, MaxToolCalls+1)
	for i := 0; i <= MaxToolCalls; i++ {
		tooMany = append(tooMany, callDelta(fmt.Sprintf(
			`{"index":%d,"id":"c%d","function":{"name":"t","arguments":"{}"}}`, i, i)))
	}
	big := `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"t","arguments":"` +
		strings.Repeat("x", MaxToolCallArgs+1) + `"}}]}}]}`
	for name, tc := range map[string]struct {
		body, want string
	}{
		"count": {sse(tooMany...), "16"},
		"size":  {sse(big), "64 KiB"},
	} {
		t.Run(name, func(t *testing.T) {
			c := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, tc.body) })
			_, err := c.ChatStream(context.Background(), Request{Model: "m"}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error mentioning %q, got %v", tc.want, err)
			}
		})
	}
}

func TestToolsRejected(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"this model does not support tools"}`)
	})
	req := Request{Model: "m", Tools: []ToolDef{readTool()}}
	if _, err := c.Chat(context.Background(), req); !errors.Is(err, ErrToolsUnsupported) {
		t.Fatalf("chat: want ErrToolsUnsupported, got %v", err)
	}
	if _, err := c.ChatStream(context.Background(), req, nil); !errors.Is(err, ErrToolsUnsupported) {
		t.Fatalf("stream: want ErrToolsUnsupported, got %v", err)
	}
	if _, err := c.Chat(context.Background(), Request{Model: "m"}); errors.Is(err, ErrToolsUnsupported) {
		t.Fatal("a request without tools must not report ErrToolsUnsupported")
	}
}

func FuzzToolCallDeltas(f *testing.F) {
	f.Add(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"t","arguments":"{}"}}]}}]}`)
	f.Add(`{"choices":[{"delta":{"tool_calls":[{"index":-1}]}}]}`)
	f.Add(`{"choices":[{"delta":{"tool_calls":[{"index":99999999}]}}]}`)
	f.Fuzz(func(t *testing.T, chunk string) {
		var ch wireChunk
		if json.Unmarshal([]byte(chunk), &ch) != nil {
			return
		}
		var out Response
		var sb strings.Builder
		acc := &toolAccumulator{}
		for range 3 {
			if err := applyChunk(&out, &sb, acc, ch, nil); err != nil {
				return
			}
		}
		calls := acc.calls()
		if len(calls) > MaxToolCalls {
			t.Fatalf("%d calls", len(calls))
		}
		for _, c := range calls {
			if len(c.Arguments) > MaxToolCallArgs {
				t.Fatalf("arguments of %d bytes", len(c.Arguments))
			}
		}
	})
}
