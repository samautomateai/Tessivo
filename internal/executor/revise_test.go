package executor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
)

func TestPipelineReviseIncludesParent(t *testing.T) {
	spec := summarizeSpec(t)
	spec.Revise = &Revision{Parent: json.RawMessage(goodSummary), Requests: "user: fix point 3"}
	m := &scripted{replies: []string{goodSummary}}
	if _, err := (&Pipeline{Model: m}).Run(context.Background(), spec, nil); err != nil {
		t.Fatal(err)
	}
	user := m.reqs[0].Messages[1].Content
	for _, want := range []string{
		"Summarize contracts", "Document to revise (JSON):\n" + goodSummary,
		"Revision requests:\nuser: fix point 3", "Change only what the requests ask for.",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("revision message lacks %q", want)
		}
	}
}

func TestPipelineReviseBudget(t *testing.T) {
	spec := summarizeSpec(t)
	spec.Settings.ContextBudget = config.Ptr(1000)
	spec.Documents = []extract.Document{{Source: "a.md", Kind: "text", Text: strings.Repeat("x", 1600)}} // ≈400 tokens
	parent := `{"title":"` + strings.Repeat("p", 2800) + `"}`                                            // ≈700 tokens
	spec.Revise = &Revision{Parent: json.RawMessage(parent), Requests: "user: shorter"}
	m := &scripted{replies: []string{"notes", goodSummary}}
	res, err := (&Pipeline{Model: m}).Run(context.Background(), spec, nil)
	if err != nil || res.Strategy != StrategyMapReduce {
		t.Fatalf("want map-reduce once the parent takes its share: %+v %v", res, err)
	}
	spec.Revise = nil
	m = &scripted{replies: []string{goodSummary}}
	if res, _ := (&Pipeline{Model: m}).Run(context.Background(), spec, nil); res.Strategy != StrategyFull {
		t.Fatalf("without a revision the sources fit: %s", res.Strategy)
	}
}
