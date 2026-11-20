// Package executor runs a work type against a model. The Executor interface
// is engine-agnostic: Pipeline (deterministic, our own code) implements it now
// and the Eino-based agent mode will implement it behind the same interface.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// Model is what an executor needs from the LLM layer; *llm.Router satisfies it.
type Model interface {
	ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error)
}

// Spec is everything a run needs.
type Spec struct {
	WorkType  worktype.WorkType
	Prompt    string
	Vars      map[string]string
	Documents []extract.Document
	Memory    []string // pinned notebook facts
	Settings  config.Effective
}

// Event reports progress to the caller (UI, CLI).
type Event struct {
	Kind string // "status" or "delta"
	Text string
}

// Event kinds.
const (
	EventStatus = "status"
	EventDelta  = "delta"
)

// Strategies a run can use to fit context into the model.
const (
	StrategyFull      = "full"
	StrategyMapReduce = "map-reduce"
)

// Result is a finished run.
type Result struct {
	Data        json.RawMessage // validated JSON (or the raw reply when invalid)
	Valid       bool
	Model       string // label of the model that produced the final answer
	Attempts    []llm.Attempt
	Strategy    string
	TotalTokens int
	Validation  string // last validation error, if any
}

// Executor runs a spec.
type Executor interface {
	Run(ctx context.Context, spec Spec, onEvent func(Event)) (Result, error)
}

var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

// Fill replaces {{name}} placeholders with vars and fails on any left unset.
func Fill(prompt string, vars map[string]string) (string, error) {
	var missing []string
	out := varPattern.ReplaceAllStringFunc(prompt, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return v
		}
		missing = append(missing, name)
		return m
	})
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", fmt.Errorf("prompt variables not set: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// Variables lists the distinct {{variables}} a prompt uses, in order.
func Variables(prompt string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range varPattern.FindAllStringSubmatch(prompt, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func formatContext(docs []extract.Document) string {
	var sb strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&sb, "<source path=%q kind=%q>\n%s\n</source>\n\n", d.Source, d.Kind, strings.TrimSpace(d.Text))
	}
	return sb.String()
}

func contextTokens(docs []extract.Document) int {
	n := 0
	for _, d := range docs {
		n += d.Tokens()
	}
	return n
}
