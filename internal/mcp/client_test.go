package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

func TestMain(m *testing.M) {
	mcptest.Main()
	os.Exit(m.Run())
}

func fake(t *testing.T, mode string) Server {
	t.Helper()
	prog, args, env := mcptest.Command(t, mode)
	return Server{Name: "fake", Command: prog, Args: args, Env: env}
}

func connect(t *testing.T, s Server) *Client {
	t.Helper()
	c, err := Connect(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestStdioHandshakeAndTools(t *testing.T) {
	c := connect(t, fake(t, mcptest.ModeOK))
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 2 || tools[0].Name != "echo" || string(tools[0].InputSchema) != `{"type":"object"}` {
		t.Fatalf("tools: %v %+v", err, tools)
	}
	res, err := c.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil || res.Text != "hi" || res.IsError {
		t.Fatalf("call: %v %+v", err, res)
	}
	res, _ = c.CallTool(context.Background(), "nope", nil)
	if !res.IsError {
		t.Fatalf("want isError: %+v", res)
	}
	prompts, err := c.ListPrompts(context.Background())
	if err != nil || len(prompts) != 1 || !prompts[0].Arguments[0].Required {
		t.Fatalf("prompts: %v %+v", err, prompts)
	}
	text, mime, err := c.ReadResource(context.Background(), "file:///notes.md")
	if err != nil || !strings.Contains(text, "Ship in March") || mime != "text/markdown" {
		t.Fatalf("read: %v %q %q", err, text, mime)
	}
}

func TestStdioPagination(t *testing.T) {
	c := connect(t, fake(t, mcptest.ModePages))
	res, err := c.ListResources(context.Background())
	if err != nil || len(res) != 3 {
		t.Fatalf("pages: %v %+v", err, res)
	}
}

func TestStdioInitializeTimeout(t *testing.T) {
	old := handshakeTimeout
	handshakeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { handshakeTimeout = old })
	start := time.Now()
	_, err := Connect(context.Background(), fake(t, mcptest.ModeSilent))
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("want a timeout error quickly, got %v after %v", err, time.Since(start))
	}
}

func TestStdioServerExits(t *testing.T) {
	c := connect(t, fake(t, mcptest.ModeExit))
	if _, err := c.ListTools(context.Background()); !errors.Is(err, ErrServerExited) {
		t.Fatalf("want ErrServerExited, got %v", err)
	}
	if _, err := c.ListTools(context.Background()); !errors.Is(err, ErrServerExited) {
		t.Fatalf("second call: %v", err)
	}
}

func TestStdioFloodBounded(t *testing.T) {
	_, err := Connect(context.Background(), fake(t, mcptest.ModeFlood))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

func TestStdioFailQuotesStderr(t *testing.T) {
	s := fake(t, mcptest.ModeFail)
	s.Env[mcptest.StderrEnv] = "boom: bad config"
	_, err := Connect(context.Background(), s)
	if err == nil || !strings.Contains(err.Error(), "boom: bad config") {
		t.Fatalf("want stderr in the error, got %v", err)
	}
}

func TestStdioMissingProgram(t *testing.T) {
	_, err := Connect(context.Background(), Server{Name: "x", Command: "tessivo-no-such-program"})
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestStdioEnvIsFiltered(t *testing.T) {
	t.Setenv("TESSIVO_TEST_PARENT_SECRET", "leak")
	s := fake(t, mcptest.ModeOK)
	s.Env["MINE"] = "given"
	c := connect(t, s)
	for name, want := range map[string]string{"MINE": "given", "TESSIVO_TEST_PARENT_SECRET": ""} {
		res, err := c.CallTool(context.Background(), "env", json.RawMessage(`{"name":"`+name+`"}`))
		if err != nil || res.Text != want {
			t.Fatalf("%s: %v %q", name, err, res.Text)
		}
	}
}

func TestCallResultJoinsText(t *testing.T) {
	c := connect(t, fake(t, mcptest.ModeOK))
	res, err := c.CallTool(context.Background(), "mixed", nil)
	if err != nil || res.Text != "see image\n[image: image/png]" {
		t.Fatalf("%v %q", err, res.Text)
	}
}

func TestReadResourceLimits(t *testing.T) {
	c := connect(t, fake(t, mcptest.ModeOK))
	if _, _, err := c.ReadResource(context.Background(), "file:///big"); !errors.Is(err, ErrResourceTooLarge) {
		t.Fatalf("big: %v", err)
	}
	if _, _, err := c.ReadResource(context.Background(), "file:///blob"); err == nil ||
		!strings.Contains(err.Error(), "binary") {
		t.Fatalf("blob: %v", err)
	}
}

func TestCloseStopsServer(t *testing.T) {
	c, err := Connect(context.Background(), fake(t, mcptest.ModeOK))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Close() }() // the result is read below
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close hung")
	}
	if _, err := c.ListTools(context.Background()); err == nil {
		t.Fatal("call after Close must fail")
	}
}
