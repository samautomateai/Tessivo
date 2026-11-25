// Package worktype defines Tessivo's built-in work types — Analyze, Summarize,
// Document and Report — each with a starter prompt, suggested context and the
// JSON schema its output must follow.
package worktype

import (
	"embed"
	"fmt"
	"sort"

	"github.com/samautomateai/tessivo/internal/schema"
)

// IDs of the built-in work types.
const (
	Analyze   = "analyze"
	Summarize = "summarize"
	Document  = "document"
	Report    = "report"
)

// WorkType is a recipe for one kind of work.
type WorkType struct {
	ID               string
	Name             string
	Question         string // the question it answers
	StarterPrompt    string
	SuggestedContext []string
	SystemPrompt     string
	Schema           []byte // JSON schema of the structured output
}

// ParsedSchema returns the output schema ready for validation.
func (w WorkType) ParsedSchema() (*schema.Schema, error) {
	s, err := schema.Parse(w.Schema)
	if err != nil {
		return nil, fmt.Errorf("work type %s: %w", w.ID, err)
	}
	return s, nil
}

//go:embed schemas/*.json
var schemaFS embed.FS

func mustSchema(id string) []byte {
	b, err := schemaFS.ReadFile("schemas/" + id + ".json")
	if err != nil {
		panic(fmt.Sprintf("worktype: missing embedded schema %s: %v", id, err))
	}
	return b
}

const systemBase = "You are Tessivo, a careful analyst. Use only the provided context. " +
	"Cite file names or sources as evidence. If something is unknown, say so instead of guessing. " +
	"Reply with JSON only, matching the requested schema."

var builtins = map[string]WorkType{
	Analyze: {
		ID: Analyze, Name: "Analyze", Question: "What does this mean, and what's the risk?",
		StarterPrompt: "Assess the risk and impact described by the context. Explain your reasoning with evidence, " +
			"and list each problem as a finding a team could file as an issue.",
		SuggestedContext: []string{"code base", "diff or PR", "documents", "spreadsheet or CSV", "MCP server"},
	},
	Summarize: {
		ID: Summarize, Name: "Summarize", Question: "What do I need to know?",
		StarterPrompt:    "Summarize these sources for a busy reader. Keep what matters; drop the rest.",
		SuggestedContext: []string{"documents", "transcripts", "threads", "URLs"},
	},
	Document: {
		ID: Document, Name: "Document", Question: "How does this work?",
		StarterPrompt:    "Explain what this is and how it fits together, for a new team member.",
		SuggestedContext: []string{"code base", "existing docs", "notes"},
	},
	Report: {
		ID: Report, Name: "Report", Question: "What happened, and what's next?",
		StarterPrompt:    "Produce this period's report: highlights, issues, trends versus last period, next steps.",
		SuggestedContext: []string{"logs", "tickets CSV", "metrics spreadsheet", "previous report"},
	},
}

func init() {
	for id, w := range builtins {
		w.SystemPrompt = systemBase
		w.Schema = mustSchema(id)
		builtins[id] = w
	}
}

// Get returns a built-in work type by ID.
func Get(id string) (WorkType, error) {
	w, ok := builtins[id]
	if !ok {
		return WorkType{}, fmt.Errorf("unknown work type %q (want analyze, summarize, document or report)", id)
	}
	return w, nil
}

// All returns the built-in work types sorted by ID.
func All() []WorkType {
	out := make([]WorkType, 0, len(builtins))
	for _, w := range builtins {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
