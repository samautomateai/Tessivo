package executor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// scripted returns canned replies in order and records requests.
type scripted struct {
	replies []string
	err     error
	reqs    []llm.Request
}

func (s *scripted) ChatStream(_ context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return llm.Result{Attempts: []llm.Attempt{{Target: "p/a", Err: s.err}}}, s.err
	}
	r := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	if onDelta != nil {
		onDelta(r)
	}
	return llm.Result{Response: llm.Response{Content: r, Usage: llm.Usage{TotalTokens: 10}}, Target: "p/a",
		Attempts: []llm.Attempt{{Target: "p/a"}}}, nil
}

const goodSummary = `{"title":"T","tldr":"x","key_points":["a"],"per_source":[],"action_items":[]}`

func summarizeSpec(t *testing.T) Spec {
	t.Helper()
	wt, err := worktype.Get(worktype.Summarize)
	if err != nil {
		t.Fatal(err)
	}
	return Spec{
		WorkType: wt, Prompt: "Summarize {{topic}}", Vars: map[string]string{"topic": "contracts"},
		Documents: []extract.Document{{Source: "a.md", Kind: "markdown", Text: "alpha"}},
		Memory:    []string{"Fiscal year starts in April"},
		Settings:  config.Effective{RunSettings: config.RunSettings{ThinkingEffort: "low", MaxOutputTokens: config.Ptr(99)}},
	}
}

func TestPipelineValidFirstTime(t *testing.T) {
	m := &scripted{replies: []string{"```json\n" + goodSummary + "\n```"}}
	var events []Event
	onEvent := func(e Event) { events = append(events, e) }
	res, err := (&Pipeline{Model: m}).Run(context.Background(), summarizeSpec(t), onEvent)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || string(res.Data) != goodSummary || res.Model != "p/a" || res.Strategy != StrategyFull {
		t.Fatalf("res %+v", res)
	}
	req := m.reqs[0]
	if req.Format == nil || req.Format.Name != "summarize" || req.ReasoningEffort != "low" || *req.MaxTokens != 99 {
		t.Fatalf("request %+v", req)
	}
	user := req.Messages[1].Content
	for _, want := range []string{"Summarize contracts", "Fiscal year starts in April", `<source path="a.md"`} {
		if !strings.Contains(user, want) {
			t.Errorf("user message missing %q", want)
		}
	}
	if len(events) < 2 || events[len(events)-1].Kind != EventDelta {
		t.Fatalf("events %+v", events)
	}
}

func TestPipelineRetriesWithValidationFeedback(t *testing.T) {
	m := &scripted{replies: []string{`{"title":"T"}`, goodSummary}}
	res, err := (&Pipeline{Model: m}).Run(context.Background(), summarizeSpec(t), nil)
	if err != nil || !res.Valid || len(m.reqs) != 2 || res.TotalTokens != 20 {
		t.Fatalf("res %+v err %v", res, err)
	}
	last := m.reqs[1].Messages
	if !strings.Contains(last[len(last)-1].Content, "missing required property") {
		t.Fatal("validation errors not fed back")
	}
}

func TestPipelineGivesUpAfterRetries(t *testing.T) {
	m := &scripted{replies: []string{"not json at all"}}
	res, err := (&Pipeline{Model: m, MaxValidationRetries: -1}).Run(context.Background(), summarizeSpec(t), nil)
	if !errors.Is(err, llm.ErrInvalidOutput) || res.Valid || string(res.Data) != `"not json at all"` {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(m.reqs) != 1 {
		t.Fatalf("want no retries, got %d calls", len(m.reqs))
	}
}

func TestPipelineMapReduce(t *testing.T) {
	spec := summarizeSpec(t)
	spec.Settings.ContextBudget = config.Ptr(10)
	spec.Documents = []extract.Document{{Source: "big.txt", Kind: "text", Text: strings.Repeat("line\n", 2000)}}
	m := &scripted{replies: []string{"note one", "note two", goodSummary}}
	res, err := (&Pipeline{Model: m}).Run(context.Background(), spec, nil)
	if err != nil || res.Strategy != StrategyMapReduce || !res.Valid {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(m.reqs) != 4 || m.reqs[0].Format != nil {
		t.Fatalf("want 3 map calls without format + 1 reduce, got %d", len(m.reqs))
	}
	if !strings.Contains(m.reqs[3].Messages[1].Content, "big.txt (part 1 notes)") {
		t.Fatal("reduce should see condensed notes")
	}
}

func TestPipelineErrors(t *testing.T) {
	spec := summarizeSpec(t)
	spec.Vars = nil
	if _, err := (&Pipeline{Model: &scripted{}}).Run(context.Background(), spec, nil); err == nil ||
		!strings.Contains(err.Error(), "topic") {
		t.Fatalf("want missing variable error, got %v", err)
	}
	fail := &scripted{err: &llm.APIError{Status: 500}}
	if _, err := (&Pipeline{Model: fail}).Run(context.Background(), summarizeSpec(t), nil); err == nil {
		t.Fatal("want model error")
	}
	spec = summarizeSpec(t)
	spec.Settings.ContextBudget = config.Ptr(1)
	if _, err := (&Pipeline{Model: fail}).Run(context.Background(), spec, nil); err == nil ||
		!strings.Contains(err.Error(), "condense") {
		t.Fatalf("want map error, got %v", err)
	}
	spec.WorkType.Schema = []byte("{")
	if _, err := (&Pipeline{Model: fail}).Run(context.Background(), summarizeSpec2(spec), nil); err == nil {
		t.Fatal("want schema error")
	}
}

func summarizeSpec2(s Spec) Spec { s.Vars = map[string]string{"topic": "x"}; return s }

func TestHelpers(t *testing.T) {
	if got := Variables("{{a}} {{ b }} {{a}}"); strings.Join(got, ",") != "a,b" {
		t.Fatalf("vars %v", got)
	}
	parts := split("aaaa\nbbbb\ncccc", 6)
	if len(parts) != 3 || parts[0] != "aaaa\n" {
		t.Fatalf("split %q", parts)
	}
	if got := split("abcdefgh", 3); len(got) != 3 {
		t.Fatalf("split without newlines %q", got)
	}
	if userMessage("p", nil, nil) != "p" {
		t.Fatal("bare prompt")
	}
}
