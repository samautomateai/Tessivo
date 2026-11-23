package research

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestOpenRefusesFilePathsOutsideSources(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "thesis.docx")
	if err := os.WriteFile(outside, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "a.md")
	_ = os.WriteFile(src, []byte("a"), 0o600)
	dir := filepath.Join(root, "nb")
	nb, err := Create(dir, "nb", "Nb", "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := nb.AddPath(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{filepath.Join("..", "thesis.docx"), outside, filepath.Join("sources", "other-x")} {
		nb.Manifest.Sources[0].Path = bad
		if cur, err := Open(dir); err == nil { // this test overwrites the manifest on purpose
			nb.Manifest.Revision = cur.Manifest.Revision
		}
		if err := nb.Save(); err != nil {
			t.Fatal(err)
		}
		opened, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if p := opened.SourcePath(opened.Manifest.Sources[0]); p != "" {
			t.Fatalf("%q: source path %q, want none", bad, p)
		}
		if err := opened.RemoveSource(s.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("%q: removing the source deleted a file outside the notebook: %v", bad, err)
		}
		nb.Manifest.Sources = []Source{s}
	}
}

func TestCitationsInsideCodeAreNotCitations(t *testing.T) {
	items := []Item{{N: 1, Kind: ItemSource, ID: "s1", Label: "Design doc"}}
	code := "Use `items[1]` like this:\n\n```go\nx := a[1]\n```\n"
	if got := ParseCitations(code, items); len(got) != 0 {
		t.Fatalf("code read as citations: %+v", got)
	}
	text := code + "It is documented [1]."
	cites := ParseCitations(text, items)
	if len(cites) != 1 {
		t.Fatalf("cites %+v", cites)
	}
	got := Relabel(text, cites)
	if !strings.Contains(got, "`items[1]`") || !strings.Contains(got, "x := a[1]") ||
		!strings.Contains(got, "documented [Design doc].") {
		t.Fatalf("relabel touched code: %q", got)
	}
}

func TestResearchFormatVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nb")
	if _, err := Create(dir, "nb", "Nb", ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ManifestFile)
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"formatVersion": 1`) {
		t.Fatalf("new research notebooks record their format: %s", b)
	}
	_ = os.WriteFile(path, []byte(strings.Replace(string(b), `"formatVersion": 1,`, "", 1)), 0o600)
	if nb, err := Open(dir); err != nil || nb.Manifest.FormatVersion != FormatVersion {
		t.Fatalf("legacy: %v", err)
	}
	_ = os.WriteFile(path, []byte(strings.Replace(string(b), `"formatVersion": 1`, `"formatVersion": 7`, 1)), 0o600)
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Fatalf("newer: %v", err)
	}
}

func TestTwoHandlesBothAddSources(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nb")
	a, err := Create(dir, "nb", "Nb", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(dir) // the CLI and the app each hold a copy
	if err != nil {
		t.Fatal(err)
	}
	for i, h := range []*Notebook{a, b} {
		src := filepath.Join(root, string(rune('a'+i))+".md")
		_ = os.WriteFile(src, []byte("x"), 0o600)
		if _, err := h.AddPath(src); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Open(dir)
	if err != nil || len(got.Manifest.Sources) != 2 {
		t.Fatalf("both sources must be kept: %v %d", err, len(got.Manifest.Sources))
	}
	a.Manifest.Name = "stale"
	if err := a.Save(); !errors.Is(err, notebook.ErrChanged) {
		t.Fatalf("a stale save must be refused: %v", err)
	}
}
