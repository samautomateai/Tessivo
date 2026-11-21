package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/samautomateai/tessivo/internal/llm"
)

const moreNote = "[more entries not shown; narrow the path or pattern]\n"

type listDirectory struct{ r *Roots }

func (listDirectory) Def() llm.ToolDef {
	return def("list_directory", "List files and folders (folders end in /) inside the run's folders. "+
		"depth 1 lists direct children; up to 5.",
		`{"type":"object","properties":{"path":{"type":"string"},`+
			`"depth":{"type":"integer","minimum":1,"maximum":5}}}`)
}

func (t listDirectory) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	depth := min(max(a.Depth, 1), 5)
	base, err := t.r.Resolve(a.Path)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	n := 0
	err = walk(ctx, base, func(rel string, d fs.DirEntry) (bool, error) {
		if strings.Count(rel, "/") >= depth {
			if d.IsDir() {
				return true, fs.SkipDir
			}
			return true, nil
		}
		if n == maxEntries {
			sb.WriteString(moreNote)
			return false, nil
		}
		n++
		sb.WriteString(rel)
		if d.IsDir() {
			sb.WriteString("/")
		}
		sb.WriteString("\n")
		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("list %s: %w", a.Path, err)
	}
	return sb.String(), nil
}

type globTool struct{ r *Roots }

func (globTool) Def() llm.ToolDef {
	return def("glob", "Find files by pattern inside the run's folders. Patterns use / and ** matches any "+
		"number of folders, e.g. **/*.go.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},`+
			`"required":["pattern"]}`)
}

func (t globTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	base, err := t.r.Resolve(a.Path)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	n := 0
	err = walk(ctx, base, func(rel string, d fs.DirEntry) (bool, error) {
		if d.IsDir() || !matchGlob(a.Pattern, rel) {
			return true, nil
		}
		if n == maxEntries {
			sb.WriteString(moreNote)
			return false, nil
		}
		n++
		sb.WriteString(display(a.Path, rel) + "\n")
		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("glob %s: %w", a.Pattern, err)
	}
	return sb.String(), nil
}

// matchGlob matches a slash-separated name against a pattern in which a "**"
// segment matches zero or more segments and other segments use path.Match.
func matchGlob(pattern, name string) bool {
	return matchSegs(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if matchSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
