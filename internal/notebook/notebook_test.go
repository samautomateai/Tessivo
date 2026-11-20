package notebook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/worktype"
)

func TestCreateOpenAndContext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "risk")
	nb, err := Create(dir, "", worktype.Analyze)
	if err != nil {
		t.Fatal(err)
	}
	if nb.Manifest.Name != "risk" {
		t.Fatalf("name: %q", nb.Manifest.Name)
	}
	if _, err := Create(dir, "again", worktype.Analyze); err == nil {
		t.Fatal("want already-exists error")
	}
	src := filepath.Join(t.TempDir(), "my notes (v2).md")
	_ = os.WriteFile(src, []byte("hello"), 0o600)
	item, err := nb.AddContext(src)
	if err != nil {
		t.Fatal(err)
	}
	if item.InPlace || !strings.HasPrefix(item.Path, ContextDir) || strings.ContainsAny(item.Path, " ()") {
		t.Fatalf("copied item: %+v", item)
	}
	if b, _ := os.ReadFile(nb.ContextPath(item)); string(b) != "hello" {
		t.Fatal("file not copied")
	}
	code := t.TempDir()
	ref, err := nb.AddContext(code)
	if err != nil || !ref.InPlace || nb.ContextPath(ref) != code {
		t.Fatalf("reference: %v %+v", err, ref)
	}
	if err := nb.SetEnabled(ref.Label, false); err != nil {
		t.Fatal(err)
	}
	if err := nb.SetEnabled("nope", false); err == nil {
		t.Fatal("want missing item error")
	}
	re, err := Open(dir)
	if err != nil || len(re.Manifest.Context) != 2 || re.Manifest.Context[1].Enabled {
		t.Fatalf("reopen: %v %+v", err, re.Manifest)
	}
	if _, err := nb.AddContext(filepath.Join(code, "missing")); err == nil {
		t.Fatal("want missing context error")
	}
}

func TestCreateAndOpenErrors(t *testing.T) {
	if _, err := Create(t.TempDir(), "x", "poetry"); err == nil {
		t.Fatal("want bad work type")
	}
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("want missing manifest")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{"workType":"poetry"}`), 0o600)
	if _, err := Open(dir); err == nil {
		t.Fatal("want bad work type on open")
	}
	_ = os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{`), 0o600)
	if _, err := Open(dir); err == nil {
		t.Fatal("want parse error")
	}
	nb, _ := Create(filepath.Join(t.TempDir(), "n"), "n", worktype.Report)
	nb.Manifest.Settings.Mode = "warp"
	if err := nb.Save(); err == nil {
		t.Fatal("want invalid settings")
	}
	if _, err := Create("/proc/tessivo-nope", "x", worktype.Report); err == nil {
		t.Fatal("want mkdir error")
	}
}

func TestPromptsRunsOutputs(t *testing.T) {
	nb, err := Create(filepath.Join(t.TempDir(), "n"), "n", worktype.Summarize)
	if err != nil {
		t.Fatal(err)
	}
	if err := nb.SavePrompt(Prompt{Name: "weekly", Text: "Summarize {{week}}"}); err != nil {
		t.Fatal(err)
	}
	if err := nb.SavePrompt(Prompt{Name: "../evil", Text: "x"}); err == nil {
		t.Fatal("want bad name")
	}
	if err := nb.SavePrompt(Prompt{Name: "bad", Settings: config.RunSettings{Mode: "x"}}); err == nil {
		t.Fatal("want bad settings")
	}
	p, err := nb.LoadPrompt("weekly")
	if err != nil || p.Text != "Summarize {{week}}" {
		t.Fatalf("%v %+v", err, p)
	}
	if _, err := nb.LoadPrompt("../x"); err == nil {
		t.Fatal("want bad name on load")
	}
	names, _ := nb.ListPrompts()
	if len(names) != 1 || names[0] != "weekly" {
		t.Fatalf("names %v", names)
	}
	if err := nb.SaveRun(Run{ID: "r1", Valid: true}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"o1", "o2"} {
		o := Output{ID: id, RunID: "r1", Title: id, Data: json.RawMessage(`{"a":1}`)}
		if err := nb.SaveOutput(o, "# "+id); err != nil {
			t.Fatal(err)
		}
	}
	_ = nb.SaveOutput(Output{ID: "o0", Data: json.RawMessage(`{}`)}, "")
	outs, err := nb.ListOutputs()
	if err != nil || len(outs) != 3 || outs[0].ID != "o2" {
		t.Fatalf("outputs: %v %+v", err, outs)
	}
	if _, err := os.Stat(filepath.Join(nb.Dir, OutputsDir, "o1.md")); err != nil {
		t.Fatal("markdown not written")
	}
	_ = os.WriteFile(filepath.Join(nb.Dir, OutputsDir, "zz.json"), []byte("{"), 0o600)
	if _, err := nb.ListOutputs(); err == nil {
		t.Fatal("want parse error")
	}
	_ = os.RemoveAll(filepath.Join(nb.Dir, PromptsDir))
	if _, err := nb.ListPrompts(); err == nil {
		t.Fatal("want list error")
	}
}

func TestSanitizeAndIDs(t *testing.T) {
	if sanitize("ok-name.md") != "ok-name.md" {
		t.Fatal("safe names unchanged")
	}
	long := sanitize(strings.Repeat("é", 100) + ".md")
	if len([]rune(long)) != 64 || !strings.HasSuffix(long, ".md") {
		t.Fatalf("long: %q", long)
	}
	if a, b := NewID(), NewID(); a == b || len(a) != 22 {
		t.Fatalf("ids %q %q", a, b)
	}
	if err := writeJSON("/proc/nope/x.json", 1); err == nil {
		t.Fatal("want write error")
	}
	if err := writeJSON(filepath.Join(t.TempDir(), "x.json"), func() {}); err == nil {
		t.Fatal("want encode error")
	}
}
