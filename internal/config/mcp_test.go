package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPCheck(t *testing.T) {
	stdio := MCPServer{Name: "gh", Command: "npx", Args: []string{"-y", "server"}}
	remote := MCPServer{Name: "docs", URL: "https://mcp.example.com/mcp"}
	many := make([]MCPServer, MaxMCPServers+1)
	for i := range many {
		many[i] = MCPServer{Name: fmt.Sprintf("s%d", i), Command: "x"}
	}
	cases := []struct {
		name string
		cfg  MCPConfig
		ok   bool
	}{
		{"valid", MCPConfig{Servers: []MCPServer{stdio, remote}}, true},
		{"loopback http", MCPConfig{Servers: []MCPServer{{Name: "l", URL: "http://127.0.0.1:9000/mcp"}}}, true},
		{"bad name", MCPConfig{Servers: []MCPServer{{Name: "Bad Name", Command: "x"}}}, false},
		{"both", MCPConfig{Servers: []MCPServer{{Name: "b", Command: "x", URL: "https://a"}}}, false},
		{"neither", MCPConfig{Servers: []MCPServer{{Name: "n"}}}, false},
		{"remote http", MCPConfig{Servers: []MCPServer{{Name: "r", URL: "http://example.com/mcp"}}}, false},
		{"remote http allowed", MCPConfig{Servers: []MCPServer{
			{Name: "r", URL: "http://example.com/mcp", AllowInsecureHTTP: true},
		}}, true},
		{"url with credentials", MCPConfig{Servers: []MCPServer{{Name: "u", URL: "https://a:b@x.com"}}}, false},
		{"duplicate", MCPConfig{Servers: []MCPServer{stdio, stdio}}, false},
		{"too many", MCPConfig{Servers: many}, false},
	}
	for _, c := range cases {
		if err := c.cfg.Check(); (err == nil) != c.ok {
			t.Errorf("%s: err=%v", c.name, err)
		}
	}
}

func TestMCPRejectsLiteralSecret(t *testing.T) {
	bad := MCPServer{
		Name: "gh", URL: "https://x.com",
		Headers: map[string]string{"Authorization": "Bearer sk-abc123def456ghi789"},
	}
	if err := bad.Check(); err == nil || strings.Contains(err.Error(), "sk-abc") {
		t.Fatalf("literal token: %v", err)
	}
	badEnv := MCPServer{Name: "gh", Command: "x", Env: map[string]string{"TOKEN": "ghp_" + strings.Repeat("a", 36)}}
	if err := badEnv.Check(); err == nil {
		t.Fatal("literal env token accepted")
	}
	good := MCPServer{
		Name: "gh", URL: "https://x.com",
		Headers: map[string]string{"Authorization": "Bearer ${secret:GH}"},
	}
	if err := good.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestExpandSecrets(t *testing.T) {
	get := func(ref string) (string, error) {
		if ref == "GH" {
			return "tok", nil
		}
		return "", errors.New("not found")
	}
	cases := map[string]string{
		"Bearer ${secret:GH}":       "Bearer tok",
		"${secret:GH}-${secret:GH}": "tok-tok",
		"plain":                     "plain",
	}
	for in, want := range cases {
		if got, err := ExpandSecrets(in, get); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	if _, err := ExpandSecrets("x ${secret:MISSING}", get); err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := ExpandSecrets("${secret:bad name}", get); err == nil {
		t.Fatal("bad reference accepted")
	}
}

func TestConfigRoundTripsMCP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	f := Default()
	f.MCP.Servers = []MCPServer{
		{Name: "gh", Command: "npx", Args: []string{"-y", "s"}, Env: map[string]string{"A": "1"}},
	}
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := got.MCP.Find("gh"); !ok || s.Command != "npx" || s.Env["A"] != "1" {
		t.Fatalf("round trip: %+v", got.MCP)
	}
	f.MCP.Servers = append(f.MCP.Servers, f.MCP.Servers[0])
	if err := f.Validate(); err == nil {
		t.Fatal("Validate must check MCP servers")
	}
}

func TestMCPRejectsCredentialsInArgsAndURL(t *testing.T) {
	for _, s := range []MCPServer{
		{Name: "a", Command: "npx", Args: []string{"server", "--token=ghp_" + strings.Repeat("a", 36)}},
		{Name: "b", Command: "npx", Args: []string{"--api-key", "sk-abc123def456ghi789"}},
		{Name: "c", URL: "https://mcp.example.com/mcp?api_key=abc123def456"},
	} {
		if err := s.Check(); err == nil || !strings.Contains(err.Error(), "credential") {
			t.Errorf("%s: want a credential error, got %v", s.Name, err)
		}
	}
	ok := MCPServer{Name: "d", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}}
	if err := ok.Check(); err != nil {
		t.Fatal(err)
	}
}
