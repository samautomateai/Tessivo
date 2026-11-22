package agenttools

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
)

type fakeCaller struct {
	text    string
	isError bool
	err     error
	called  string
}

func (f *fakeCaller) CallTool(_ context.Context, name string, _ json.RawMessage) (string, bool, error) {
	f.called = name
	return f.text, f.isError, f.err
}

func TestSanitizeToolName(t *testing.T) {
	cases := map[[2]string]string{
		{"gh", "search_issues"}:        "mcp__gh__search_issues",
		{"my-docs", "Read.File"}:       "mcp__my_docs__read_file",
		{"x", strings.Repeat("a", 80)}: "mcp__x__" + strings.Repeat("a", 56),
		{"x", "ünïcode tool"}:          "mcp__x___n_code_tool",
	}
	for in, want := range cases {
		if got := SanitizeToolName(in[0], in[1]); got != want {
			t.Errorf("%v: %q, want %q", in, got, want)
		}
	}
}

var toolNameRE = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

func FuzzSanitizeToolName(f *testing.F) {
	f.Add("gh", "search")
	f.Add("", "")
	f.Add("A-B", "\x00\xff")
	f.Fuzz(func(t *testing.T, server, tool string) {
		if got := SanitizeToolName(server, tool); !toolNameRE.MatchString(got) {
			t.Fatalf("%q", got)
		}
	})
}

func TestMCPToolsDedupe(t *testing.T) {
	tools := MCPTools("x", []MCPToolSpec{{Name: "Read"}, {Name: "read"}, {Name: "READ"}}, &fakeCaller{}, nil)
	var names []string
	for _, tl := range tools {
		names = append(names, tl.Def().Name)
	}
	if strings.Join(names, ",") != "mcp__x__read,mcp__x__read_2,mcp__x__read_3" {
		t.Fatalf("names: %v", names)
	}
}

func TestMCPToolRunAndSchema(t *testing.T) {
	c := &fakeCaller{text: "result"}
	tools := MCPTools("gh", []MCPToolSpec{
		{Name: "search", Description: "Search issues", Schema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "zbad", Schema: json.RawMessage(`[1,2]`)}, // sorts after "search"
	}, c, nil)
	if def := tools[0].Def(); !strings.Contains(def.Description, "gh") || string(def.Parameters) !=
		`{"type":"object","properties":{}}` {
		t.Fatalf("def: %+v", def)
	}
	if string(tools[1].Def().Parameters) != `{"type":"object"}` {
		t.Fatalf("bad schema not replaced: %s", tools[1].Def().Parameters)
	}
	out, err := tools[0].Run(context.Background(), json.RawMessage(`{}`))
	if err != nil || out != "result" || c.called != "search" {
		t.Fatalf("run: %q %v %q", out, err, c.called)
	}
	c.isError = true
	if out, _ := tools[0].Run(context.Background(), nil); out != "error: result" {
		t.Fatalf("isError: %q", out)
	}
	c.err = errors.New("server exited")
	if _, err := tools[0].Run(context.Background(), nil); err == nil {
		t.Fatal("want error")
	}
}

// The same tools listed in another order (after a pause, say) keep their names.
func TestMCPToolNamesIgnoreListingOrder(t *testing.T) {
	names := func(specs []MCPToolSpec) map[string]string {
		out := map[string]string{}
		for _, tl := range MCPTools("x", specs, &fakeCaller{}, nil) {
			out[tl.(*mcpTool).remote] = tl.Def().Name
		}
		return out
	}
	a := names([]MCPToolSpec{{Name: "Read"}, {Name: "read"}})
	b := names([]MCPToolSpec{{Name: "read"}, {Name: "Read"}})
	if a["Read"] != b["Read"] || a["read"] != b["read"] {
		t.Fatalf("names depend on order: %v vs %v", a, b)
	}
}

// Two servers whose names sanitize alike must not offer the same tool name.
func TestMCPToolNamesUniqueAcrossServers(t *testing.T) {
	taken := map[string]bool{}
	first := MCPTools("a-b", []MCPToolSpec{{Name: "x"}}, &fakeCaller{}, taken)
	second := MCPTools("a_b", []MCPToolSpec{{Name: "x"}}, &fakeCaller{}, taken)
	if first[0].Def().Name == second[0].Def().Name {
		t.Fatalf("both servers offer %q", first[0].Def().Name)
	}
}
