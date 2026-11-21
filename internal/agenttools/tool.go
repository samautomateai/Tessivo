package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/samautomateai/tessivo/internal/llm"
)

// Tool is one capability the agent can call.
type Tool interface {
	Def() llm.ToolDef
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry is the set of tools offered to the model, in a stable order.
type Registry struct {
	order []string
	tools map[string]Tool
}

// NewRegistry indexes tools by name; a later tool with the same name wins.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range tools {
		name := t.Def().Name
		if _, dup := r.tools[name]; !dup {
			r.order = append(r.order, name)
		}
		r.tools[name] = t
	}
	return r
}

// Defs returns every tool's definition.
func (r *Registry) Defs() []llm.ToolDef {
	out := make([]llm.ToolDef, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.tools[n].Def())
	}
	return out
}

// Get finds a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// CodeTools returns the read-only tools rooted in r.
func CodeTools(r *Roots) []Tool {
	return []Tool{readFile{r}, listDirectory{r}, globTool{r}, grepTool{r}}
}

// Bounds on what one tool call returns.
const (
	maxRead    = 256 << 10
	maxEntries = 2000
	maxMatches = 500
	maxLine    = 1 << 20
)

// skipDirs are never walked: version control, dependencies and build output.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "build": true, "dist": true, "target": true, "vendor": true,
	".dart_tool": true,
}

func decode(args json.RawMessage, v any) error {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if err := json.Unmarshal(args, v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func def(name, desc, schema string) llm.ToolDef {
	return llm.ToolDef{Name: name, Description: desc, Parameters: json.RawMessage(schema)}
}

// walk visits files and folders under base, skipping skipDirs, and passes
// each entry's slash-separated path relative to base.
func walk(ctx context.Context, base string, visit func(rel string, d fs.DirEntry) (bool, error)) error {
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("stopped: %w", ctxErr)
		}
		if p == base {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		rel, relErr := filepath.Rel(base, p)
		if relErr != nil {
			return nil //nolint:nilerr // cannot happen for paths under base; skip defensively
		}
		more, vErr := visit(filepath.ToSlash(rel), d)
		if vErr != nil {
			return vErr
		}
		if !more {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}
	return nil
}

// display joins the caller's base argument and a relative path for output,
// so results can be passed back to other tools as they are.
func display(base, rel string) string {
	if base == "" || base == "." {
		return rel
	}
	return filepath.ToSlash(filepath.Join(base, rel))
}
