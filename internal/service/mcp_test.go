package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
	"github.com/samautomateai/tessivo/internal/secrets"
)

const fakeSecret = "s3cr3t-value-for-tests"

// fakeServer is a config entry that starts the fake MCP server in mode.
func fakeServer(t *testing.T, name, mode string) config.MCPServer {
	t.Helper()
	prog, args, env := mcptest.Command(t, mode)
	return config.MCPServer{Name: name, Command: prog, Args: args, Env: env}
}

func mcpSvc(t *testing.T) *Service {
	t.Helper()
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	if err := s.Config.Save(s.ConfigPath); err != nil {
		t.Fatal(err)
	}
	s.Secrets = secrets.Map{"TESSIVO_LITELLM_KEY": "k", "GH": fakeSecret}
	return s
}

func TestAddListRemoveMCP(t *testing.T) {
	s := mcpSvc(t)
	srv := fakeServer(t, "fake", mcptest.ModeOK)
	srv.Env["GH_TOKEN"] = "${secret:GH}"
	if _, err := s.AddMCPServer(srv); err != nil {
		t.Fatal(err)
	}
	onDisk, err := config.Load(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := onDisk.MCP.Find("fake"); !ok || got.Env["GH_TOKEN"] != "${secret:GH}" {
		t.Fatalf("not saved: %+v", onDisk.MCP)
	}
	list := s.MCPServers()
	if len(list) != 1 || list[0].Env["GH_TOKEN"] != "${secret:GH}" {
		t.Fatalf("list: %+v", list)
	}
	if err := s.RemoveMCPServer("fake"); err != nil {
		t.Fatal(err)
	}
	if onDisk, _ = config.Load(s.ConfigPath); len(onDisk.MCP.Servers) != 0 || len(s.MCPServers()) != 0 {
		t.Fatalf("not removed: %+v", onDisk.MCP)
	}
	if err := s.RemoveMCPServer("fake"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove unknown: %v", err)
	}
}

func TestAddMCPInvalidAndDuplicate(t *testing.T) {
	s := mcpSvc(t)
	if _, err := s.AddMCPServer(config.MCPServer{Name: "Bad Name", Command: "x"}); !IsInvalid(err) {
		t.Fatalf("invalid: %v", err)
	}
	srv := fakeServer(t, "fake", mcptest.ModeOK)
	if _, err := s.AddMCPServer(srv); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddMCPServer(srv); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestRemoveAttachedMCP409(t *testing.T) {
	s := mcpSvc(t)
	if _, err := s.AddMCPServer(fakeServer(t, "fake", mcptest.ModeOK)); err != nil {
		t.Fatal(err)
	}
	info, _ := s.CreateNotebook("Ops", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	nb.Manifest.Agent.MCPServers = []string{"fake"}
	if err := nb.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMCPServer("fake"); !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "Ops") {
		t.Fatalf("attached: %v", err)
	}
}

func TestTestMCPServerListsEverything(t *testing.T) {
	s := mcpSvc(t)
	if _, err := s.AddMCPServer(fakeServer(t, "fake", mcptest.ModeOK)); err != nil {
		t.Fatal(err)
	}
	res, err := s.TestMCPServer(context.Background(), "fake")
	if err != nil || len(res.Tools) != 2 || len(res.Resources) != 1 || len(res.Prompts) != 1 {
		t.Fatalf("%v %+v", err, res)
	}
	if _, err := s.TestMCPServer(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestConnectMCPExpandsSecrets(t *testing.T) {
	s := mcpSvc(t)
	srv := fakeServer(t, "fake", mcptest.ModeOK)
	srv.Env["GH_TOKEN"] = "${secret:GH}"
	if _, err := s.AddMCPServer(srv); err != nil {
		t.Fatal(err)
	}
	c, err := s.connectMCP(context.Background(), "fake")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	res, err := c.CallTool(context.Background(), "env", json.RawMessage(`{"name":"GH_TOKEN"}`))
	if err != nil || res.Text != fakeSecret {
		t.Fatalf("secret did not reach the server: %v %q", err, res.Text)
	}
	raw, _ := os.ReadFile(s.ConfigPath)
	if strings.Contains(string(raw), fakeSecret) {
		t.Fatal("secret written to the config")
	}
}

func TestMCPErrorsRedacted(t *testing.T) {
	s := mcpSvc(t)
	srv := fakeServer(t, "fake", mcptest.ModeFail)
	srv.Env[mcptest.StderrEnv] = "bad token ${secret:GH}"
	if _, err := s.AddMCPServer(srv); err != nil {
		t.Fatal(err)
	}
	_, err := s.TestMCPServer(context.Background(), "fake")
	if err == nil || strings.Contains(err.Error(), fakeSecret) || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("want a redacted error, got %v", err)
	}
	srv2 := fakeServer(t, "nosecret", mcptest.ModeOK)
	srv2.Env["X_TOKEN"] = "${secret:MISSING}"
	if _, err := s.AddMCPServer(srv2); err != nil {
		t.Fatal(err)
	}
	_, err = s.TestMCPServer(context.Background(), "nosecret")
	if err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("missing secret: %v", err)
	}
}
