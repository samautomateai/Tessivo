package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// Compaction: once the prompt passes compactAt of the budget, the oldest tool
// results are replaced by a one-line note. The final step cuts each finding.
const (
	compactAt   = 0.6
	findingSize = 2 << 10
)

func budget(spec executor.Spec) int {
	if spec.Settings.ContextBudget != nil {
		return *spec.Settings.ContextBudget
	}
	return executor.DefaultContextBudget
}

func promptTokens(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += extract.EstimateTokens(m.Content)
		for _, c := range m.ToolCalls {
			n += extract.EstimateTokens(c.Arguments)
		}
	}
	return n
}

// compact elides the oldest tool results until the prompt fits.
func compact(cp *Checkpoint, budget int) {
	limit := int(float64(budget) * compactAt)
	for i := range cp.Messages {
		if promptTokens(cp.Messages) <= limit {
			return
		}
		m := &cp.Messages[i]
		if m.Role == llm.RoleTool && !strings.HasPrefix(m.Content, "[result elided") {
			m.Content = fmt.Sprintf("[result elided: %d bytes]", len(m.Content))
		}
	}
}

// final turns the transcript into the work type's validated output.
func (a *Agent) final(ctx context.Context, spec executor.Spec, cp *Checkpoint, note string,
	emit func(executor.Event),
) (executor.Result, error) {
	emit(executor.Event{Kind: executor.EventStatus, Text: "generating " + spec.WorkType.Name})
	prompt, err := executor.Fill(spec.Prompt, spec.Vars)
	if err != nil {
		return executor.Result{}, err
	}
	var user strings.Builder
	user.WriteString(prompt)
	for _, m := range spec.Memory {
		user.WriteString("\n- " + m)
	}
	user.WriteString("\n\nFindings from the investigation:\n")
	user.WriteString(findings(cp.Messages, budget(spec)*4))
	if note != "" {
		user.WriteString("\n(" + note + "; answer with the evidence gathered so far.)\n")
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: executor.SystemPrompt(spec.WorkType)},
		{Role: llm.RoleUser, Content: user.String()},
	}
	res := executor.Result{Strategy: executor.StrategyAgent, TotalTokens: cp.Tokens, Attempts: llmAttempts(cp.Attempts)}
	retries := a.MaxValidationRetries
	if retries == 0 {
		retries = 2
	}
	res, err = executor.Generate(ctx, a.Model, spec, msgs, res, retries,
		func(s string) { emit(executor.Event{Kind: executor.EventDelta, Text: s}) })
	cp.Tokens = res.TotalTokens
	return res, err
}

// findings lists assistant notes and tool results after the opening messages,
// each cut to findingSize, keeping the newest when over maxChars.
func findings(msgs []llm.Message, maxChars int) string {
	names := map[string]string{}
	var parts []string
	for _, m := range msgs[min(2, len(msgs)):] {
		switch m.Role {
		case llm.RoleAssistant:
			for _, c := range m.ToolCalls {
				names[c.ID] = c.Name
			}
			if strings.TrimSpace(m.Content) != "" {
				parts = append(parts, "Note: "+cut(m.Content))
			}
		case llm.RoleTool:
			parts = append(parts, fmt.Sprintf("[%s] %s", names[m.ToolCallID], cut(m.Content)))
		}
	}
	out := strings.Join(parts, "\n")
	if len(out) > maxChars {
		out = "…" + out[len(out)-maxChars:]
	}
	return out
}

func cut(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > findingSize {
		return s[:findingSize] + "…"
	}
	return s
}

func llmAttempts(rs []notebook.AttemptRecord) []llm.Attempt {
	out := make([]llm.Attempt, 0, len(rs))
	for _, r := range rs {
		at := llm.Attempt{Target: r.Target, Trigger: r.Trigger}
		if r.Error != "" {
			at.Err = errors.New(r.Error)
		}
		out = append(out, at)
	}
	return out
}
