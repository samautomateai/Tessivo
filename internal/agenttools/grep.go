package agenttools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samautomateai/tessivo/internal/llm"
)

// maxGrepFile skips files too large to be source or documents.
const maxGrepFile = 16 << 20

type grepTool struct{ r *Roots }

func (grepTool) Def() llm.ToolDef {
	return def("grep", "Search file contents inside the run's folders with a regular expression (RE2). "+
		"Returns path:line: text. glob limits which files are searched, e.g. *.go.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},`+
			`"glob":{"type":"string"},"ignore_case":{"type":"boolean"}},"required":["pattern"]}`)
}

type grepArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path"`
	Glob       string `json:"glob"`
	IgnoreCase bool   `json:"ignore_case"`
}

func (t grepTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a grepArgs
	if err := decode(args, &a); err != nil {
		return "", err
	}
	expr := a.Pattern
	if a.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}
	base, err := t.r.Resolve(a.Path)
	if err != nil {
		return "", err
	}
	g := &grepRun{re: re, args: a}
	info, err := os.Stat(base)
	if err != nil {
		return "", fmt.Errorf("grep %s: %w", a.Path, err)
	}
	if !info.IsDir() {
		g.file(base, filepath.Base(base), true)
		return g.sb.String(), nil
	}
	err = walk(ctx, base, func(rel string, d fs.DirEntry) (bool, error) {
		// Symlinks are skipped: reading through one could leave the run's folders.
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 ||
			(a.Glob != "" && !matchGlob(a.Glob, rel) && !matchGlob(a.Glob, d.Name())) {
			return true, nil
		}
		return g.file(filepath.Join(base, filepath.FromSlash(rel)), display(a.Path, rel), false), nil
	})
	if err != nil {
		return "", fmt.Errorf("grep %s: %w", a.Pattern, err)
	}
	return g.sb.String(), nil
}

type grepRun struct {
	re   *regexp.Regexp
	args grepArgs
	sb   strings.Builder
	n    int
}

// file searches one file and reports whether to keep going. A single file
// named directly is shown by its own name.
func (g *grepRun) file(p, shown string, direct bool) bool {
	if direct {
		shown = g.args.Path
	}
	info, err := os.Stat(p)
	if err != nil || info.Size() > maxGrepFile {
		return true
	}
	data, err := os.ReadFile(p) // #nosec G304 -- p is under a root resolved by Roots
	if err != nil || bytes.IndexByte(data[:min(len(data), 8<<10)], 0) >= 0 {
		return true
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if !g.re.MatchString(text) {
			continue
		}
		if g.n == maxMatches {
			g.sb.WriteString("[limit reached: 500 matches; narrow the pattern, path or glob]\n")
			return false
		}
		g.n++
		if len(text) > 500 {
			text = text[:500] + "…"
		}
		fmt.Fprintf(&g.sb, "%s:%d: %s\n", shown, line, text)
	}
	return true
}
