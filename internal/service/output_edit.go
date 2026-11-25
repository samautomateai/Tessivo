package service

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// MaxEditedOutput bounds an edited output's JSON.
const MaxEditedOutput = 1 << 20

// EditOutput saves data, the output's structured result as the user edited
// it, as a new version of the output: checked against the work type's
// schema, rendered, linked to its parent and audited. No model is called and
// the parent is never changed.
func (s *Service) EditOutput(notebookID, outputID string, data json.RawMessage) (OutputInfo, error) {
	if len(data) > MaxEditedOutput {
		return OutputInfo{}, invalidf("the edited output is over %d bytes", MaxEditedOutput)
	}
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return OutputInfo{}, err
	}
	wt, err := worktype.Get(ref.out.WorkType)
	if err != nil {
		return OutputInfo{}, &InvalidError{err.Error()}
	}
	sch, err := wt.ParsedSchema()
	if err != nil {
		return OutputInfo{}, fmt.Errorf("edit output: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return OutputInfo{}, invalidf("the edited output is not JSON: %v", err)
	}
	body := compact.Bytes()
	if wt.ID == worktype.Analyze {
		body = withFindings(body)
	}
	if err := sch.ValidateJSON(body); err != nil {
		return OutputInfo{}, invalidf("the edited output does not fit the %s structure: %v", wt.Name, err)
	}
	if bytes.Equal(body, compactOrSelf(ref.out.Data)) {
		return OutputInfo{}, invalidf("nothing changed")
	}
	out := notebook.Output{
		ID: notebook.NewID(), RunID: ref.out.RunID, WorkType: ref.out.WorkType, Title: titleOf(body),
		CreatedAt: nowRFC3339(), Data: body, ParentID: ref.out.ID,
	}
	md, err := render.Markdown(out.Data, RequiredOrder(wt))
	if err != nil {
		return OutputInfo{}, fmt.Errorf("edit output: %w", err)
	}
	if err := ref.nb.SaveOutput(out, md); err != nil {
		return OutputInfo{}, fmt.Errorf("edit output: %w", err)
	}
	if err := s.auditEdit(ref.nb, out); err != nil {
		return OutputInfo{}, err
	}
	return s.GetOutput(notebookID, out.ID)
}

func compactOrSelf(b []byte) []byte {
	var c bytes.Buffer
	if json.Compact(&c, b) != nil {
		return b
	}
	return c.Bytes()
}

// auditEdit records the edit: the new version, its parent and a digest of
// the content (never the content itself).
func (s *Service) auditEdit(nb *notebook.Notebook, out notebook.Output) error {
	l, err := audit.Open(s.AuditPath(nb), s.actor())
	if err != nil {
		return fmt.Errorf("edit output: %w", err)
	}
	err = l.Append(audit.KindOutputSaved, out.RunID, map[string]any{
		"output": out.ID, "edited_from": out.ParentID, "sha256": audit.Digest(string(out.Data)),
	})
	if cerr := l.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("edit output: %w", err)
	}
	return nil
}
