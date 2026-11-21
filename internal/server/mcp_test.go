package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

func fakeBody(t *testing.T, name, mode string) string {
	t.Helper()
	prog, args, env := mcptest.Command(t, mode)
	b, _ := json.Marshal(map[string]any{"name": name, "command": prog, "args": args, "env": env})
	return string(b)
}

func TestMCPServersAPI(t *testing.T) {
	ts, _ := newTestServer(t, "")
	if code, body := call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t, "fake", mcptest.ModeOK)); code != 201 ||
		!strings.Contains(body, `"name":"fake"`) {
		t.Fatalf("add: %d %s", code, body)
	}
	if code, body := call(t, ts, "GET", "/v1/mcp/servers", ""); code != 200 || !strings.Contains(body, "fake") {
		t.Fatalf("list: %d %s", code, body)
	}
	code, body := call(t, ts, "POST", "/v1/mcp/servers/fake/test", "")
	if code != 200 || !strings.Contains(body, `"name":"echo"`) || !strings.Contains(body, "file:///notes.md") {
		t.Fatalf("test: %d %s", code, body)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/mcp/servers/fake", ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
}

func TestMCPAddConflict409(t *testing.T) {
	ts, _ := newTestServer(t, "")
	call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t, "fake", mcptest.ModeOK))
	if code, _ := call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t, "fake", mcptest.ModeOK)); code != 409 {
		t.Fatalf("duplicate: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/mcp/servers", `{"name":"Bad Name","command":"x"}`); code != 400 {
		t.Fatalf("invalid: %d", code)
	}
}

func TestMCPTestUnknown404(t *testing.T) {
	ts, _ := newTestServer(t, "")
	if code, _ := call(t, ts, "POST", "/v1/mcp/servers/nope/test", ""); code != 404 {
		t.Fatalf("test unknown: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/mcp/servers/nope", ""); code != 404 {
		t.Fatalf("delete unknown: %d", code)
	}
}
