package service

import (
	"context"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
)

// ModelCheck is the result of one tiny request to one model.
type ModelCheck struct {
	Model  string `json:"model"` // provider/model
	OK     bool   `json:"ok"`
	Millis int64  `json:"millis"`
	Reply  string `json:"reply,omitempty"` // the start of the answer
	Error  string `json:"error,omitempty"` // redacted
}

// Limits of a model check: a few tokens, no retries, a short deadline.
const (
	checkTokens  = 16
	checkTimeout = 30 * time.Second
	checkReply   = 200
)

// TestModel sends one tiny request to every model behind name (an alias or
// provider/model), one model at a time, with no retries and no fallback, so
// each result says whether that model answers. Tokens count toward the limits.
func (s *Service) TestModel(ctx context.Context, name string) ([]ModelCheck, error) {
	alias, err := s.cfg().ResolveModel(name)
	if err != nil {
		return nil, &InvalidError{err.Error()}
	}
	redact := s.redactor()
	out := make([]ModelCheck, 0, len(alias.Chain))
	for _, ref := range alias.Chain {
		out = append(out, s.checkOne(ctx, ref, redact))
	}
	return out, nil
}

func (s *Service) checkOne(ctx context.Context, ref config.ModelRef, redact func(string) string) ModelCheck {
	zero := 0
	label := ref.Provider + "/" + ref.Model
	one := config.Alias{Name: label, Chain: []config.ModelRef{ref}, MaxRetries: &zero}
	c := ModelCheck{Model: label}
	model, err := s.aliasModel(one)
	if err != nil {
		c.Error = redact(err.Error())
		return c
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	tokens := checkTokens
	req := llm.Request{
		Messages:        []llm.Message{{Role: "user", Content: "Reply with the word OK."}},
		MaxTokens:       &tokens,
		ReasoningEffort: "off",
	}
	start := time.Now()
	_, text, err := streamText(ctx, model, req, 0, nil)
	c.Millis = time.Since(start).Milliseconds()
	if err != nil {
		c.Error = redact(err.Error())
		return c
	}
	c.OK, c.Reply = true, truncateRunes(strings.TrimSpace(text), checkReply)
	return c
}
