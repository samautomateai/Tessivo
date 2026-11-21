package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// runIDPattern matches notebook.NewID values (and nothing with a path in it).
var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// findRun locates the notebook holding a run's record or checkpoint.
func (s *Service) findRun(runID string) (*notebook.Notebook, error) {
	if !runIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("%q: %w", runID, ErrRunNotFound)
	}
	dirs, err := s.notebookDirs()
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		runs := filepath.Join(dir, notebook.RunsDir)
		if fileExists(filepath.Join(runs, runID+".checkpoint.json")) || fileExists(filepath.Join(runs, runID+".json")) {
			return notebook.Open(dir)
		}
	}
	return nil, fmt.Errorf("%s: %w", runID, ErrRunNotFound)
}

// notebookDirs lists NotebookDirs, then the workspace's notebook folders.
func (s *Service) notebookDirs() ([]string, error) {
	dirs := append([]string(nil), s.NotebookDirs...)
	entries, err := os.ReadDir(s.Workspace)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("list workspace: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && idPattern.MatchString(e.Name()) &&
			fileExists(filepath.Join(s.Workspace, e.Name(), notebook.ManifestFile)) {
			dirs = append(dirs, filepath.Join(s.Workspace, e.Name()))
		}
	}
	return dirs, nil
}

func loadStored(nb *notebook.Notebook, runID string) (*storedRun, error) {
	var st storedRun
	if err := nb.LoadCheckpoint(runID, &st); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrRunNotPaused
		}
		return nil, err
	}
	if st.Check == nil {
		return nil, fmt.Errorf("checkpoint %s has no agent state", runID)
	}
	return &st, nil
}

// How far an answer reaches beyond the call it answers.
const (
	RememberRun      = "run"
	RememberNotebook = "notebook"
)

// Answer records the user's answer (allow or deny) to an approval. remember
// "run" applies it to later calls of the same tool in this run; "notebook"
// also writes the rule to the notebook. It does not resume the run.
func (s *Service) Answer(runID, approvalID, answer, remember string) error {
	if answer != "allow" && answer != "deny" {
		return invalidf("answer %q: want allow or deny", answer)
	}
	if remember != "" && remember != RememberRun && remember != RememberNotebook {
		return invalidf("remember %q: want run or notebook", remember)
	}
	nb, err := s.findRun(runID)
	if err != nil {
		return err
	}
	unlock, err := nb.LockRun(runID)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := loadStored(nb, runID)
	if err != nil {
		return err
	}
	rec, ok := findApproval(st.Check, approvalID)
	switch {
	case !ok:
		return fmt.Errorf("%s: %w", approvalID, ErrApprovalNotFound)
	case rec.Answer != "":
		return ErrApprovalAnswered
	case s.expired(rec.RequestedAt):
		return ErrApprovalExpired
	case st.Check.Status != notebook.StatusWaiting:
		return ErrRunNotPaused
	}
	return s.applyAnswer(nb, st, rec, answer, remember)
}

func (s *Service) applyAnswer(nb *notebook.Notebook, st *storedRun, rec *notebook.ApprovalRecord,
	answer, remember string,
) error {
	rec.Answer, rec.AnsweredAt, rec.Remember = answer, s.now().UTC().Format(time.RFC3339), remember
	for i := range st.Check.Pending {
		if st.Check.Pending[i].ApprovalID == rec.ID {
			st.Check.Pending[i].Answer = answer
		}
	}
	rule := config.RuleAllow
	if answer == "deny" {
		rule = config.RuleBlock
	}
	if remember != "" {
		if st.Check.RunRules == nil {
			st.Check.RunRules = map[string]string{}
		}
		st.Check.RunRules[rec.Tool] = rule
	}
	if remember == RememberNotebook {
		if nb.Manifest.Agent.Tools == nil {
			nb.Manifest.Agent.Tools = map[string]string{}
		}
		nb.Manifest.Agent.Tools[rec.Tool] = rule
		if err := nb.Save(); err != nil {
			return err
		}
	}
	if err := nb.SaveCheckpoint(st.Check.RunID, st); err != nil {
		return err
	}
	return s.auditOnce(nb, st, audit.KindToolDecision, map[string]any{
		"approval_id": rec.ID, "tool": rec.Tool, "answer": answer, "remember": remember,
	})
}

func (s *Service) auditOnce(nb *notebook.Notebook, st *storedRun, kind string, data map[string]any) error {
	l, err := audit.Open(s.AuditPath(nb), st.Actor)
	if err != nil {
		return err
	}
	return errors.Join(l.Append(kind, st.Check.RunID, data), l.Close())
}

func (s *Service) expired(requestedAt string) bool {
	t, err := time.Parse(time.RFC3339, requestedAt)
	if err != nil {
		return false
	}
	days := s.Config.Agent.WithDefaults().ApprovalExpiryDays
	return s.now().After(t.Add(time.Duration(days) * 24 * time.Hour))
}

// Resume continues a paused run whose approvals are answered, or an
// interrupted one, in the caller's goroutine.
func (s *Service) Resume(ctx context.Context, runID string, onEvent func(executor.Event)) (RunOutcome, error) {
	nb, err := s.findRun(runID)
	if err != nil {
		return RunOutcome{}, err
	}
	unlock, err := nb.LockRun(runID)
	if err != nil {
		return RunOutcome{}, err
	}
	defer unlock()
	st, err := loadStored(nb, runID)
	if err != nil {
		return RunOutcome{}, err
	}
	if err := resumable(st); err != nil {
		return RunOutcome{}, err
	}
	spec, err := st.Spec.spec()
	if err != nil {
		return RunOutcome{}, err
	}
	model, err := s.modelFor(spec)
	if err != nil {
		return RunOutcome{}, err
	}
	l, err := audit.Open(s.AuditPath(nb), st.Actor)
	if err != nil {
		return RunOutcome{}, fmt.Errorf("audit: %w", err)
	}
	rec := &runAudit{log: l, runID: runID, actor: st.Actor}
	if err := l.Append(audit.KindRunResume, runID, map[string]any{"status": st.Check.Status}); err != nil {
		_ = l.Close()
		return RunOutcome{}, fmt.Errorf("audit: %w", err)
	}
	s.emit(nb, st.Check, RunResumed, "", "", "")
	return s.drive(ctx, nb, st, rec, spec, model, false, onEvent, false)
}

func resumable(st *storedRun) error {
	switch st.Check.Status {
	case notebook.StatusInterrupted, notebook.StatusRunning:
		return nil
	case notebook.StatusWaiting:
		for _, p := range st.Check.Pending {
			if p.ApprovalID != "" && p.Answer == "" {
				return ErrApprovalPending
			}
		}
		return nil
	}
	return ErrRunNotPaused
}

// Cancel stops a run executing in this process, or closes a paused or
// interrupted one as cancelled.
func (s *Service) Cancel(runID string) error {
	if j, ok := s.jobTable().get(runID); ok {
		j.cancel()
		return nil
	}
	nb, err := s.findRun(runID)
	if err != nil {
		return err
	}
	unlock, err := nb.LockRun(runID)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := loadStored(nb, runID)
	if err != nil {
		return err
	}
	return s.close(nb, st, notebook.StatusCancelled, context.Canceled, RunCancelled)
}

// close ends a stored run without running it: run record, checkpoint removed,
// audit and event.
func (s *Service) close(nb *notebook.Notebook, st *storedRun, status string, cause error, kind string) error {
	spec, err := st.Spec.spec()
	if err != nil {
		return err
	}
	st.Check.Status = status
	if _, err := s.saveAgentRun(nb, st, spec, executor.Result{}, cause); err != nil {
		return err
	}
	auditErr := s.auditOnce(nb, st, audit.KindRunFinish, map[string]any{"status": status, "error": cause.Error()})
	if err := errors.Join(auditErr, nb.RemoveCheckpoint(st.Check.RunID)); err != nil {
		return err
	}
	s.emit(nb, st.Check, kind, "", "", cause.Error())
	return nil
}

// Pending lists what waits for the user across the workspace: unanswered
// approvals, and interrupted runs (to resume or cancel). Expired approvals
// fail their run here, so they expire while the core runs, not only at start.
func (s *Service) Pending() ([]PendingApproval, error) {
	out := []PendingApproval{}
	err := s.eachStored(func(nb *notebook.Notebook, st *storedRun) error {
		if closed, err := s.expireIfDue(nb, st); closed || err != nil {
			return err
		}
		switch st.Check.Status {
		case notebook.StatusWaiting:
			out = append(out, s.pendingOf(nb, st)...)
		case notebook.StatusInterrupted:
			out = append(out, PendingApproval{
				RunID: st.Check.RunID, NotebookID: st.Check.NotebookID, NotebookName: nb.Manifest.Name,
				ScheduleID: st.Check.ScheduleID, Status: notebook.StatusInterrupted, RequestedAt: st.Check.UpdatedAt,
			})
		}
		return nil
	})
	return out, err
}

// PendingForSchedule reports whether a paused or interrupted run of the
// schedule blocks its next occurrence, and why.
func (s *Service) PendingForSchedule(notebookID, scheduleID string) (reason string, blocked bool) {
	_ = s.eachStored(func(nb *notebook.Notebook, st *storedRun) error {
		if st.Check.NotebookID != notebookID || st.Check.ScheduleID != scheduleID {
			return nil
		}
		if closed, err := s.expireIfDue(nb, st); closed || err != nil {
			return err
		}
		blocked = true
		reason = "previous run awaiting approval"
		if st.Check.Status == notebook.StatusInterrupted {
			reason = "previous run interrupted; resume or cancel it"
		}
		return nil
	})
	return reason, blocked
}

// expireIfDue fails a waiting run whose oldest unanswered approval expired.
func (s *Service) expireIfDue(nb *notebook.Notebook, st *storedRun) (bool, error) {
	if st.Check.Status != notebook.StatusWaiting {
		return false, nil
	}
	for _, a := range st.Check.Approvals {
		if a.Answer == "" && s.expired(a.RequestedAt) {
			unlock, err := nb.LockRun(st.Check.RunID)
			if err != nil {
				return false, nil //nolint:nilerr // in use elsewhere: the holder or a later pass expires it
			}
			defer unlock()
			return true, s.close(nb, st, notebook.StatusFailed, errors.New("approval expired"), RunFailed)
		}
	}
	return false, nil
}

// Recover runs at startup: runs that were executing become interrupted (they
// resume only when asked), and runs whose approval expired fail.
func (s *Service) Recover() error {
	return s.eachStored(func(nb *notebook.Notebook, st *storedRun) error {
		switch st.Check.Status {
		case notebook.StatusRunning, "":
			unlock, err := nb.LockRun(st.Check.RunID)
			if err != nil {
				return nil //nolint:nilerr // locked: another process is running it
			}
			defer unlock()
			st.Check.Status = notebook.StatusInterrupted
			spec, err := st.Spec.spec()
			if err != nil {
				return err
			}
			_, saveErr := s.saveAgentRun(nb, st, spec, executor.Result{}, nil)
			return errors.Join(saveErr, nb.SaveCheckpoint(st.Check.RunID, st))
		case notebook.StatusWaiting:
			_, err := s.expireIfDue(nb, st)
			return err
		}
		return nil
	})
}

// eachStored calls fn for every checkpoint in the workspace.
func (s *Service) eachStored(fn func(*notebook.Notebook, *storedRun) error) error {
	dirs, err := s.notebookDirs()
	if err != nil {
		return err
	}
	var errs []error
	for _, dir := range dirs {
		nb, err := notebook.Open(dir)
		if err != nil {
			continue
		}
		ids, err := nb.ListCheckpoints()
		if err != nil {
			continue
		}
		for _, id := range ids {
			st, err := loadStored(nb, id)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			errs = append(errs, fn(nb, st))
		}
	}
	return errors.Join(errs...)
}

// ResumeCheck reports whether Resume would start now, without running it:
// the run exists, is paused with every approval answered (or interrupted),
// and no process holds it.
func (s *Service) ResumeCheck(runID string) error {
	if _, live := s.jobTable().get(runID); live {
		return fmt.Errorf("run is executing: %w", ErrConflict)
	}
	nb, err := s.findRun(runID)
	if err != nil {
		return err
	}
	st, err := loadStored(nb, runID)
	if err != nil {
		return err
	}
	return resumable(st)
}

// RunStatus is the run's current status: from its checkpoint while one
// exists, else from its run record.
func (s *Service) RunStatus(runID string) (string, error) {
	if _, live := s.jobTable().get(runID); live {
		return notebook.StatusRunning, nil
	}
	nb, err := s.findRun(runID)
	if err != nil {
		return "", err
	}
	if st, err := loadStored(nb, runID); err == nil {
		return st.Check.Status, nil
	}
	r, err := nb.LoadRun(runID)
	if err != nil {
		return "", err
	}
	return r.Status, nil
}
