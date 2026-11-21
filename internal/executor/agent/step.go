package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// Tool execution bounds.
const (
	maxParallel = 4
	maxResult   = 32 << 10
	eventText   = 2 << 10
)

// runPending works through cp.Pending in order: it answers blocked, denied and
// invalid calls at once, runs consecutive runnable calls in parallel batches,
// and pauses at the first call that still needs the user.
func (a *Agent) runPending(ctx context.Context, cp *Checkpoint, emit func(executor.Event)) error {
	for len(cp.Pending) > 0 {
		p := cp.Pending[0]
		if msg, ok := a.immediate(p); ok {
			a.answer(cp, p, msg, "", 0)
			cp.Pending = cp.Pending[1:]
			emitResult(emit, p, msg)
			continue
		}
		if p.Decision == Ask && p.Answer == "" {
			return a.pause(cp, emit)
		}
		if err := a.runBatch(ctx, cp, emit); err != nil {
			return err
		}
	}
	return a.save(cp)
}

// immediate returns the result for calls that are answered without running.
func (a *Agent) immediate(p PendingCall) (string, bool) {
	if _, ok := a.Tools.Get(p.Call.Name); !ok {
		return fmt.Sprintf("unknown tool %q", p.Call.Name), true
	}
	if !json.Valid([]byte(orEmpty(p.Call.Arguments))) {
		return "invalid arguments: not valid JSON", true
	}
	switch {
	case p.Decision == Block:
		return "denied by policy", true
	case p.Decision == Ask && p.Answer == "deny":
		return "denied by the user", true
	}
	return "", false
}

func runnable(p PendingCall) bool {
	return p.Decision == Allow || (p.Decision == Ask && p.Answer == "allow")
}

func orEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

// runBatch runs up to maxParallel consecutive runnable calls. They are marked
// running and saved first, so a crash leaves them recognisable.
func (a *Agent) runBatch(ctx context.Context, cp *Checkpoint, emit func(executor.Event)) error {
	n := 0
	for n < len(cp.Pending) && n < maxParallel && runnable(cp.Pending[n]) {
		if _, instant := a.immediate(cp.Pending[n]); instant {
			break
		}
		cp.Pending[n].State = StateRunning
		n++
	}
	if err := a.save(cp); err != nil {
		return err
	}
	batch := append([]PendingCall(nil), cp.Pending[:n]...)
	results := make([]toolOutcome, n)
	var wg sync.WaitGroup
	for i, p := range batch {
		emit(executor.Event{Kind: executor.EventToolCall, Text: p.Call.Name, Data: map[string]any{
			"id": p.Call.ID, keyTool: p.Call.Name, "args": a.redact(p.Call.Arguments),
		}})
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = a.runTool(ctx, p.Call)
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("agent cancelled: %w", err)
	}
	for i, p := range batch {
		a.answer(cp, p, results[i].text, results[i].errText, results[i].d)
		emitResult(emit, p, results[i].text)
	}
	cp.Pending = cp.Pending[n:]
	return a.save(cp)
}

type toolOutcome struct {
	text, errText string
	d             time.Duration
}

func (a *Agent) runTool(ctx context.Context, c llm.ToolCall) toolOutcome {
	t, _ := a.Tools.Get(c.Name)
	start := a.now()
	out, err := t.Run(ctx, json.RawMessage(orEmpty(c.Arguments)))
	d := a.now().Sub(start)
	if err != nil {
		return toolOutcome{text: "error: " + err.Error(), errText: err.Error(), d: d}
	}
	if len(out) > maxResult {
		out = out[:maxResult] + fmt.Sprintf("\n[result truncated at 32 KiB of %d bytes]", len(out))
	}
	return toolOutcome{text: out, d: d}
}

// answer appends the tool message the model will see and the run record.
func (a *Agent) answer(cp *Checkpoint, p PendingCall, text, errText string, d time.Duration) {
	cp.Messages = append(cp.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: p.Call.ID, Content: text})
	rec := notebook.ToolCallRecord{
		ID: p.Call.ID, Tool: p.Call.Name, Policy: string(p.Decision), Error: errText, DurationMS: d.Milliseconds(),
		Args:   notebook.TruncateRecord(a.redact(p.Call.Arguments)),
		Result: notebook.TruncateRecord(a.redact(text)),
	}
	if p.Answer != "" {
		rec.Decision = p.Answer
	}
	cp.ToolCalls = append(cp.ToolCalls, rec)
}

// pause gives the first waiting call an approval id, records the question and
// saves the checkpoint as waiting.
func (a *Agent) pause(cp *Checkpoint, emit func(executor.Event)) error {
	p := &cp.Pending[0]
	if p.ApprovalID == "" {
		p.ApprovalID = notebook.NewID()
		cp.Approvals = append(cp.Approvals, notebook.ApprovalRecord{
			ID: p.ApprovalID, Tool: p.Call.Name, Args: notebook.TruncateRecord(a.redact(p.Call.Arguments)),
			RequestedAt: a.stamp(),
		})
	}
	cp.Status = notebook.StatusWaiting
	if err := a.save(cp); err != nil {
		return err
	}
	emit(executor.Event{Kind: executor.EventApprovalNeeded, Text: p.Call.Name, Data: map[string]any{
		"runId": cp.RunID, "approvalId": p.ApprovalID, keyTool: p.Call.Name, "args": a.redact(p.Call.Arguments),
	}})
	return &PausedError{Approvals: []PendingCall{*p}}
}

func (a *Agent) redact(s string) string {
	if a.Redact == nil {
		return s
	}
	return a.Redact(s)
}

func emitResult(emit func(executor.Event), p PendingCall, text string) {
	if len(text) > eventText {
		text = text[:eventText] + "…"
	}
	emit(executor.Event{Kind: executor.EventToolResult, Text: p.Call.Name, Data: map[string]any{
		"id": p.Call.ID, keyTool: p.Call.Name, "result": text,
	}})
}

// keyTool names the tool in event data.
const keyTool = "tool"
