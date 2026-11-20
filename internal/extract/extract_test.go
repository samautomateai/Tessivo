package extract

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"a.md": "markdown", "b.TXT": "text", "c.csv": "csv", "d.tsv": "csv", "e.json": "json", "f.go": "code",
		"Dockerfile": "code", "photo.png": "", "noext": "",
	}
	for name, want := range cases {
		if got := KindOf(name); got != want {
			t.Errorf("KindOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFileAndPath(t *testing.T) {
	dir := t.TempDir()
	md := put(t, dir, "notes.md", "# Title\nbody")
	docs, err := Path(md)
	if err != nil || len(docs) != 1 || docs[0].Kind != "markdown" || docs[0].Tokens() != 3 {
		t.Fatalf("%v %+v", err, docs)
	}
	if _, err := File(put(t, dir, "x.png", "png")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want unsupported, got %v", err)
	}
	if _, err := File(put(t, dir, "bin.txt", "a\x00b")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want binary rejected, got %v", err)
	}
	if _, err := Path(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("want missing error")
	}
	if _, err := File(filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("want open error")
	}
	big := put(t, dir, "big.txt", strings.Repeat("a", MaxFileBytes+1))
	if _, err := File(big); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("want size error, got %v", err)
	}
	if _, err := File(put(t, dir, "bad.json", "{")); err == nil {
		t.Fatal("want json error")
	}
}

func TestCSVSmallTableWithStats(t *testing.T) {
	d, err := CSV("sales.csv", []byte("region,amount,note\nnorth,10,ok\nsouth,\"1,000\",\neast,5.5,ok\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 rows, 3 columns", "amount: numeric, min 5.5, max 1000", "sum 1015.5",
		"region: text, 3 distinct", "note: text, 1 distinct values, empty 1, values: ok (2)", "All rows"} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("missing %q in:\n%s", want, d.Text)
		}
	}
}

func TestCSVLargeTableIsSampled(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("id\tname\n")
	for i := 0; i < FullRowsLimit+5; i++ {
		sb.WriteString("1\tx\n")
	}
	d, err := CSV("big.tsv", []byte(sb.String()))
	if err != nil || !strings.Contains(d.Text, "First 20 rows (sample)") {
		t.Fatalf("%v\n%s", err, d.Text)
	}
	if e, _ := CSV("e.csv", nil); e.Text != "(empty table)" {
		t.Fatal("empty table")
	}
	if _, err := CSV("bad.csv", []byte("a,\"b\n")); err == nil {
		t.Fatal("want parse error")
	}
	if got := topValues(map[string]int{}); got != "" {
		t.Fatal("no values")
	}
}

func TestJSONTruncates(t *testing.T) {
	big := `{"k":"` + strings.Repeat("x", MaxJSONChars) + `"}`
	d, err := JSON("big.json", []byte(big))
	if err != nil || !strings.HasSuffix(d.Text, "(truncated)") {
		t.Fatalf("%v", err)
	}
	if d, _ := FromBytes("stdin", "text", []byte("hi")); d.Text != "hi" {
		t.Fatal("text passthrough")
	}
}

func TestTreeRespectsIgnoresAndLimits(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitignore", "# comment\n/secret/\n*.gen.go\n!keep.go\n\n")
	put(t, root, "main.go", "package main")
	put(t, root, "pkg/util.go", "package pkg")
	put(t, root, "pkg/util.gen.go", "package pkg")
	put(t, root, "secret/key.txt", "nope")
	put(t, root, "node_modules/x.js", "nope")
	put(t, root, "image.png", "nope")
	put(t, root, "huge.txt", strings.Repeat("a", 300<<10))
	put(t, root, "README.md", "# hi")

	docs, err := Tree(root, TreeOptions{Exclude: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range docs {
		got = append(got, d.Source)
	}
	if strings.Join(got, ",") != "main.go,pkg/util.go" {
		t.Fatalf("got %v", got)
	}
	all, _ := Tree(root, TreeOptions{IncludeAll: true, MaxFiles: 2})
	if len(all) != 2 {
		t.Fatalf("limit: got %d", len(all))
	}
	viaPath, err := Path(root)
	if err != nil || len(viaPath) != 3 {
		t.Fatalf("Path(dir): %v %d", err, len(viaPath))
	}
	if _, err := Tree(filepath.Join(root, "missing"), TreeOptions{}); err == nil {
		t.Fatal("want walk error")
	}
}
