package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/executor/agent"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// Run event kinds, sent to OnRunEvent.
const (
	RunApprovalNeeded = "run.approval_needed"
	RunResumed        = "run.resumed"
	RunFinished       = "run.finished"
	RunFailed         = "run.failed"
	RunCancelled      = "run.cancelled"
)

// Agent run errors. Not-found errors wrap ErrNotFound and state errors wrap
// ErrConflict, so the API maps them to 404 and 409.
var (
	ErrRunNotFound      = fmt.Errorf("run %w", ErrNotFound)
	ErrApprovalNotFound = fmt.Errorf("approval %w", ErrNotFound)
	ErrApprovalAnswered = fmt.Errorf("approval already answered: %w", ErrConflict)
	ErrApprovalExpired  = fmt.Errorf("approval expired: %w", ErrConflict)
	ErrApprovalPending  = fmt.Errorf("run is still waiting for an approval: %w", ErrConflict)
	ErrRunNotPaused     = fmt.Errorf("run is not paused or interrupted: %w", ErrConflict)
)

// RunEvent tells listeners (the API's event stream, notifications) about an
// agent run.
type RunEvent struct {
	Kind         string `json:"kind"`
	RunID        string `json:"runId"`
	NotebookID   string `json:"notebookId"`
	NotebookName string `json:"notebookName"`
	ScheduleID   string `json:"scheduleId,omitempty"`
	ApprovalID   string `json:"approvalId,omitempty"`
	Tool         string `json:"tool,omitempty"`
	SourceID     string `json:"sourceId,omitempty"` // research summary events
	Error        string `json:"error,omitempty"`
	Time         string `json:"time"`
}

// PendingApproval is a tool call waiting for the user's answer.
type PendingApproval struct {
	RunID        string `json:"runId"`
	NotebookID   string `json:"notebookId"`
	NotebookName string `json:"notebookName"`
	ScheduleID   string `json:"scheduleId,omitempty"`
	Status       string `json:"status"` // waiting_approval, or interrupted (resume or cancel the run)
	ApprovalID   string `json:"approvalId,omitempty"`
	Tool         string `json:"tool"`
	Args         string `json:"args"`
	RequestedAt  string `json:"requestedAt"`
	ExpiresAt    string `json:"expiresAt"`
}

// storedRun is the checkpoint file: the agent's checkpoint plus what the
// service needs to continue the run.
type storedRun struct {
	Version   int               `json:"version"`
	Actor     string            `json:"actor"`
	StartedAt string            `json:"startedAt"`
	Spec      storedSpec        `json:"spec"`
	ToolRules map[string]string `json:"toolRules,omitempty"`
	Audited   int               `json:"audited"` // tool calls already written to the audit log
	Check     *agent.Checkpoint `json:"checkpoint"`
}

type storedSpec struct {
	WorkType string            `json:"workType"`
	Prompt   string            `json:"prompt"`
	Vars     map[string]string `json:"vars,omitempty"`
	Settings config.Effective  `json:"settings"`
	Memory   []string          `json:"memory,omitempty"`
	Context  []string          `json:"context,omitempty"`
}

func toStored(spec executor.Spec) storedSpec {
	st := storedSpec{
		WorkType: spec.WorkType.ID, Prompt: spec.Prompt, Vars: spec.Vars, Settings: spec.Settings, Memory: spec.Memory,
	}
	for _, d := range spec.Documents {
		st.Context = append(st.Context, d.Source)
	}
	return st
}

func (st storedSpec) spec() (executor.Spec, error) {
	wt, err := worktype.Get(st.WorkType)
	if err != nil {
		return executor.Spec{}, fmt.Errorf("stored run: %w", err)
	}
	return executor.Spec{WorkType: wt, Prompt: st.Prompt, Vars: st.Vars, Settings: st.Settings, Memory: st.Memory}, nil
}

func (s *Service) runAgent(ctx context.Context, req RunRequest, spec executor.Spec, model executor.Model,
	onEvent func(executor.Event),
) (RunOutcome, error) {
	nb := req.Notebook
	if nb == nil {
		return RunOutcome{}, invalidf("agent mode needs a notebook")
	}
	rec, err := s.startAudit(nb, spec, req.Actor)
	if err != nil {
		return RunOutcome{}, fmt.Errorf("audit: %w", err)
	}
	unlock, err := nb.LockRun(rec.runID)
	if err != nil {
		_ = rec.log.Close()
		return RunOutcome{}, err
	}
	defer unlock()
	st := &storedRun{
		Version: 1, Actor: rec.actor, StartedAt: s.now().UTC().Format(time.RFC3339), Spec: toStored(spec),
		ToolRules: req.ToolRules,
		Check: &agent.Checkpoint{
			RunID: rec.runID, NotebookID: filepath.Base(nb.Dir), ScheduleID: req.ScheduleID, Roots: contextRoots(nb),
		},
	}
	return s.drive(ctx, nb, st, rec, spec, model, req.Detached, onEvent, true)
}

// contextRoots are the enabled context items that exist on disk.
func contextRoots(nb *notebook.Notebook) []string {
	var roots []string
	for _, c := range nb.Manifest.Context {
		if !c.Enabled {
			continue
		}
		if p := nb.ContextPath(c); fileExists(p) {
			roots = append(roots, p)
		}
	}
	return roots
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// drive starts or continues the agent and settles the outcome.
func (s *Service) drive(ctx context.Context, nb *notebook.Notebook, st *storedRun, rec *runAudit,
	spec executor.Spec, model executor.Model, detached bool, onEvent func(executor.Event), start bool,
) (RunOutcome, error) {
	a, closeTools, err := s.buildAgent(ctx, nb, st, model)
	if err != nil {
		_ = rec.log.Close()
		return RunOutcome{}, err
	}
	defer closeTools() // MCP servers stop when the run ends or pauses; resume reconnects
	ctx, emit, done := s.jobTable().track(ctx, st.Check.RunID, detached, onEvent)
	defer done()
	var res executor.Result
	if start {
		res, err = a.Start(ctx, spec, st.Check, emit)
	} else {
		res, err = a.Continue(ctx, spec, st.Check, emit)
	}
	if errors.Is(err, context.Canceled) && !errors.Is(context.Cause(ctx), errUserCancel) {
		return s.interrupt(nb, st, rec, spec, err)
	}
	return s.settle(nb, st, rec, spec, res, err)
}

// interrupt keeps a run that stopped without being cancelled (the core is
// shutting down): its checkpoint stays, marked interrupted, for Resume.
func (s *Service) interrupt(nb *notebook.Notebook, st *storedRun, rec *runAudit, spec executor.Spec,
	runErr error,
) (RunOutcome, error) {
	defer func() { _ = rec.log.Close() }()
	st.Check.Status = notebook.StatusInterrupted
	auditErr := s.auditTools(rec, st)
	_, saveErr := s.saveAgentRun(nb, st, spec, executor.Result{}, nil)
	err := errors.Join(auditErr, nb.SaveCheckpoint(st.Check.RunID, st), saveErr)
	return RunOutcome{RunID: st.Check.RunID, Spec: spec}, errors.Join(runErr, err)
}

// buildAgent assembles the agent: code tools over the run's folders and the
// tools of the notebook's MCP servers. closeTools stops those servers.
func (s *Service) buildAgent(ctx context.Context, nb *notebook.Notebook, st *storedRun, model executor.Model,
) (a *agent.Agent, closeTools func(), err error) {
	pol := agent.Policy{Run: st.ToolRules, Notebook: nb.Manifest.Agent.Tools, App: s.cfg().Agent.Tools}
	if err := pol.Check(); err != nil {
		return nil, nil, &InvalidError{err.Error()}
	}
	var tools []agenttools.Tool
	var existing []string // a folder removed while the run was paused is skipped, not fatal
	for _, p := range st.Check.Roots {
		if fileExists(p) {
			existing = append(existing, p)
		}
	}
	if len(existing) > 0 {
		roots, err := agenttools.NewRoots(existing...)
		if err != nil {
			return nil, nil, fmt.Errorf("agent folders: %w", err)
		}
		tools = append(agenttools.CodeTools(roots),
			agenttools.RunCommandTool(roots, nb.Manifest.Agent.Commands, s.commandEnv()))
	}
	remote, closeTools, err := s.mcpTools(ctx, nb)
	if err != nil {
		return nil, nil, err
	}
	tools = append(tools, remote...)
	lim := s.cfg().Agent.WithDefaults()
	return &agent.Agent{
		Model: model, Tools: agenttools.NewRegistry(tools...), Policy: pol, Now: s.now,
		Limits: agent.Limits{
			MaxSteps: lim.MaxSteps, MaxTokens: lim.MaxTokens, MaxWall: time.Duration(lim.MaxMinutes) * time.Minute,
			MaxToolCalls: lim.MaxToolCalls,
		},
		Save:   func(*agent.Checkpoint) error { return nb.SaveCheckpoint(st.Check.RunID, st) },
		Redact: s.redactor(),
	}, closeTools, nil
}

// redactor removes the configured provider keys and credential-shaped text.
func (s *Service) redactor() func(string) string {
	var known []string
	for _, p := range s.cfg().Providers {
		if p.APIKeyRef == "" || s.Secrets == nil {
			continue
		}
		if v, err := s.Secrets.Get(p.APIKeyRef); err == nil && v != "" {
			known = append(known, v)
		}
	}
	return func(text string) string { return secrets.RedactPatterns(secrets.Redact(text, known...)) }
}

// settle records the outcome: a paused run keeps its checkpoint; a finished,
// failed or cancelled one gets its run record and output, and loses it.
func (s *Service) settle(nb *notebook.Notebook, st *storedRun, rec *runAudit, spec executor.Spec,
	res executor.Result, runErr error,
) (RunOutcome, error) {
	cp := st.Check
	auditErr := s.auditTools(rec, st)
	out := RunOutcome{RunID: cp.RunID, Result: res, Spec: spec}
	if errors.Is(runErr, agent.ErrPaused) {
		defer func() { _ = rec.log.Close() }()
		_, saveErr := s.saveAgentRun(nb, st, spec, res, nil)
		if err := errors.Join(auditErr, nb.SaveCheckpoint(cp.RunID, st), saveErr); err != nil {
			return out, err
		}
		out.Paused = s.pendingOf(nb, st)
		for _, p := range out.Paused {
			s.emit(nb, cp, RunApprovalNeeded, p.ApprovalID, p.Tool, "")
		}
		return out, nil
	}
	switch {
	case errors.Is(runErr, context.Canceled):
		cp.Status = notebook.StatusCancelled
	case runErr != nil:
		cp.Status = notebook.StatusFailed
	default:
		cp.Status = notebook.StatusFinished
	}
	var err error
	out.OutputID, err = s.saveAgentRun(nb, st, spec, res, runErr)
	started, _ := time.Parse(time.RFC3339, st.StartedAt)
	finishErr := rec.finish(res, runErr, out.OutputID, s.now().Sub(started))
	if err := errors.Join(auditErr, err, finishErr, nb.RemoveCheckpoint(cp.RunID)); err != nil {
		return out, errors.Join(runErr, err)
	}
	s.emit(nb, cp, map[string]string{
		notebook.StatusCancelled: RunCancelled, notebook.StatusFailed: RunFailed, notebook.StatusFinished: RunFinished,
	}[cp.Status], "", "", errText(runErr))
	if runErr != nil {
		return out, runErr
	}
	out.Markdown, err = render.Markdown(res.Data, RequiredOrder(spec.WorkType))
	return out, err
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
