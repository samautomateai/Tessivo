package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name, in      string
		crlf, keepTWS bool
		want          []string
	}{
		{name: "clean", in: "a\nb\n"},
		{name: "empty", in: ""},
		{name: "crlf", in: "a\r\nb\r\n", want: []string{"2 line(s) do not end with LF"}},
		{name: "bom", in: "\xEF\xBB\xBFa\n", want: []string{"byte-order mark"}},
		{name: "invalid", in: "a\xff\n", want: []string{"not valid UTF-8"}},
		{name: "no final newline", in: "a", want: []string{"does not end with a newline"}},
		{name: "trailing", in: "a \nb\t\n", want: []string{"2 line(s) have trailing whitespace"}},
		{name: "markdown keeps trailing", in: "a  \n", keepTWS: true},
		{name: "bat wants crlf", in: "a\r\nb\n", crlf: true, want: []string{"1 line(s) do not end with CRLF"}},
		{name: "bat clean", in: "a\r\n", crlf: true},
	}
	for _, c := range cases {
		got := strings.Join(Check([]byte(c.in), c.crlf, c.keepTWS), "|")
		if len(c.want) == 0 && got != "" {
			t.Errorf("%s: unexpected %q", c.name, got)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: want %q in %q", c.name, w, got)
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want      string
		crlf, keepTWS bool
	}{
		{in: "\xEF\xBB\xBFa \r\nb\t\r\nc", want: "a\nb\nc\n"},
		{in: "a\nb\n", want: "a\r\nb\r\n", crlf: true},
		{in: "a  \n", want: "a  \n", keepTWS: true},
		{in: "", want: ""},
		{in: "\xEF\xBB\xBF", want: ""},
		{in: "a\n\n\n", want: "a\n\n\n"},
	}
	for _, c := range cases {
		if got := string(Normalize([]byte(c.in), c.crlf, c.keepTWS)); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunReportsAndFixes(t *testing.T) {
	dir := t.TempDir()
	goFile := write(t, dir, "a.go", "package a\r\n")
	md := write(t, dir, "docs/r.md", "line  \nend")
	bat := write(t, dir, "x.bat", "echo\n")
	bad := write(t, dir, "bad.json", "\xff\n")
	write(t, dir, "ok.yml", "a: 1\n")
	write(t, dir, "image.png", "\r\n\xff")
	write(t, dir, "build/gen.go", "x\r\n")
	write(t, dir, ".gitignore", "bin/\n")

	var out bytes.Buffer
	if code := run([]string{dir}, &out); code != 1 {
		t.Fatalf("want 1, got %d\n%s", code, &out)
	}
	wants := []string{"a.go: 1 line(s) do not end with LF", "r.md: does not end", "x.bat: 1 line(s)", "bad.json"}
	for _, want := range wants {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in\n%s", want, &out)
		}
	}
	if strings.Contains(out.String(), "png") || strings.Contains(out.String(), "gen.go") {
		t.Errorf("checked a skipped file:\n%s", &out)
	}

	out.Reset()
	if code := run([]string{"-fix", dir}, &out); code != 1 || !strings.Contains(out.String(), "bad.json") {
		t.Fatalf("invalid UTF-8 must remain: %d\n%s", code, &out)
	}
	for p, want := range map[string]string{goFile: "package a\n", md: "line  \nend\n", bat: "echo\r\n"} {
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{dir}, &out); code != 0 {
		t.Fatalf("after fix: %d\n%s", code, &out)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 7 {
		t.Errorf("temporary files left: %v", entries)
	}
}

func TestRunErrors(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"-nope"}, &out); code != 2 {
		t.Errorf("bad flag: %d", code)
	}
	if code := run([]string{filepath.Join(t.TempDir(), "missing")}, &out); code != 2 {
		t.Errorf("missing dir: %d", code)
	}
}

func TestRunDefaultDirAndLargeFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "big.txt", strings.Repeat("x\r\n", maxSize/3+1))
	t.Chdir(dir)
	var out bytes.Buffer
	if code := run(nil, &out); code != 0 {
		t.Fatalf("large file must be skipped: %d\n%s", code, &out)
	}
}

func TestWriteAtomicErrors(t *testing.T) {
	if err := writeAtomic(filepath.Join(t.TempDir(), "missing", "f"), nil, 0o600); err == nil {
		t.Error("write into a missing directory succeeded")
	}
	if _, err := checkFile(filepath.Join(t.TempDir(), "missing.go"), false); err == nil {
		t.Error("missing file accepted")
	}
}

func TestIsText(t *testing.T) {
	for name, want := range map[string]bool{
		"a.go": true, "A.MD": true, "pre-commit": true, ".gitattributes": true, "x.png": false, "coverage.out": false,
	} {
		if isText(name) != want {
			t.Errorf("isText(%q) != %v", name, want)
		}
	}
}
