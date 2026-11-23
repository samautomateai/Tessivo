package service

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/textdiff"
)

// OutputDiff compares two versions of an output line by line.
type OutputDiff struct {
	From   string          `json:"from"` // the older version
	To     string          `json:"to"`
	Lines  []textdiff.Line `json:"lines"`
	Counts textdiff.Counts `json:"counts"`
}

// DiffOutputs compares an output with against, or with the version it
// revised when against is empty.
func (s *Service) DiffOutputs(notebookRef, outputID, against string) (OutputDiff, error) {
	nb, err := s.openNotebookRef(notebookRef)
	if err != nil {
		return OutputDiff{}, err
	}
	to, toMD, err := nb.GetOutput(outputID)
	if err != nil {
		return OutputDiff{}, fmt.Errorf("output %s: %w", outputID, ErrNotFound)
	}
	if against == "" {
		against = to.ParentID
	}
	if against == "" {
		return OutputDiff{}, invalidf("output %s is an original, not a revision; give a version to compare", outputID)
	}
	_, fromMD, err := nb.GetOutput(against)
	if err != nil {
		return OutputDiff{}, fmt.Errorf("output %s: %w", against, ErrNotFound)
	}
	lines := textdiff.Lines(fromMD, toMD)
	return OutputDiff{From: against, To: outputID, Lines: lines, Counts: textdiff.Summary(lines)}, nil
}
