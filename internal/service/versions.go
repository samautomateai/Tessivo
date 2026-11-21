package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// maxVersionDepth bounds the walk up a chain of versions.
const maxVersionDepth = 1000

// versions numbers outputs by their revision chain: an original is 1 and each
// revision is its parent's number plus one. Missing parents end the chain;
// cycles and very long chains are cut, so the walk always ends.
func versions(outs []notebook.Output) map[string]int {
	parent := make(map[string]string, len(outs))
	for _, o := range outs {
		parent[o.ID] = o.ParentID
	}
	v := make(map[string]int, len(outs))
	for _, o := range outs {
		n, id, seen := 1, o.ParentID, map[string]bool{o.ID: true}
		for id != "" && !seen[id] && n < maxVersionDepth {
			n++
			seen[id] = true
			next, known := parent[id]
			if !known {
				break
			}
			id = next
		}
		v[o.ID] = n
	}
	return v
}

// NewVersion revises an output from its thread: the parent output and the
// conversation up to fromMessageID (default: the latest reply) go through the
// pipeline with the notebook's current context, memory, prompt and model. The
// result is validated and saved as a new output linked to its parent; the
// parent is never changed. Agent-mode notebooks also use the pipeline.
func (s *Service) NewVersion(ctx context.Context, notebookID, outputID, fromMessageID string) (RunOutcome, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return RunOutcome{}, err
	}
	st, err := s.threadStore(ref.nb)
	if err != nil {
		return RunOutcome{}, err
	}
	release, err := st.Acquire(outputID)
	if err != nil {
		return RunOutcome{}, chatErr(err)
	}
	defer release()
	sess, err := st.Get(outputID)
	if errors.Is(err, chat.ErrNotFound) {
		return RunOutcome{}, invalidf("ask for changes in the output's thread first")
	}
	if err != nil {
		return RunOutcome{}, chatErr(err)
	}
	msgID, err := revisionPoint(sess, fromMessageID)
	if err != nil {
		return RunOutcome{}, err
	}
	req, spec, err := s.versionSpec(ref, sess, msgID)
	if err != nil {
		return RunOutcome{}, err
	}
	model, err := s.modelFor(spec)
	if err != nil {
		return RunOutcome{}, err
	}
	out, err := s.runPipeline(ctx, req, spec, model, nil)
	if err != nil {
		return out, err
	}
	return out, s.noteVersion(st, ref, out.OutputID)
}

// revisionPoint returns the message a version is made from: the given
// assistant reply, or the latest one.
func revisionPoint(sess chat.Session, id string) (string, error) {
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		m := sess.Messages[i]
		if m.Role == chat.RoleAssistant && (id == "" || m.ID == id) {
			return m.ID, nil
		}
	}
	if id != "" {
		return "", invalidf("message %q is not a reply in this thread", id)
	}
	return "", invalidf("ask for changes in the output's thread first")
}

// versionSpec builds the revision run: the prompt the output's run would use
// today, the run's vars, forced to the pipeline.
func (s *Service) versionSpec(ref outputRef, sess chat.Session, msgID string) (RunRequest, executor.Spec, error) {
	prompt := ref.run.Prompt
	tg, err := s.promptTarget(ref.nb, ref.run)
	if err == nil && tg.Current != "" {
		prompt = tg.Current
	}
	req := RunRequest{
		Notebook: ref.nb, Prompt: prompt, Vars: ref.run.Vars, Save: true,
		PromptName: tg.PromptName, ScheduleID: ref.run.ScheduleID,
	}
	spec, err := s.buildSpec(RunRequest{Notebook: ref.nb, Prompt: prompt, Vars: ref.run.Vars})
	if err != nil {
		return req, executor.Spec{}, err
	}
	spec.Settings.Mode = config.ModePipeline
	spec.Revise = &executor.Revision{
		Parent: ref.out.Data, Requests: threadTranscript(sess.Messages, msgID, maxTranscript),
		ParentID: ref.out.ID, MessageID: msgID,
	}
	return req, spec, nil
}

// noteVersion tells the parent thread's reader where the version went.
func (s *Service) noteVersion(st *chat.Store, ref outputRef, outputID string) error {
	outs, err := ref.nb.ListOutputs()
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Saved as v%d (%s)", versions(outs)[outputID], outputID)
	_, err = st.Update(ref.out.ID, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, chat.Message{
			ID: chat.NewMessageID(), Role: chat.RoleNote, Content: text, CreatedAt: nowRFC3339(),
		})
		return nil
	})
	return chatErr(err)
}
