package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLayers(t *testing.T) {
	e := Resolve(
		Layer{"app", RunSettings{
			Model: "fast", Temperature: Ptr(0.2), ThinkingEffort: ThinkingLow, Mode: ModePipeline,
		}},
		Layer{"notebook", RunSettings{Model: "smart", ContextBudget: Ptr(1000)}},
		Layer{"run", RunSettings{ThinkingEffort: ThinkingHigh, MaxOutputTokens: Ptr(50), Mode: ModeAgent}},
	)
	if e.Model != "smart" || e.Sources["model"] != "notebook" {
		t.Fatalf("model: %q from %q", e.Model, e.Sources["model"])
	}
	if *e.Temperature != 0.2 || e.Sources["temperature"] != "app" {
		t.Fatal("temperature should come from app")
	}
	if e.ThinkingEffort != ThinkingHigh || e.Sources["thinking_effort"] != "run" {
		t.Fatal("thinking effort should come from run")
	}
	if *e.MaxOutputTokens != 50 || *e.ContextBudget != 1000 || e.Mode != ModeAgent {
		t.Fatalf("unexpected: %+v", e)
	}
}

func TestRunSettingsValidate(t *testing.T) {
	bad := []RunSettings{
		{ThinkingEffort: "max"},
		{Mode: "turbo"},
		{Temperature: Ptr(3.0)},
		{MaxOutputTokens: Ptr(0)},
		{ContextBudget: Ptr(-1)},
	}
	for _, s := range bad {
		if s.Validate() == nil {
			t.Errorf("want error for %+v", s)
		}
	}
	if err := (RunSettings{ThinkingEffort: ThinkingOff, Mode: ModeAgent}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func validFile() File {
	f := Default()
	f.Aliases = []Alias{{Name: "smart", Chain: []ModelRef{{"litellm", "a"}, {"litellm", "b"}}}}
	return f
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.json")
	f := validFile()
	if err := f.Save(p); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v, want 0600", info.Mode().Perm())
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Aliases) != 1 || got.Aliases[0].Chain[1].Model != "b" {
		t.Fatalf("round trip lost data: %+v", got)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("want error for missing file")
	}
	unknown := filepath.Join(dir, "u.json")
	_ = os.WriteFile(unknown, []byte(`{"api_key":"secret"}`), 0o600)
	if _, err := Load(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("want unknown field error, got %v", err)
	}
	invalid := filepath.Join(dir, "i.json")
	_ = os.WriteFile(invalid, []byte(`{"providers":[{"name":"x","base_url":"ftp://x"}]}`), 0o600)
	if _, err := Load(invalid); err == nil {
		t.Fatal("want validation error")
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	f := File{Providers: []Provider{{Name: ""}}}
	if err := f.Save(filepath.Join(t.TempDir(), "c.json")); err == nil {
		t.Fatal("want error")
	}
	g := validFile()
	if err := g.Save(filepath.Join("/proc", "nope", "c.json")); err == nil {
		t.Fatal("want error writing to an unwritable dir")
	}
}

func TestValidateAliases(t *testing.T) {
	f := validFile()
	f.Providers = append(f.Providers, Provider{Name: "litellm", BaseURL: "http://x"})
	f.Aliases = append(f.Aliases,
		Alias{Name: "smart", Chain: []ModelRef{{"litellm", "a"}}},
		Alias{Name: "a/b"},
		Alias{Name: "x", Chain: []ModelRef{{"nope", "m"}}, FallbackOn: []string{"weather"}, MaxRetries: 11},
	)
	f.Defaults.Mode = "bad"
	err := f.Validate()
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{"duplicate name", "duplicate", "must not contain", "chain needs", "unknown provider",
		"unknown fallback trigger", "max_retries", "defaults"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestResolveModel(t *testing.T) {
	f := validFile()
	a, err := f.ResolveModel("smart")
	if err != nil || len(a.Chain) != 2 {
		t.Fatalf("alias: %v %+v", err, a)
	}
	if got := a.Triggers(); len(got) != len(DefaultTriggers) {
		t.Fatal("want default triggers")
	}
	a.FallbackOn = []string{TriggerTimeout}
	if got := a.Triggers(); len(got) != 1 {
		t.Fatal("want custom triggers")
	}
	direct, err := f.ResolveModel("litellm/qwen")
	if err != nil || direct.Chain[0].Model != "qwen" {
		t.Fatalf("direct: %v %+v", err, direct)
	}
	for _, bad := range []string{"nope", "other/m", "litellm/"} {
		if _, err := f.ResolveModel(bad); err == nil {
			t.Errorf("want error for %q", bad)
		}
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("TESSIVO_CONFIG", "/tmp/x.json")
	if p, _ := DefaultPath(); p != "/tmp/x.json" {
		t.Fatalf("got %q", p)
	}
	t.Setenv("TESSIVO_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	p, err := DefaultPath()
	if err != nil || !strings.HasSuffix(p, filepath.Join("tessivo", "config.json")) {
		t.Fatalf("got %q %v", p, err)
	}
}

func TestProviderLookup(t *testing.T) {
	f := validFile()
	if _, ok := f.Provider("litellm"); !ok {
		t.Fatal("want provider")
	}
	if _, ok := f.Provider("nope"); ok {
		t.Fatal("want miss")
	}
}
