// Package mcptest is a fake MCP server for tests. The test binary runs it as
// a child process: call Main first in TestMain, then start the server with
// the program, arguments and environment Command returns.
package mcptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// JSON keys the fake answers with.
const (
	kName        = "name"
	kDescription = "description"
	kContent     = "content"
	kMimeType    = "mimeType"
	kURI         = "uri"
	kType        = "type"
)

// Fake server behaviours, chosen with Command's mode.
const (
	ModeOK       = "ok"     // answers everything
	ModeSilent   = "silent" // never answers
	ModeExit     = "exit"   // answers initialize, then exits on the next request
	ModeFlood    = "flood"  // writes 6 MiB without a newline
	ModeFail     = "fail"   // prints $FAKE_STDERR to stderr and exits 1
	ModePages    = "pages"  // resources/list in three pages
	ModeStall    = "stall"  // answers initialize, then stops reading its input
	ModePings    = "pings"  // answers initialize, then sends 20000 pings and stops reading
	modeEnv      = "TESSIVO_MCP_FAKE"
	stderrEnv    = "FAKE_STDERR"
	helperMarker = "-tessivo-mcp-fake"
)

// Command returns how to start the fake in mode: the test binary itself.
func Command(t testing.TB, mode string) (program string, args []string, env map[string]string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe, []string{helperMarker}, map[string]string{modeEnv: mode}
}

// StderrEnv names the variable whose value ModeFail prints to stderr.
const StderrEnv = stderrEnv

// Main turns the process into the fake when Command started it, and exits.
func Main() {
	if len(os.Args) < 2 || os.Args[1] != helperMarker {
		return
	}
	os.Exit(Serve(os.Getenv(modeEnv), os.Stdin, os.Stdout, os.Stderr))
}

type msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
}

// Serve runs the fake over in and out until in closes.
func Serve(mode string, in io.Reader, out, errOut io.Writer) int {
	switch mode {
	case ModeFail:
		_, _ = fmt.Fprintln(errOut, os.Getenv(stderrEnv))
		return 1
	case ModeFlood:
		_, _ = out.Write([]byte(strings.Repeat("x", 6<<20)))
		_, _ = io.Copy(io.Discard, in)
		return 0
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	enc := json.NewEncoder(out)
	initialized := false
	for sc.Scan() {
		var m msg
		if json.Unmarshal(sc.Bytes(), &m) != nil || m.ID == nil || mode == ModeSilent {
			continue
		}
		if mode == ModeExit && initialized {
			return 0
		}
		initialized = initialized || m.Method == "initialize"
		_ = enc.Encode(msg{JSONRPC: "2.0", ID: m.ID, Result: answer(mode, m)})
		if initialized && (mode == ModeStall || mode == ModePings) {
			return stop(mode, enc)
		}
	}
	return 0
}

func answer(mode string, m msg) any {
	var p struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
		Cursor    string            `json:"cursor"`
		URI       string            `json:"uri"`
	}
	_ = json.Unmarshal(m.Params, &p)
	switch m.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
			"serverInfo": map[string]string{kName: "fake", "version": "1"},
		}
	case "tools/list":
		return map[string]any{"tools": []map[string]any{
			{kName: "echo", kDescription: "Echoes text", "inputSchema": map[string]any{kType: "object"}},
			{kName: "env", kDescription: "Reads an environment variable"},
		}}
	case "tools/call":
		return callTool(p.Name, p.Arguments)
	case "resources/list":
		return resources(mode, p.Cursor)
	case "resources/read":
		return readResource(p.URI)
	case "prompts/list":
		return map[string]any{"prompts": []map[string]any{{
			kName: "review", kDescription: "Review a diff",
			"arguments": []map[string]any{{kName: "diff", "required": true}},
		}}}
	}
	return map[string]any{}
}

func callTool(name string, args map[string]string) any {
	text := func(s string) map[string]string { return map[string]string{kType: "text", "text": s} }
	switch name {
	case "echo":
		return map[string]any{kContent: []any{text(args["text"])}}
	case "env":
		return map[string]any{kContent: []any{text(os.Getenv(args[kName]))}}
	case "big":
		return map[string]any{kContent: []any{text(strings.Repeat("b", 100<<10))}}
	case "mixed":
		return map[string]any{kContent: []any{
			text("see image"),
			map[string]string{kType: "image", kMimeType: "image/png", "data": "iVBORw0KGgo="},
		}}
	default:
		return map[string]any{kContent: []any{text("no such tool")}, "isError": true}
	}
}

func resources(mode, cursor string) any {
	if mode != ModePages {
		return map[string]any{"resources": []map[string]string{
			{kURI: "file:///notes.md", kName: "notes", kMimeType: "text/markdown"},
		}}
	}
	next := map[string]string{"": "p2", "p2": "p3", "p3": ""}[cursor]
	page := map[string]any{"resources": []map[string]string{{kURI: "file:///" + cursor + "x", kName: cursor}}}
	if next != "" {
		page["nextCursor"] = next
	}
	return page
}

func readResource(uri string) any {
	contents := map[string]string{kURI: uri, kMimeType: "text/markdown"}
	switch uri {
	case "file:///big":
		contents["text"] = strings.Repeat("r", 3<<20)
	case "file:///blob":
		contents[kMimeType], contents["blob"] = "image/png", "iVBORw0KGgo="
	default:
		contents["text"] = "# Notes\nShip in March."
	}
	return map[string]any{"contents": []any{contents}}
}

// stop sends pings (ModePings), then never reads its input again; it ends
// when the client kills it.
func stop(mode string, enc *json.Encoder) int {
	if mode == ModePings {
		for i := range 20000 {
			_ = enc.Encode(msg{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf(`"p%d"`, i)), Method: "ping"})
		}
	}
	for {
		time.Sleep(time.Hour) // select{} would end the process as a deadlock
	}
}
