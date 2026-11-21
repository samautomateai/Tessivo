package service

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
)

// AuditFile is the audit log name inside a notebook.
const AuditFile = "audit.jsonl"

// AuditPath is the notebook's log, or the user-level log without a notebook.
func (s *Service) AuditPath(nb *notebook.Notebook) string {
	if nb != nil {
		return filepath.Join(nb.Dir, AuditFile)
	}
	return s.UserAuditPath
}

type runAudit struct {
	log   *audit.Log
	runID string
	actor string
}

func (s *Service) startAudit(nb *notebook.Notebook, spec executor.Spec, actor string) (*runAudit, error) {
	if actor == "" {
		actor = s.Actor
	}
	if actor == "" {
		actor = defaultActor
	}
	l, err := audit.Open(s.AuditPath(nb), actor)
	if err != nil {
		return nil, err
	}
	a := &runAudit{log: l, runID: notebook.NewID(), actor: actor}
	sources := make([]string, 0, len(spec.Documents))
	for _, d := range spec.Documents {
		sources = append(sources, d.Source)
	}
	err = l.Append(audit.KindRunStart, a.runID, map[string]any{
		"work_type": spec.WorkType.ID, "prompt_sha256": audit.Digest(spec.Prompt), "prompt_chars": len(spec.Prompt),
		"sources": sources, "model": spec.Settings.Model, "thinking": spec.Settings.ThinkingEffort,
	})
	if err != nil {
		_ = l.Close()
		return nil, err
	}
	return a, nil
}

func (a *runAudit) finish(res executor.Result, runErr error, outputID string, d time.Duration) error {
	defer func() { _ = a.log.Close() }()
	for _, at := range res.Attempts {
		data := map[string]any{"target": at.Target, "trigger": at.Trigger}
		if at.Err != nil {
			data["error"] = at.Err
		}
		if err := a.log.Append(audit.KindModelAttempt, a.runID, data); err != nil {
			return err
		}
	}
	data := map[string]any{
		"valid": res.Valid, "model_used": res.Model, "strategy": res.Strategy, "total_tokens": res.TotalTokens,
		"duration_ms": d.Milliseconds(),
	}
	if runErr != nil {
		data["error"] = runErr
	}
	if err := a.log.Append(audit.KindRunFinish, a.runID, data); err != nil {
		return err
	}
	if outputID != "" {
		return a.log.Append(audit.KindOutputSaved, a.runID, map[string]any{"output_id": outputID})
	}
	return nil
}

func saveRun(nb *notebook.Notebook, runID string, spec executor.Spec, res executor.Result, runErr error,
	started time.Time, decorate func(*notebook.Run),
) (string, error) {
	run := notebook.Run{
		ID: runID, StartedAt: started.UTC().Format(time.RFC3339), WorkType: spec.WorkType.ID,
		DurationMS: time.Since(started).Milliseconds(), Prompt: spec.Prompt, Vars: spec.Vars, Settings: spec.Settings,
		ModelUsed: res.Model, Strategy: res.Strategy, TotalTokens: res.TotalTokens, Valid: res.Valid,
	}
	for _, d := range spec.Documents {
		run.Context = append(run.Context, d.Source)
	}
	for _, a := range res.Attempts {
		ar := notebook.AttemptRecord{Target: a.Target, Trigger: a.Trigger}
		if a.Err != nil {
			ar.Error = a.Err.Error()
		}
		run.Attempts = append(run.Attempts, ar)
	}
	if runErr != nil {
		run.Error = runErr.Error()
	}
	if res.Valid {
		out := notebook.Output{
			ID: notebook.NewID(), RunID: run.ID, WorkType: spec.WorkType.ID,
			Title: titleOf(res.Data), CreatedAt: run.StartedAt, Data: res.Data,
		}
		if r := spec.Revise; r != nil {
			out.ParentID, out.FromThread = r.ParentID, r.MessageID
		}
		md, _ := render.Markdown(res.Data, RequiredOrder(spec.WorkType))
		if err := nb.SaveOutput(out, md); err != nil {
			return "", err
		}
		run.OutputID = out.ID
	}
	if decorate != nil {
		decorate(&run)
	}
	return run.OutputID, nb.SaveRun(run)
}

func titleOf(data json.RawMessage) string {
	var v struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal(data, &v)
	return v.Title
}

// defaultActor names the core in audit entries when no actor is set.
const defaultActor = "core"
