package config

import (
	"strings"
	"testing"
)

func TestGetAndSetKeys(t *testing.T) {
	f := Default()
	f, err := f.Set("defaults.model", "litellm/qwen")
	if err != nil {
		t.Fatal(err)
	}
	if f, err = f.Set("limits.max_tokens_per_day", "50000"); err != nil {
		t.Fatal(err)
	}
	if f, err = f.Set("defaults.temperature", "0.3"); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.Get("defaults.model"); v != `"litellm/qwen"` {
		t.Fatalf("model: %s", v)
	}
	if v, _ := f.Get("limits.max_tokens_per_day"); v != "50000" {
		t.Fatalf("limit: %s", v)
	}
	if v, _ := f.Get("providers.0.name"); v != `"litellm"` {
		t.Fatalf("provider: %s", v)
	}
	if v, _ := f.Get("limits"); !strings.Contains(v, `"max_tokens_per_day": 50000`) {
		t.Fatalf("object: %s", v)
	}
	for _, bad := range [][2]string{
		{"defaults.bogus", "1"},              // unknown field
		{"defaults.thinking_effort", "huge"}, // invalid value
		{"limits.max_tokens_per_day", "lots"},
		{"providers.9.name", "x"}, // no such element
		{"", "x"},
	} {
		if _, err := f.Set(bad[0], bad[1]); err == nil {
			t.Errorf("set %s=%s accepted", bad[0], bad[1])
		}
	}
	if _, err := f.Get("defaults.nope"); err == nil {
		t.Error("get of a missing key succeeded")
	}
}

func TestParseIsStrictAndBounded(t *testing.T) {
	if _, err := Parse([]byte(`{"providers":[],"aliases":[],"defaults":{},"limits":{},"extra":1}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := Parse([]byte(strings.Repeat(" ", MaxConfigBytes+1))); err == nil {
		t.Fatal("oversized config accepted")
	}
	b, _ := Default().Marshal()
	if _, err := Parse(b); err != nil {
		t.Fatal(err)
	}
}

func TestSetCreatesMissingParents(t *testing.T) {
	f, err := Default().Set("agent.max_steps", "20")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := f.Get("agent.max_steps"); v != "20" {
		t.Fatalf("got %s", v)
	}
}
