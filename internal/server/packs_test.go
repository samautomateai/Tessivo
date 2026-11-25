package server

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/pack"
)

func TestPacksAPI(t *testing.T) {
	ts, s := newTestServer(t, summary)
	s.svc.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	dir := filepath.Join(t.TempDir(), "team")
	if err := pack.Init(dir, "team", "Team pack"); err != nil {
		t.Fatal(err)
	}
	src, _ := json.Marshal(map[string]string{"source": dir})
	code, body := call(t, ts, "POST", "/v1/packs", string(src))
	if code != 201 || !strings.Contains(body, `"id":"team"`) {
		t.Fatalf("add: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/packs", string(src)); code != 409 {
		t.Fatalf("add twice: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/packs", `{"source":"file:///x"}`); code != 400 {
		t.Fatalf("file url: %d", code)
	}
	code, body = call(t, ts, "GET", "/v1/packs", "")
	if code != 200 || !strings.Contains(body, `"name":"Team pack"`) {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, body := call(t, ts, "POST", "/v1/packs/refresh", ""); code != 200 || !strings.Contains(body, "team") {
		t.Fatalf("refresh: %d %s", code, body)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/packs/team", ""); code != 204 {
		t.Fatalf("remove: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/packs/team", ""); code != 404 {
		t.Fatalf("remove twice: %d", code)
	}
}

func TestTrustPackServerAPI(t *testing.T) {
	ts, s := newTestServer(t, summary)
	s.svc.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	dir := filepath.Join(t.TempDir(), "team")
	if err := pack.Init(dir, "team", "Team pack"); err != nil {
		t.Fatal(err)
	}
	servers := []config.MCPServer{{Name: "github", Command: "npx"}}
	if err := pack.Export(dir, pack.Manifest{ID: "team", Name: "Team pack"}, nil, nil, servers); err != nil {
		t.Fatal(err)
	}
	src, _ := json.Marshal(map[string]string{"source": dir})
	call(t, ts, "POST", "/v1/packs", string(src))
	code, body := call(t, ts, "GET", "/v1/mcp/servers", "")
	if code != 200 || !strings.Contains(body, `"name":"team/github"`) || !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, body := call(t, ts, "POST", "/v1/mcp/servers/team%2Fgithub/trust", ""); code != 204 {
		t.Fatalf("trust: %d %s", code, body)
	}
	if _, body := call(t, ts, "GET", "/v1/mcp/servers", ""); !strings.Contains(body, `"trusted":true`) {
		t.Fatalf("after trust: %s", body)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/mcp/servers/team%2Fgithub/trust", ""); code != 204 {
		t.Fatalf("untrust: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/mcp/servers/nope/trust", ""); code != 400 {
		t.Fatalf("unknown: %d", code)
	}
}
