package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
