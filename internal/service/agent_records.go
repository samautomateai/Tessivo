package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/executor/agent"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// saveAgentRun writes the run record (and output when valid) with the agent's
// status, steps, tool calls and approvals. It returns the output id.
func (s *Service) saveAgentRun(nb *notebook.Notebook, st *storedRun, spec executor.Spec, res executor.Result,
	runErr error,
) (string, error) {
	cp := st.Check
	started, err := time.Parse(time.RFC3339, st.StartedAt)
	if err != nil {
		started = s.now()
	}
	if res.Strategy == "" {
		res.Strategy = executor.StrategyAgent
	}
	if res.TotalTokens == 0 {
		res.TotalTokens = cp.Tokens
	}
	spec.Documents = nil
	return saveRun(nb, cp.RunID, spec, res, runErr, started, func(r *notebook.Run) {
		r.Mode, r.Status, r.ScheduleID, r.Steps = config.ModeAgent, cp.Status, cp.ScheduleID, cp.Steps
		r.Context, r.ToolCalls, r.Approvals = st.Spec.Context, cp.ToolCalls, cp.Approvals
		if len(r.Attempts) == 0 {
			r.Attempts = cp.Attempts
		}
	})
}

// auditTools writes tool calls not yet audited, as digests only.
func (s *Service) auditTools(rec *runAudit, st *storedRun) error {
	calls := st.Check.ToolCalls
	for ; st.Audited < len(calls); st.Audited++ {
		c := calls[st.Audited]
		data := map[string]any{
			"tool": c.Tool, "policy": c.Policy, "args_sha256": audit.Digest(c.Args),
			"result_sha256": audit.Digest(c.Result), "duration_ms": c.DurationMS,
		}
		if c.Decision != "" {
			data["decision"] = c.Decision
		}
		if c.Error != "" {
			data["error"] = "tool error"
		}
		if err := rec.log.Append(audit.KindToolCall, st.Check.RunID, data); err != nil {
			return err
		}
	}
	return nil
}

// pendingOf lists the run's unanswered approvals.
func (s *Service) pendingOf(nb *notebook.Notebook, st *storedRun) []PendingApproval {
	days := s.Config.Agent.WithDefaults().ApprovalExpiryDays
	var out []PendingApproval
	for _, p := range st.Check.Pending {
		if p.ApprovalID == "" || p.Answer != "" {
			continue
		}
		rec, ok := findApproval(st.Check, p.ApprovalID)
		if !ok {
			continue
		}
		req, _ := time.Parse(time.RFC3339, rec.RequestedAt)
		out = append(out, PendingApproval{
			RunID: st.Check.RunID, NotebookID: st.Check.NotebookID, NotebookName: nb.Manifest.Name,
			ScheduleID: st.Check.ScheduleID, ApprovalID: p.ApprovalID, Tool: rec.Tool, Args: rec.Args,
			RequestedAt: rec.RequestedAt, ExpiresAt: req.Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339),
		})
	}
	return out
}

func findApproval(cp *agent.Checkpoint, id string) (*notebook.ApprovalRecord, bool) {
	for i := range cp.Approvals {
		if cp.Approvals[i].ID == id {
			return &cp.Approvals[i], true
		}
	}
	return nil, false
}

func (s *Service) emit(nb *notebook.Notebook, cp *agent.Checkpoint, kind, approvalID, tool, errMsg string) {
	if s.OnRunEvent == nil {
		return
	}
	s.OnRunEvent(RunEvent{
		Kind: kind, RunID: cp.RunID, NotebookID: cp.NotebookID, NotebookName: nb.Manifest.Name,
		ScheduleID: cp.ScheduleID, ApprovalID: approvalID, Tool: tool, Error: errMsg,
		Time: s.now().UTC().Format(time.RFC3339),
	})
}

// jobTable tracks runs executing in this process: their cancel functions and
// event subscribers.
type jobTable struct {
	mu   sync.Mutex
	jobs map[string]*job
}

type job struct {
	cancel context.CancelFunc
	mu     sync.Mutex
	subs   map[chan executor.Event]struct{}
}

func (s *Service) jobTable() *jobTable {
	s.jobsOnce.Do(func() { s.jobs = &jobTable{jobs: map[string]*job{}} })
	return s.jobs
}

// track registers a live run. A detached run ignores the caller's
// cancellation. The returned emit fans events out to onEvent and subscribers.
func (t *jobTable) track(ctx context.Context, runID string, detached bool, onEvent func(executor.Event),
) (context.Context, func(executor.Event), func()) {
	if detached {
		ctx = context.WithoutCancel(ctx)
	}
	ctx, cancel := context.WithCancel(ctx)
	j := &job{cancel: cancel, subs: map[chan executor.Event]struct{}{}}
	t.mu.Lock()
	t.jobs[runID] = j
	t.mu.Unlock()
	emit := func(e executor.Event) {
		if onEvent != nil {
			onEvent(e)
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		for ch := range j.subs {
			select {
			case ch <- e:
			default: // a slow subscriber misses events rather than stalling the run
			}
		}
	}
	return ctx, emit, func() {
		cancel()
		t.mu.Lock()
		delete(t.jobs, runID)
		t.mu.Unlock()
		j.mu.Lock()
		for ch := range j.subs {
			close(ch)
		}
		j.subs = nil
		j.mu.Unlock()
	}
}

func (t *jobTable) get(runID string) (*job, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	j, ok := t.jobs[runID]
	return j, ok
}

// Subscribe streams a live run's events until it ends. ok is false when the
// run is not executing in this process.
func (s *Service) Subscribe(runID string) (events <-chan executor.Event, stop func(), ok bool) {
	j, ok := s.jobTable().get(runID)
	if !ok {
		return nil, func() {}, false
	}
	ch := make(chan executor.Event, 64)
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.subs == nil {
		return nil, func() {}, false
	}
	j.subs[ch] = struct{}{}
	return ch, func() {
		j.mu.Lock()
		defer j.mu.Unlock()
		if _, live := j.subs[ch]; live {
			delete(j.subs, ch)
			close(ch)
		}
	}, true
}

// liveRuns lists runs executing in this process.
func (s *Service) liveRuns() []string {
	t := s.jobTable()
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]string, 0, len(t.jobs))
	for id := range t.jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
