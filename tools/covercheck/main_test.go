package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `mode: set
a.go:1.1,2.2 3 1
a.go:3.1,4.2 1 0
a.go:3.1,4.2 1 1
b.go:1.1,2.2 1 0
`

func TestTotal(t *testing.T) {
	got, err := Total(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if got != 80 {
		t.Fatalf("want 80, got %v", got)
	}
	if got, _ := Total(strings.NewReader("mode: set\n")); got != 100 {
		t.Fatalf("empty profile: want 100, got %v", got)
	}
	for _, bad := range []string{"mode: set\nx y\n", "a.go:1 x 1\n", "a.go:1 1 x\n"} {
		if _, err := Total(strings.NewReader(bad)); err == nil {
			t.Fatalf("want error for %q", bad)
		}
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.out")
	if err := os.WriteFile(p, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := run([]string{"-profile", p, "-min", "80"}, &buf); code != 0 {
		t.Fatalf("want 0, got %d: %s", code, buf.String())
	}
	if code := run([]string{"-profile", p, "-min", "90"}, &buf); code != 1 {
		t.Fatalf("want 1, got %d", code)
	}
	if code := run([]string{"-profile", filepath.Join(dir, "none")}, &buf); code != 2 {
		t.Fatalf("want 2, got %d", code)
	}
	if code := run([]string{"-bogus"}, &buf); code != 2 {
		t.Fatalf("want 2, got %d", code)
	}
	bad := filepath.Join(dir, "bad.out")
	_ = os.WriteFile(bad, []byte("mode: set\nnope\n"), 0o600)
	if code := run([]string{"-profile", bad}, &buf); code != 2 {
		t.Fatalf("want 2, got %d", code)
	}
}
