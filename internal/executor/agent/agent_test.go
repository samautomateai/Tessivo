package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/worktype"
)

const goodSummary = `{"title":"T","tldr":"x","key_points":["a"],"per_source":[],"action_items":[]}`

// fakeModel returns queued replies in order and records every request.
type fakeModel struct {
	mu      sync.Mutex
	replies []llm.Response
	errs    []error
	reqs    []llm.Request
}

func (m *fakeModel) ChatStream(_ context.Context, req llm.Request, _ func(string)) (llm.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs = append(m.reqs, req)
	if len(m.errs) > 0 && m.errs[0] != nil {
		err := m.errs[0]
		m.errs = m.errs[1:]
		return llm.Result{Attempts: []llm.Attempt{{Target: "p/m", Err: err}}}, err
	}
	if len(m.errs) > 0 {
		m.errs = m.errs[1:]
	}
	if len(m.replies) == 0 {
		return llm.Result{}, errors.New("fake model: no reply queued")
	}
	r := m.replies[0]
	if len(m.replies) > 1 {
		m.replies = m.replies[1:]
	}
	r.Usage.TotalTokens = 10
	return llm.Result{Response: r, Target: "p/m", Attempts: []llm.Attempt{{Target: "p/m"}}}, nil
}

func calls(cs ...llm.ToolCall) llm.Response { return llm.Response{ToolCalls: cs} }

func call(id, name, args string) llm.ToolCall {
	return llm.ToolCall{ID: id, Name: name, Arguments: args}
}

// fakeTool counts its runs and returns a fixed result.
type fakeTool struct {
	name string
	mu   sync.Mutex
	runs int
	out  string
}

func (t *fakeTool) Def() llm.ToolDef {
	return llm.ToolDef{Name: t.name, Description: "fake", Parameters: json.RawMessage(`{"type":"object"}`)}
}

func (t *fakeTool) Run(context.Context, json.RawMessage) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runs++
	return t.out, nil
}

func (t *fakeTool) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.runs
}

func spec(t *testing.T) executor.Spec {
	t.Helper()
	wt, err := worktype.Get(worktype.Summarize)
	if err != nil {
		t.Fatal(err)
	}
	return executor.Spec{WorkType: wt, Prompt: "Summarize the repo", Settings: config.Effective{}}
}

type harness struct {
	agent *Agent
	model *fakeModel
	saves []Checkpoint
	read  *fakeTool
	risky *fakeTool
}

func newHarness(replies ...llm.Response) *harness {
	h := &harness{
		model: &fakeModel{replies: replies},
		read:  &fakeTool{name: "read_file", out: "file text"},
		risky: &fakeTool{name: "x", out: "did x"},
	}
	h.agent = &Agent{
		Model:  h.model,
		Tools:  agenttools.NewRegistry(h.read, h.risky),
		Limits: Limits{MaxSteps: 10, MaxTokens: 100000, MaxWall: time.Minute},
		Save: func(cp *Checkpoint) error {
			b, _ := json.Marshal(cp)
			var c Checkpoint
			_ = json.Unmarshal(b, &c)
			h.saves = append(h.saves, c)
			return nil
		},
	}
	return h
}

func (h *harness) last() Checkpoint { return h.saves[len(h.saves)-1] }

func TestAgentFinishes(t *testing.T) {
	h := newHarness(
		calls(call("c1", "read_file", `{"path":"a"}`)),
		llm.Response{Content: "I have what I need."},
		llm.Response{Content: goodSummary},
	)
	cp := &Checkpoint{RunID: "r1"}
	res, err := h.agent.Start(context.Background(), spec(t), cp, nil)
	if err != nil || !res.Valid || res.Strategy != executor.StrategyAgent {
		t.Fatalf("res %+v err %v", res, err)
	}
	if cp.Steps != 2 || h.read.count() != 1 || len(h.saves) < 2 {
		t.Fatalf("steps %d reads %d saves %d", cp.Steps, h.read.count(), len(h.saves))
	}
	if len(cp.ToolCalls) != 1 || cp.ToolCalls[0].Tool != "read_file" || cp.ToolCalls[0].Policy != "allow" {
		t.Fatalf("tool records: %+v", cp.ToolCalls)
	}
	first := h.model.reqs[0]
	if len(first.Tools) != 3 || first.ToolChoice != "auto" {
		t.Fatalf("first request must offer read_file, x and finish: %d tools", len(first.Tools))
	}
	final := h.model.reqs[len(h.model.reqs)-1]
	if final.Format == nil || len(final.Tools) != 0 || !strings.Contains(final.Messages[1].Content, "file text") {
		t.Fatal("the final call must use the schema, no tools, and carry the findings")
	}
}

func TestAgentPausesOnAsk(t *testing.T) {
	h := newHarness(calls(call("c1", "read_file", `{}`), call("c2", "x", `{"a":1}`), call("c3", "read_file", `{}`)))
	cp := &Checkpoint{RunID: "r1"}
	_, err := h.agent.Start(context.Background(), spec(t), cp, nil)
	var paused *PausedError
	if !errors.Is(err, ErrPaused) || !errors.As(err, &paused) || len(paused.Approvals) != 1 {
		t.Fatalf("want a pause with one approval, got %v", err)
	}
	saved := h.last()
	if saved.Status != "waiting_approval" || len(saved.Pending) != 2 || saved.Pending[0].ApprovalID == "" {
		t.Fatalf("checkpoint: status %q pending %+v", saved.Status, saved.Pending)
	}
	if h.read.count() != 1 || h.risky.count() != 0 {
		t.Fatalf("calls before the ask run; the ask and later calls wait: reads %d x %d",
			h.read.count(), h.risky.count())
	}
	if len(saved.Approvals) != 1 || saved.Approvals[0].Tool != "x" || saved.Approvals[0].RequestedAt == "" {
		t.Fatalf("approvals: %+v", saved.Approvals)
	}
}

func pausedCheckpoint(t *testing.T, h *harness) *Checkpoint {
	t.Helper()
	cp := &Checkpoint{RunID: "r1"}
	if _, err := h.agent.Start(context.Background(), spec(t), cp, nil); !errors.Is(err, ErrPaused) {
		t.Fatalf("want pause, got %v", err)
	}
	return cp
}

func TestAgentContinueAllowDeny(t *testing.T) {
	for _, answer := range []string{"allow", "deny"} {
		t.Run(answer, func(t *testing.T) {
			h := newHarness(
				calls(call("c1", "x", `{}`)),
				llm.Response{Content: "done"},
				llm.Response{Content: goodSummary},
			)
			cp := pausedCheckpoint(t, h)
			cp.Pending[0].Answer = answer
			res, err := h.agent.Continue(context.Background(), spec(t), cp, nil)
			if err != nil || !res.Valid {
				t.Fatalf("res %+v err %v", res, err)
			}
			wantRuns, wantText := 1, "did x"
			if answer == "deny" {
				wantRuns, wantText = 0, "denied by the user"
			}
			if h.risky.count() != wantRuns {
				t.Fatalf("x ran %d times, want %d", h.risky.count(), wantRuns)
			}
			if got := toolResult(h.model.reqs[1], "c1"); !strings.Contains(got, wantText) {
				t.Fatalf("model saw %q, want %q", got, wantText)
			}
		})
	}
}

func toolResult(req llm.Request, id string) string {
	for _, m := range req.Messages {
		if m.Role == llm.RoleTool && m.ToolCallID == id {
			return m.Content
		}
	}
	return ""
}

func TestAgentRemembersForRun(t *testing.T) {
	h := newHarness(
		calls(call("c1", "x", `{}`)),
		calls(call("c2", "x", `{}`)),
		llm.Response{Content: "done"},
		llm.Response{Content: goodSummary},
	)
	cp := pausedCheckpoint(t, h)
	cp.Pending[0].Answer = "allow"
	cp.RunRules = map[string]string{"x": "allow"}
	if _, err := h.agent.Continue(context.Background(), spec(t), cp, nil); err != nil {
		t.Fatalf("a tool allowed for the run must not ask again: %v", err)
	}
	if h.risky.count() != 2 {
		t.Fatalf("x ran %d times", h.risky.count())
	}
}

func TestAgentInterruptedCall(t *testing.T) {
	h := newHarness(llm.Response{Content: "ok"}, llm.Response{Content: goodSummary})
	cp := &Checkpoint{
		RunID: "r1", Status: "interrupted",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "s"},
			{Role: llm.RoleUser, Content: "u"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call("c1", "x", `{}`)}},
		},
		Pending: []PendingCall{{Call: call("c1", "x", `{}`), Decision: Allow, State: StateRunning}},
	}
	if _, err := h.agent.Continue(context.Background(), spec(t), cp, nil); err != nil {
		t.Fatal(err)
	}
	if h.risky.count() != 0 {
		t.Fatal("a call in flight at the crash must never run again")
	}
	if got := toolResult(h.model.reqs[0], "c1"); !strings.Contains(got, "outcome unknown") {
		t.Fatalf("model saw %q", got)
	}
}

func TestAgentLimits(t *testing.T) {
	loop := calls(call("c", "read_file", `{}`))
	h := newHarness(loop, loop, loop, llm.Response{Content: goodSummary})
	h.agent.Limits.MaxSteps = 2
	h.model.replies = []llm.Response{loop, loop, {Content: goodSummary}}
	cp := &Checkpoint{RunID: "r1"}
	res, err := h.agent.Start(context.Background(), spec(t), cp, nil)
	if err != nil || !res.Valid || cp.Steps != 2 {
		t.Fatalf("res %+v err %v steps %d", res, err, cp.Steps)
	}
	final := h.model.reqs[len(h.model.reqs)-1]
	if !strings.Contains(final.Messages[1].Content, "limit reached: steps") {
		t.Fatal("the final step must say which limit stopped the loop")
	}
}

func TestAgentBadArgs(t *testing.T) {
	h := newHarness(
		calls(call("c1", "read_file", `{"path":`)),
		llm.Response{Content: "done"},
		llm.Response{Content: goodSummary},
	)
	if _, err := h.agent.Start(context.Background(), spec(t), &Checkpoint{RunID: "r1"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := toolResult(h.model.reqs[1], "c1"); !strings.Contains(got, "invalid arguments") || h.read.count() != 0 {
		t.Fatalf("model saw %q, reads %d", got, h.read.count())
	}
}

func TestAgentBlockedAndUnknownTools(t *testing.T) {
	h := newHarness(
		calls(call("c1", "read_file", `{}`), call("c2", "nope", `{}`)),
		llm.Response{Content: "done"},
		llm.Response{Content: goodSummary},
	)
	h.agent.Policy = Policy{Notebook: map[string]string{"read_file": "block"}}
	if _, err := h.agent.Start(context.Background(), spec(t), &Checkpoint{RunID: "r1"}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolResult(h.model.reqs[1], "c1"), "denied by policy") ||
		!strings.Contains(toolResult(h.model.reqs[1], "c2"), "unknown tool") {
		t.Fatal("blocked and unknown tools must answer the model without running")
	}
}

func TestAgentToolsUnsupported(t *testing.T) {
	h := newHarness()
	h.model.errs = []error{fmt.Errorf("%w: provider said no", llm.ErrToolsUnsupported)}
	sp := spec(t)
	sp.Settings.Model = "small"
	_, err := h.agent.Start(context.Background(), sp, &Checkpoint{RunID: "r1"}, nil)
	if !errors.Is(err, llm.ErrToolsUnsupported) || !strings.Contains(err.Error(), "small") {
		t.Fatalf("want an error naming the model, got %v", err)
	}
}

func TestAgentFinishTool(t *testing.T) {
	h := newHarness(calls(call("c1", "finish", `{"rationale":"enough"}`)), llm.Response{Content: goodSummary})
	cp := &Checkpoint{RunID: "r1"}
	if res, err := h.agent.Start(context.Background(), spec(t), cp, nil); err != nil || !res.Valid || cp.Steps != 1 {
		t.Fatalf("res %+v err %v steps %d", res, err, cp.Steps)
	}
}

func TestAgentCompactsOldResults(t *testing.T) {
	big := strings.Repeat("y", 4000)
	h := newHarness(
		calls(call("c1", "read_file", `{}`)),
		calls(call("c2", "read_file", `{}`)),
		calls(call("c3", "read_file", `{}`)),
		llm.Response{Content: "done"},
		llm.Response{Content: goodSummary},
	)
	h.read.out = big
	sp := spec(t)
	sp.Settings.ContextBudget = config.Ptr(2000)
	if _, err := h.agent.Start(context.Background(), sp, &Checkpoint{RunID: "r1"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := toolResult(h.model.reqs[3], "c1"); !strings.Contains(got, "result elided") {
		t.Fatalf("the oldest result must be elided once the prompt is large, got %d bytes", len(got))
	}
}

func TestAgentFillsMissingToolCallIDs(t *testing.T) {
	h := newHarness(
		calls(call("", "read_file", `{}`)),
		llm.Response{Content: "done"},
		llm.Response{Content: goodSummary},
	)
	if _, err := h.agent.Start(context.Background(), spec(t), &Checkpoint{RunID: "r1"}, nil); err != nil {
		t.Fatal(err)
	}
	var callID, resultID string
	for _, m := range h.model.reqs[1].Messages {
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) == 1 {
			callID = m.ToolCalls[0].ID
		}
		if m.Role == llm.RoleTool {
			resultID = m.ToolCallID
		}
	}
	if callID == "" || callID != resultID {
		t.Fatalf("a call without an id must get one, used by its result: call %q result %q", callID, resultID)
	}
}

func TestAgentToolCallLimit(t *testing.T) {
	two := calls(call("a", "read_file", `{}`), call("b", "read_file", `{}`))
	h := newHarness(two, two, llm.Response{Content: goodSummary})
	h.agent.Limits.MaxToolCalls = 3
	h.model.replies = []llm.Response{two, two, two, {Content: goodSummary}}
	cp := &Checkpoint{RunID: "r1"}
	res, err := h.agent.Start(context.Background(), spec(t), cp, nil)
	if err != nil || !res.Valid || len(cp.ToolCalls) != 4 {
		t.Fatalf("res %+v err %v calls %d", res, err, len(cp.ToolCalls))
	}
	final := h.model.reqs[len(h.model.reqs)-1]
	if !strings.Contains(final.Messages[1].Content, "limit reached: tool calls") {
		t.Fatal("the final step must say the tool-call limit stopped the loop")
	}
}
