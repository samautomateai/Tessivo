package config

import (
	"encoding/json"
	"testing"
)

func TestRetriesDefaultAndExplicitZero(t *testing.T) {
	var a Alias
	if a.Retries() != DefaultRetries {
		t.Fatalf("unset: %d", a.Retries())
	}
	_ = json.Unmarshal([]byte(`{"name":"x","chain":[],"max_retries":0}`), &a)
	if a.Retries() != 0 {
		t.Fatalf("explicit 0 must turn retries off: %d", a.Retries())
	}
	f := Default()
	f.Aliases = append(f.Aliases, Alias{Name: "smart", Chain: []ModelRef{{Provider: "litellm", Model: "a"}}})
	n := 11
	f.Aliases[0].MaxRetries = &n
	if err := f.Validate(); err == nil {
		t.Fatal("11 retries accepted")
	}
	f.Aliases[0].MaxRetries = nil
	f.Providers[0].MaxConcurrent = 65
	if err := f.Validate(); err == nil {
		t.Fatal("max_concurrent 65 accepted")
	}
}

func intPtr(n int) *int { return &n }
