package agent

import (
	"strings"
	"testing"
)

func TestDecideOrder(t *testing.T) {
	p := Policy{
		Run:      map[string]string{"grep": "block"},
		Notebook: map[string]string{"grep": "allow", "mcp__jira__*": "allow"},
		App:      map[string]string{"mcp__*": "block"},
	}
	cases := map[string]Decision{
		"grep":         Block, // run beats notebook
		"mcp__jira__x": Allow, // notebook pattern beats app pattern
		"mcp__wiki__x": Block, // app pattern
		"unknown_tool": Ask,   // nothing matches: ask
		"read_file":    Allow, // built-in default
		"finish":       Allow,
	}
	for tool, want := range cases {
		if got := p.Decide(tool); got != want {
			t.Errorf("Decide(%q) = %s, want %s", tool, got, want)
		}
	}
}

func TestDecideExactBeatsPattern(t *testing.T) {
	p := Policy{Notebook: map[string]string{"mcp__*": "block", "mcp__jira__*": "ask", "mcp__jira__search": "allow"}}
	if got := p.Decide("mcp__jira__search"); got != Allow {
		t.Fatalf("exact name: %s", got)
	}
	if got := p.Decide("mcp__jira__create"); got != Ask {
		t.Fatalf("longest pattern: %s", got)
	}
	if got := p.Decide("mcp__x"); got != Block {
		t.Fatalf("short pattern: %s", got)
	}
}

func TestPolicyCheck(t *testing.T) {
	err := Policy{Notebook: map[string]string{"grep": "maybe"}}.Check()
	if err == nil || !strings.Contains(err.Error(), "notebook") || !strings.Contains(err.Error(), "grep") {
		t.Fatalf("want error naming notebook and grep, got %v", err)
	}
	if err := (Policy{Run: map[string]string{"x": "allow"}}).Check(); err != nil {
		t.Fatal(err)
	}
}

func TestParseDecision(t *testing.T) {
	for _, s := range []string{"allow", "ask", "block"} {
		if d, err := ParseDecision(s); err != nil || string(d) != s {
			t.Errorf("ParseDecision(%q) = %v, %v", s, d, err)
		}
	}
	if _, err := ParseDecision("yes"); err == nil {
		t.Fatal("want error")
	}
}
