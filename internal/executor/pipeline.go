package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/schema"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// DefaultContextBudget applies when settings do not set one (tokens).
const DefaultContextBudget = 100_000

// Pipeline is the deterministic executor: fit context (full or map-reduce),
// make one structured-output call, validate, and retry with the validation
// errors fed back.
type Pipeline struct {
	Model                Model
	MaxValidationRetries int // default 2; set -1 for none
}

// Run executes the spec.
func (p *Pipeline) Run(ctx context.Context, spec Spec, onEvent func(Event)) (Result, error) {
	emit := func(kind, text string) {
		if onEvent != nil {
			onEvent(Event{Kind: kind, Text: text})
		}
	}
	prompt, err := Fill(spec.Prompt, spec.Vars)
	if err != nil {
		return Result{}, err
	}
	sch, err := spec.WorkType.ParsedSchema()
	if err != nil {
		return Result{}, err
	}
	res := Result{Strategy: StrategyFull}
	docs := spec.Documents
	budget := DefaultContextBudget
	if spec.Settings.ContextBudget != nil {
		budget = *spec.Settings.ContextBudget
	}
	budget = reserveForRevision(budget, spec.Revise)
	if contextTokens(docs) > budget {
		res.Strategy = StrategyMapReduce
		emit(EventStatus, fmt.Sprintf("context is larger than %d tokens: condensing %d sources", budget, len(docs)))
		if docs, err = p.mapStep(ctx, spec, prompt, docs, budget, &res); err != nil {
			return res, err
		}
	}
	emit(EventStatus, "generating "+spec.WorkType.Name)
	return p.reduceStep(ctx, spec, prompt, docs, sch, res, func(s string) { emit(EventDelta, s) })
}

// NewRequest builds a model request from the run's effective settings.
func NewRequest(spec Spec, msgs []llm.Message, format *llm.ResponseFormat) llm.Request {
	s := spec.Settings
	return llm.Request{
		Messages: msgs, Temperature: s.Temperature, MaxTokens: s.MaxOutputTokens,
		ReasoningEffort: s.ThinkingEffort, Format: format,
	}
}

func (p *Pipeline) request(spec Spec, msgs []llm.Message, format *llm.ResponseFormat) llm.Request {
	return NewRequest(spec, msgs, format)
}

// SystemPrompt is the work type's system prompt plus its output schema.
func SystemPrompt(wt worktype.WorkType) string {
	return wt.SystemPrompt + "\n\nOutput JSON schema:\n" + string(wt.Schema)
}

func (p *Pipeline) reduceStep(ctx context.Context, spec Spec, prompt string, docs []extract.Document,
	_ *schema.Schema, res Result, onDelta func(string),
) (Result, error) {
	user := userMessage(prompt, spec.Memory, docs)
	if spec.Revise != nil {
		user += spec.Revise.text()
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: SystemPrompt(spec.WorkType)},
		{Role: llm.RoleUser, Content: user},
	}
	retries := p.MaxValidationRetries
	if retries == 0 {
		retries = 2
	}
	return Generate(ctx, p.Model, spec, msgs, res, retries, onDelta)
}

// Generate makes the structured-output call with the work type's schema,
// validates the reply and retries up to retries times with the validation
// error, adding attempts and tokens to res.
func Generate(ctx context.Context, m Model, spec Spec, msgs []llm.Message, res Result, retries int,
	onDelta func(string),
) (Result, error) {
	sch, err := spec.WorkType.ParsedSchema()
	if err != nil {
		return res, err
	}
	format := &llm.ResponseFormat{Name: spec.WorkType.ID, Schema: json.RawMessage(spec.WorkType.Schema)}
	msgs = append([]llm.Message(nil), msgs...)
	for attempt := 0; ; attempt++ {
		out, err := m.ChatStream(ctx, NewRequest(spec, msgs, format), onDelta)
		res.Attempts = append(res.Attempts, out.Attempts...)
		if err != nil {
			return res, fmt.Errorf("generate %s: %w", spec.WorkType.ID, err)
		}
		res.Model, res.TotalTokens = out.Target, res.TotalTokens+out.Usage.TotalTokens
		clean := schema.StripFences(out.Content)
		verr := sch.ValidateJSON([]byte(clean))
		if verr == nil {
			res.Data, res.Valid, res.Validation = json.RawMessage(clean), true, ""
			return res, nil
		}
		res.Data, res.Validation = json.RawMessage(jsonOrString(clean)), verr.Error()
		if attempt >= retries {
			return res, fmt.Errorf("%w: %w", llm.ErrInvalidOutput, verr)
		}
		msgs = append(msgs,
			llm.Message{Role: llm.RoleAssistant, Content: out.Content},
			llm.Message{Role: llm.RoleUser, Content: "Your reply did not match the schema: " + verr.Error() +
				"\nReply again with corrected JSON only."},
		)
	}
}

// mapStep condenses each source (split into budget-sized pieces) into notes
// relevant to the prompt, so the final call fits the context budget.
func (p *Pipeline) mapStep(ctx context.Context, spec Spec, prompt string, docs []extract.Document, budget int,
	res *Result,
) ([]extract.Document, error) {
	pieceTokens := max(budget/2, 1000)
	var notes []extract.Document
	for _, d := range docs {
		for i, piece := range split(d.Text, pieceTokens*4) {
			msgs := []llm.Message{
				{Role: llm.RoleSystem, Content: "Extract the facts from this source that matter for the task. " +
					"Keep names, numbers, file paths and quotes needed as evidence. Be concise."},
				{Role: llm.RoleUser, Content: "Task: " + prompt + "\n\n" + formatContext([]extract.Document{
					{Source: d.Source, Kind: d.Kind, Text: piece},
				})},
			}
			out, err := p.Model.ChatStream(ctx, p.request(spec, msgs, nil), nil)
			res.Attempts = append(res.Attempts, out.Attempts...)
			if err != nil {
				return nil, fmt.Errorf("condense %s: %w", d.Source, err)
			}
			res.TotalTokens += out.Usage.TotalTokens
			notes = append(notes, extract.Document{
				Source: fmt.Sprintf("%s (part %d notes)", d.Source, i+1),
				Kind:   "notes", Text: out.Content,
			})
		}
	}
	return notes, nil
}

func userMessage(prompt string, memory []string, docs []extract.Document) string {
	var sb strings.Builder
	sb.WriteString(prompt)
	if len(memory) > 0 {
		sb.WriteString("\n\nNotebook memory (facts to respect):\n")
		for _, m := range memory {
			sb.WriteString("- " + m + "\n")
		}
	}
	if len(docs) > 0 {
		sb.WriteString("\n\nContext:\n\n")
		sb.WriteString(formatContext(docs))
	}
	return sb.String()
}

// split breaks text into pieces of at most n runes, preferring line breaks.
func split(text string, n int) []string {
	r := []rune(text)
	if len(r) <= n {
		return []string{text}
	}
	var out []string
	for len(r) > 0 {
		end := min(n, len(r))
		if end < len(r) {
			if nl := strings.LastIndex(string(r[:end]), "\n"); nl > 0 {
				end = len([]rune(string(r[:end])[:nl])) + 1
			}
		}
		out = append(out, string(r[:end]))
		r = r[end:]
	}
	return out
}

func jsonOrString(s string) string {
	if json.Valid([]byte(s)) {
		return s
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// reserveForRevision takes the parent document and the requests out of the
// context budget, leaving the sources at least a quarter of it.
func reserveForRevision(budget int, r *Revision) int {
	if r == nil {
		return budget
	}
	left := budget - extract.EstimateTokens(string(r.Parent)+r.Requests)
	return max(left, budget/4)
}
