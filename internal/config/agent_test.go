package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigAgentDefaults(t *testing.T) {
	got := AgentConfig{}.WithDefaults()
	if got.MaxSteps != 40 || got.MaxTokens != 400000 || got.MaxMinutes != 30 || got.ApprovalExpiryDays != 7 {
		t.Fatalf("defaults: %+v", got)
	}
	kept := AgentConfig{MaxSteps: 5, MaxTokens: 10, MaxMinutes: 1, ApprovalExpiryDays: 2}.WithDefaults()
	if kept.MaxSteps != 5 || kept.MaxTokens != 10 || kept.MaxMinutes != 1 || kept.ApprovalExpiryDays != 2 {
		t.Fatalf("set values must be kept: %+v", kept)
	}
}

func TestConfigRejectsBadToolRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"providers":[],"aliases":[],"defaults":{},"limits":{},"agent":{"tools":{"grep":"maybe"}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "grep") || !strings.Contains(err.Error(), "maybe") {
		t.Fatalf("want a rule error naming grep, got %v", err)
	}
	if err := (AgentConfig{MaxSteps: -1}).Validate(); err == nil {
		t.Fatal("negative limits must be refused")
	}
}

func TestCheckToolRules(t *testing.T) {
	ok := map[string]string{"grep": "allow", "mcp__*": "ask", "x": "block"}
	if err := CheckToolRules("notebook", ok); err != nil {
		t.Fatal(err)
	}
	err := CheckToolRules("notebook", map[string]string{"grep": "maybe"})
	if err == nil || !strings.Contains(err.Error(), "notebook") || !strings.Contains(err.Error(), "grep") {
		t.Fatalf("want an error naming the layer and key, got %v", err)
	}
	if err := CheckToolRules("app", map[string]string{"": "allow"}); err == nil {
		t.Fatal("an empty tool name must be refused")
	}
}

func TestDefaultConfigHasAgentBlock(t *testing.T) {
	d := Default()
	if d.Agent.MaxSteps != 40 || d.Agent.ApprovalExpiryDays != 7 {
		t.Fatalf("starter config agent block: %+v", d.Agent)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
