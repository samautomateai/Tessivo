// Command lencheck enforces maximum file and line lengths across the repository.
//
// Usage:
//
//	lencheck [-max-line 120] [-max-file 500] [-max-test-file 800] [dir ...]
//
// Generated files (containing "Code generated" or "GENERATED CODE") and
// vendored or build directories are skipped. It exits non-zero on violations.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Limits holds the enforced maxima.
type Limits struct {
	MaxLine     int
	MaxFile     int
	MaxTestFile int
}

// Violation describes one breach of the limits.
type Violation struct {
	Path    string
	Line    int
	Message string
}

func (v Violation) String() string {
	if v.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", v.Path, v.Line, v.Message)
	}
	return fmt.Sprintf("%s: %s", v.Path, v.Message)
}

var checkedExt = map[string]bool{".go": true, ".dart": true, ".proto": true, ".yml": true, ".yaml": true}

var skippedDirs = map[string]bool{
	".git": true, "vendor": true, "build": true, ".dart_tool": true, "node_modules": true,
	"gen": true, "ephemeral": true,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	fsFlags := flag.NewFlagSet("lencheck", flag.ContinueOnError)
	fsFlags.SetOutput(out)
	lim := Limits{}
	fsFlags.IntVar(&lim.MaxLine, "max-line", 120, "maximum line length in characters")
	fsFlags.IntVar(&lim.MaxFile, "max-file", 500, "maximum lines per source file")
	fsFlags.IntVar(&lim.MaxTestFile, "max-test-file", 800, "maximum lines per test file")
	if err := fsFlags.Parse(args); err != nil {
		return 2
	}
	dirs := fsFlags.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	var all []Violation
	for _, d := range dirs {
		v, err := CheckTree(d, lim)
		if err != nil {
			_, _ = fmt.Fprintln(out, "lencheck:", err)
			return 2
		}
		all = append(all, v...)
	}
	for _, v := range all {
		_, _ = fmt.Fprintln(out, v)
	}
	if len(all) > 0 {
		_, _ = fmt.Fprintf(out, "lencheck: %d violation(s)\n", len(all))
		return 1
	}
	return 0
}

// CheckTree walks root and returns all violations.
func CheckTree(root string, lim Limits) ([]Violation, error) {
	var out []Violation
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !checkedExt[filepath.Ext(path)] {
			return nil
		}
		v, cerr := CheckFile(path, lim)
		if cerr != nil {
			return cerr
		}
		out = append(out, v...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return out, nil
}

// CheckFile checks a single file against the limits.
func CheckFile(path string, lim Limits) ([]Violation, error) {
	f, err := os.Open(path) //nolint:gosec // G304: paths come from walking the repo tree
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return checkReader(path, f, lim)
}

func checkReader(path string, r io.Reader, lim Limits) ([]Violation, error) {
	var out []Violation
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if n <= 5 && (strings.Contains(line, "Code generated") || strings.Contains(line, "GENERATED CODE")) {
			return nil, nil
		}
		if l := utf8.RuneCountInString(strings.ReplaceAll(line, "\t", "    ")); l > lim.MaxLine {
			out = append(out, Violation{path, n, fmt.Sprintf("line is %d characters (max %d)", l, lim.MaxLine)})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	maxFile := lim.MaxFile
	if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_test.dart") {
		maxFile = lim.MaxTestFile
	}
	if n > maxFile {
		out = append(out, Violation{path, 0, fmt.Sprintf("file has %d lines (max %d)", n, maxFile)})
	}
	return out, nil
}
