package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

func fakeBody(t *testing.T) string {
	t.Helper()
	prog, args, env := mcptest.Command(t, mcptest.ModeOK)
	b, _ := json.Marshal(map[string]any{"name": "fake", "command": prog, "args": args, "env": env})
	return string(b)
}

func TestMCPServersAPI(t *testing.T) {
	ts, _ := newTestServer(t, "")
	if code, body := call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t)); code != 201 ||
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
	call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t))
	if code, _ := call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t)); code != 409 {
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

func TestNotebookMCPAPI(t *testing.T) {
	ts, _ := newTestServer(t, "")
	call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t))
	code, body := call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	if code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	if code, body = call(t, ts, "PUT", "/v1/notebooks/ops/mcp", `{"servers":["fake"]}`); code != 200 ||
		!strings.Contains(body, `"mcpServers":["fake"]`) {
		t.Fatalf("attach: %d %s", code, body)
	}
	if code, _ = call(t, ts, "DELETE", "/v1/mcp/servers/fake", ""); code != 409 {
		t.Fatalf("remove attached: %d", code)
	}
	if code, _ = call(t, ts, "PUT", "/v1/notebooks/ops/mcp", `{"servers":["nope"]}`); code != 400 {
		t.Fatalf("unknown: %d", code)
	}
}

func TestAddMCPContextAPI(t *testing.T) {
	ts, _ := newTestServer(t, "")
	call(t, ts, "POST", "/v1/mcp/servers", fakeBody(t))
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	code, body := call(t, ts, "POST", "/v1/notebooks/ops/context", `{"server":"fake","uri":"file:///notes.md"}`)
	if code != 200 || !strings.Contains(body, `"kind":"mcp-resource"`) {
		t.Fatalf("add: %d %s", code, body)
	}
	if code, _ = call(t, ts, "POST", "/v1/notebooks/ops/context", `{"server":"nope","uri":"x"}`); code != 400 {
		t.Fatalf("unknown server: %d", code)
	}
	body = `{"path":"/x","server":"fake","uri":"y"}`
	if code, _ = call(t, ts, "POST", "/v1/notebooks/ops/context", body); code != 400 {
		t.Fatalf("both path and server: %d", code)
	}
}
