package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestConfigGetSetExportImport(t *testing.T) {
	h := newHarness(t)
	h.mustRun("config", "init")
	h.mustRun("config", "set", "defaults.model", "litellm/qwen")
	h.mustRun("config", "set", "limits.max_tokens_per_day", "50000")
	if out := h.mustRun("config", "get", "defaults.model"); strings.TrimSpace(out) != `"litellm/qwen"` {
		t.Fatalf("get: %q", out)
	}
	if code := h.run("config", "set", "defaults.bogus", "1"); code != ExitError {
		t.Fatalf("unknown key: exit %d", code)
	}
	exported := filepath.Join(h.dir, "export.json")
	h.mustRun("config", "export", exported)
	b, err := os.ReadFile(exported)
	if err != nil || !strings.Contains(string(b), `"max_tokens_per_day": 50000`) {
		t.Fatalf("%v %s", err, b)
	}
	h.mustRun("config", "set", "limits.max_tokens_per_day", "1")
	if code := h.run("config", "import", exported); code != ExitError {
		t.Fatal("import over an existing config needs --force")
	}
	h.mustRun("config", "import", "--force", exported)
	if out := h.mustRun("config", "get", "limits.max_tokens_per_day"); strings.TrimSpace(out) != "50000" {
		t.Fatalf("after import: %q", out)
	}
	if _, err := os.Stat(h.env.ConfigPath + ".bak"); err != nil {
		t.Fatalf("no backup of the replaced config: %v", err)
	}
	bad := filepath.Join(h.dir, "bad.json")
	_ = os.WriteFile(bad, []byte(`{"providers":[],"surprise":true}`), 0o600)
	if code := h.run("config", "import", "--force", bad); code != ExitError {
		t.Fatal("a config with unknown fields was imported")
	}
	if out := h.mustRun("config", "export"); !strings.Contains(out, `"providers"`) {
		t.Fatalf("export to stdout: %q", out)
	}
	for _, args := range [][]string{{"config", "get"}, {"config", "set", "x"}, {"config", "import"}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}

func TestServiceSavesToTheDefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	h := newHarness(t)
	h.env.ConfigPath = "" // the default location, as the desktop app starts the core
	want, err := configPath(h.env)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := newService(config.Default(), h.env)
	if err != nil {
		t.Fatal(err)
	}
	if svc.ConfigPath != want {
		t.Fatalf("settings and MCP servers would only change in memory: ConfigPath %q, want %q", svc.ConfigPath, want)
	}
}
