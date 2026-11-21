package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
)

func TestGenerateRetriesThenValidates(t *testing.T) {
	spec := summarizeSpec(t)
	m := &scripted{replies: []string{`{"title":1}`, goodSummary}}
	msgs := []llm.Message{{Role: llm.RoleUser, Content: "go"}}
	res, err := Generate(context.Background(), m, spec, msgs, Result{Strategy: StrategyAgent, TotalTokens: 5}, 2, nil)
	if err != nil || !res.Valid || res.Strategy != StrategyAgent || res.TotalTokens != 25 || len(m.reqs) != 2 {
		t.Fatalf("res %+v err %v reqs %d", res, err, len(m.reqs))
	}
	if m.reqs[0].Format == nil || m.reqs[0].Format.Name != spec.WorkType.ID {
		t.Fatal("Generate must request the work type's schema")
	}
	if len(m.reqs[1].Messages) != 3 {
		t.Fatalf("retry must add the reply and the correction: %d messages", len(m.reqs[1].Messages))
	}
}

func TestGenerateGivesUp(t *testing.T) {
	m := &scripted{replies: []string{`{"title":1}`}}
	_, err := Generate(context.Background(), m, summarizeSpec(t), nil, Result{}, 0, nil)
	if !errors.Is(err, llm.ErrInvalidOutput) || len(m.reqs) != 1 {
		t.Fatalf("err %v reqs %d", err, len(m.reqs))
	}
}

func TestNewRequestCarriesSettings(t *testing.T) {
	spec := summarizeSpec(t)
	req := NewRequest(spec, []llm.Message{{Role: llm.RoleUser, Content: "x"}}, nil)
	if req.ReasoningEffort != "low" || req.MaxTokens == nil || *req.MaxTokens != 99 || len(req.Messages) != 1 {
		t.Fatalf("request: %+v", req)
	}
}
