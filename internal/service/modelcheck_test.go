package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// checkModel answers OK, or fails for the model "down" with a key in the error.
type checkModel struct{ model string }

func (m checkModel) ChatStream(_ context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	if m.model == "down" {
		return llm.Result{}, errors.New("401 unauthorized for key sk-live-1234567890abcdef1234")
	}
	if req.MaxTokens == nil || *req.MaxTokens > 32 || len(req.Messages) != 1 {
		return llm.Result{}, errors.New("the check must be one tiny request")
	}
	onDelta("OK")
	return llm.Result{Target: "litellm/" + m.model}, nil
}

func TestTestModelChecksEveryModelOfAnAlias(t *testing.T) {
	s := newSvc(t, summary)
	s.Config.Aliases = []config.Alias{{Name: "smart", Chain: []config.ModelRef{
		{Provider: "litellm", Model: "up"}, {Provider: "litellm", Model: "down"},
	}}}
	var chains []int
	s.NewModel = func(_ config.File, a config.Alias, _ secrets.Resolver) (executor.Model, error) {
		chains = append(chains, len(a.Chain))
		if a.Retries() != 0 {
			t.Errorf("a check must not retry: %d", a.Retries())
		}
		return checkModel{a.Chain[0].Model}, nil
	}
	got, err := s.TestModel(context.Background(), "smart")
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %+v", err, got)
	}
	if !got[0].OK || got[0].Model != "litellm/up" || got[0].Reply != "OK" {
		t.Fatalf("up: %+v", got[0])
	}
	if got[1].OK || !strings.Contains(got[1].Error, "401") || strings.Contains(got[1].Error, "sk-live") {
		t.Fatalf("down must fail with a redacted error: %+v", got[1])
	}
	if len(chains) != 2 || chains[0] != 1 || chains[1] != 1 {
		t.Fatalf("each model is checked alone, without fallback: %v", chains)
	}
	one, err := s.TestModel(context.Background(), "litellm/up")
	if err != nil || len(one) != 1 || !one[0].OK {
		t.Fatalf("provider/model: %v %+v", err, one)
	}
	if _, err := s.TestModel(context.Background(), "nope"); !IsInvalid(err) {
		t.Fatalf("unknown model: %v", err)
	}
}
