package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/template"
)

func write(t *testing.T, dir, name string, v any) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	b, ok := v.([]byte)
	if !ok {
		var err error
		if b, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func goodTemplate(id string) template.Template {
	return template.Template{
		ID: id, Name: "Risk review", WorkType: "analyze", Description: "Change risk",
		Prompts: []template.Prompt{{Name: "review", Text: "Assess the risk."}},
	}
}

// teamPack writes a pack with one of each item.
func teamPack(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, ManifestFile, Manifest{
		FormatVersion: 1, ID: "team", Name: "Team pack", Publisher: "Platform",
		Recommended: []string{"prompt:triage"}, Required: []string{"template:risk-review", "mcp:github"},
	})
	write(t, dir, "templates/risk-review.json", goodTemplate("risk-review"))
	write(t, dir, "prompts/triage.json", Prompt{Name: "triage", Text: "Triage these issues.", WorkType: "analyze"})
	write(t, dir, MCPFile, []config.MCPServer{{
		Name: "github", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{"GITHUB_TOKEN": "${secret:GITHUB_TOKEN}"},
	}})
	write(t, dir, "README.md", []byte("# Team pack\nUse the risk review."))
	return dir
}

func TestLoadAGoodPack(t *testing.T) {
	p, err := Load(teamPack(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Templates) != 1 || len(p.Prompts) != 1 || len(p.MCP) != 1 || len(p.Problems) != 0 {
		t.Fatalf("%+v", p)
	}
	if p.Templates[0].Source != template.SourcePack || !strings.Contains(p.README, "risk review") {
		t.Fatalf("source %q readme %q", p.Templates[0].Source, p.README)
	}
	for ref, want := range map[string]string{
		"template:risk-review": MarkRequired, "mcp:github": MarkRequired, "prompt:triage": MarkRecommended,
		"prompt:other": "",
	} {
		if got := p.Mark(ref); got != want {
			t.Errorf("Mark(%s) = %q, want %q", ref, got, want)
		}
	}
}

func TestBadItemsBecomeProblems(t *testing.T) {
	dir := teamPack(t)
	bad := goodTemplate("broken")
	bad.Prompts = nil
	write(t, dir, "templates/broken.json", bad)
	write(t, dir, "prompts/leak.json", Prompt{Name: "leak", Text: "use ghp_" + strings.Repeat("a", 36)})
	write(t, dir, "prompts/junk.json", []byte("{not json"))
	write(t, dir, MCPFile, []config.MCPServer{
		{Name: "github", Command: "npx"},
		{
			Name: "jira", URL: "https://jira.example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer abc123"},
		},
	})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Templates) != 1 || len(p.Prompts) != 1 || len(p.MCP) != 1 {
		t.Fatalf("good items must still load: %d %d %d", len(p.Templates), len(p.Prompts), len(p.MCP))
	}
	joined := strings.Join(p.Problems, "\n")
	for _, want := range []string{"templates/broken.json", "prompts/leak.json", "prompts/junk.json", `"jira"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems should name %s:\n%s", want, joined)
		}
	}
}

func TestLimits(t *testing.T) {
	dir := teamPack(t)
	defer func(n int) { maxTemplates = n }(maxTemplates)
	maxTemplates = 2
	for i := range 3 {
		write(t, dir, fmt.Sprintf("templates/t%d.json", i), goodTemplate(fmt.Sprintf("t%d", i)))
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Templates) != 2 || !strings.Contains(strings.Join(p.Problems, " "), "at most 2 templates") {
		t.Fatalf("%d %v", len(p.Templates), p.Problems)
	}
	defer func(n int64) { maxBytes = n }(maxBytes)
	maxBytes = 512
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("too big: %v", err)
	}
}

func TestBadManifests(t *testing.T) {
	cases := map[string]any{
		"missing":   nil,
		"bad id":    Manifest{FormatVersion: 1, ID: "Bad_ID", Name: "x"},
		"version 2": Manifest{FormatVersion: 2, ID: "team", Name: "x"},
		"no name":   Manifest{FormatVersion: 1, ID: "team"},
		"bad ref":   Manifest{FormatVersion: 1, ID: "team", Name: "x", Required: []string{"script:x"}},
	}
	for name, m := range cases {
		dir := t.TempDir()
		if m != nil {
			write(t, dir, ManifestFile, m)
		}
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestInitAndExportRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pack")
	if err := Init(dir, "ops", "Ops pack"); err != nil {
		t.Fatal(err)
	}
	if p, err := Load(dir); err != nil || p.Manifest.ID != "ops" {
		t.Fatalf("init: %v %+v", err, p.Manifest)
	}
	out := filepath.Join(t.TempDir(), "export")
	m := Manifest{ID: "ops", Name: "Ops pack", Required: []string{"template:risk-review"}}
	servers := []config.MCPServer{{Name: "docs", URL: "https://docs.example.com/mcp"}}
	err := Export(out, m, []template.Template{goodTemplate("risk-review")},
		[]Prompt{{Name: "triage", Text: "Triage."}}, servers)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Load(out)
	if err != nil || len(p.Templates) != 1 || len(p.Prompts) != 1 || len(p.MCP) != 1 {
		t.Fatalf("%v %+v", err, p)
	}
	if p.Mark("template:risk-review") != MarkRequired {
		t.Fatalf("%v %+v", err, p)
	}
	leaky := []config.MCPServer{{
		Name: "x", URL: "https://x.example.com", Headers: map[string]string{"Authorization": "Bearer abc"},
	}}
	if err := Export(t.TempDir(), m, nil, nil, leaky); err == nil {
		t.Fatal("exporting a literal credential must fail")
	}
}
