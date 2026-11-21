package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (string, *Registry) {
	t.Helper()
	root := t.TempDir()
	r, err := NewRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, NewRegistry(CodeTools(r)...)
}

func run(t *testing.T, reg *Registry, name string, args any) (string, error) {
	t.Helper()
	tool, ok := reg.Get(name)
	if !ok {
		t.Fatalf("tool %s missing", name)
	}
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return tool.Run(context.Background(), b)
}

func TestRegistryDefs(t *testing.T) {
	_, reg := setup(t)
	names := map[string]bool{}
	for _, d := range reg.Defs() {
		names[d.Name] = true
		var schema map[string]any
		if err := json.Unmarshal(d.Parameters, &schema); err != nil || schema["type"] != "object" {
			t.Errorf("%s: parameters must be an object schema: %s", d.Name, d.Parameters)
		}
		if d.Description == "" {
			t.Errorf("%s: missing description", d.Name)
		}
	}
	for _, n := range []string{"read_file", "list_directory", "glob", "grep"} {
		if !names[n] {
			t.Errorf("missing tool %s", n)
		}
	}
	if _, ok := reg.Get("nope"); ok {
		t.Fatal("unknown tool found")
	}
}

func TestReadFileRange(t *testing.T) {
	root, reg := setup(t)
	mkfile(t, filepath.Join(root, "f.txt"), "one\ntwo\nthree\nfour\nfive\n")
	out, err := run(t, reg, "read_file", map[string]any{"path": "f.txt", "start_line": 2, "end_line": 3})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2\ttwo") || !strings.Contains(out, "3\tthree") || strings.Contains(out, "four") ||
		strings.Contains(out, "one") {
		t.Fatalf("range: %q", out)
	}
	mkfile(t, filepath.Join(root, "big.txt"), strings.Repeat(strings.Repeat("x", 99)+"\n", 3000))
	out, err = run(t, reg, "read_file", map[string]any{"path": "big.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxRead+200 || !strings.Contains(out, "truncated") {
		t.Fatalf("big file: %d bytes, truncated note: %v", len(out), strings.Contains(out, "truncated"))
	}
	mkfile(t, filepath.Join(root, "bin.dat"), "ab\x00cd")
	if out, err = run(t, reg, "read_file", map[string]any{"path": "bin.dat"}); err == nil {
		t.Fatalf("binary file must be refused, got %q", out)
	}
	if _, err = run(t, reg, "read_file", map[string]any{"path": "."}); err == nil {
		t.Fatal("a directory must be refused")
	}
}

func TestListAndGlobBounds(t *testing.T) {
	root, reg := setup(t)
	for i := range 2500 {
		mkfile(t, filepath.Join(root, "many", fmt.Sprintf("f%04d.txt", i)), "x")
	}
	out, err := run(t, reg, "list_directory", map[string]any{"path": "many"})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, ".txt"); n != maxEntries || !strings.Contains(out, "more entries not shown") {
		t.Fatalf("list: %d entries, note: %v", n, strings.Contains(out, "more entries not shown"))
	}
	mkfile(t, filepath.Join(root, "src", "a", "b", "deep.go"), "package b")
	mkfile(t, filepath.Join(root, "src", "top.go"), "package src")
	mkfile(t, filepath.Join(root, "node_modules", "dep", "x.go"), "package x")
	out, err = run(t, reg, "glob", map[string]any{"pattern": "**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "src/a/b/deep.go") || !strings.Contains(out, "src/top.go") ||
		strings.Contains(out, "node_modules") {
		t.Fatalf("glob: %q", out)
	}
	out, err = run(t, reg, "list_directory", map[string]any{"path": "src", "depth": 3})
	if err != nil || !strings.Contains(out, "a/b/deep.go") || !strings.Contains(out, "a/") {
		t.Fatalf("deep list: %q %v", out, err)
	}
}

func TestGrep(t *testing.T) {
	root, reg := setup(t)
	mkfile(t, filepath.Join(root, "a.go"), "package a\nfunc Hello() {}\n")
	mkfile(t, filepath.Join(root, "b.md"), "hello world\n")
	out, err := run(t, reg, "grep", map[string]any{"pattern": "hello", "ignore_case": true, "glob": "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go:2: func Hello() {}") || strings.Contains(out, "b.md") {
		t.Fatalf("grep: %q", out)
	}
	mkfile(t, filepath.Join(root, "many.txt"), strings.Repeat("match\n", 600))
	out, err = run(t, reg, "grep", map[string]any{"pattern": "^match$", "path": "many.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, ": match"); n != maxMatches || !strings.Contains(out, "limit reached") {
		t.Fatalf("grep limit: %d matches", n)
	}
	if _, err = run(t, reg, "grep", map[string]any{"pattern": "("}); err == nil {
		t.Fatal("invalid regex must be an error")
	}
}

func TestToolBadArgs(t *testing.T) {
	_, reg := setup(t)
	for _, name := range []string{"read_file", "list_directory", "glob", "grep"} {
		tool, _ := reg.Get(name)
		_, err := tool.Run(context.Background(), json.RawMessage(`{"path":`))
		if err == nil || !strings.Contains(err.Error(), "invalid arguments") {
			t.Errorf("%s: want invalid arguments, got %v", name, err)
		}
	}
	if _, err := run(t, reg, "read_file", map[string]any{"path": "../escape"}); err == nil ||
		!strings.Contains(err.Error(), "outside") {
		t.Fatalf("escape through a tool: %v", err)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"**/*.go", "a.go", true},
		{"**/*.go", "x/y/a.go", true},
		{"src/**", "src/a/b", true},
		{"src/*.go", "src/a/b.go", false},
		{"*.md", "docs/a.md", false},
		{"docs/**/*.md", "docs/a.md", true},
		{"[", "x", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.name); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}
