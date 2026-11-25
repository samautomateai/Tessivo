package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// WorkTypeSuggestion is a work type the sources point to, and why (WT-7);
// empty when there is none.
type WorkTypeSuggestion struct {
	WorkType string `json:"workType,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

var (
	codeMarkers = []string{"go.mod", "package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", ".git"}
	sheetExts   = map[string]bool{".csv": true, ".tsv": true, ".xlsx": true, ".xls": true}
	meetingExts = map[string]bool{".vtt": true, ".srt": true}
	meetingName = []string{"meeting", "minutes", "notes", "transcript", "standup"}
	docsFolders = map[string]bool{"docs": true, "doc": true, "documentation": true, "wiki": true, "handbook": true}
)

// SuggestWorkType looks at the enabled sources by fixed rules (no model call)
// and suggests a work type, but only while the notebook has no outputs and
// when it differs from the notebook's own.
func (s *Service) SuggestWorkType(id string) (WorkTypeSuggestion, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return WorkTypeSuggestion{}, err
	}
	return SuggestFor(nb), nil
}

func hasOutputs(nb *notebook.Notebook) bool {
	outs, err := nb.ListOutputs()
	return err == nil && len(outs) > 0
}

// suggestFrom applies the rules in order: code or changes, meetings, sheets, docs.
func suggestFrom(nb *notebook.Notebook) WorkTypeSuggestion {
	var total, sheets int
	var meeting, docs bool
	for _, c := range nb.Manifest.Context {
		if !c.Enabled || c.NeedsRelink || c.IsMCP() || c.IsURL() {
			continue
		}
		total++
		switch kind, sug := classifySource(nb, c); kind {
		case srcChanges:
			return sug
		case srcMeeting:
			meeting = true
		case srcSheet:
			sheets++
		case srcDocs:
			docs = true
		}
	}
	switch {
	case meeting:
		return WorkTypeSuggestion{worktype.Summarize, "the sources look like meeting notes or transcripts"}
	case total > 0 && sheets*2 > total:
		return WorkTypeSuggestion{worktype.Report, "most sources are spreadsheets"}
	case docs:
		return WorkTypeSuggestion{worktype.Document, "the sources include a documentation folder"}
	}
	return WorkTypeSuggestion{}
}

// What a source looks like to the rules.
const (
	srcOther = iota
	srcChanges
	srcMeeting
	srcSheet
	srcDocs
)

// classifySource says what one source looks like; for changes and code it also gives the suggestion.
func classifySource(nb *notebook.Notebook, c notebook.ContextItem) (int, WorkTypeSuggestion) {
	path := nb.ContextPath(c)
	ext := strings.ToLower(filepath.Ext(c.Label))
	switch {
	case c.IsGitDiff():
		return srcChanges, WorkTypeSuggestion{worktype.Analyze, "the sources include git changes"}
	case ext == ".diff" || ext == ".patch":
		return srcChanges, WorkTypeSuggestion{worktype.Analyze, fmt.Sprintf("%s is a set of changes", c.Label)}
	case c.InPlace && isCodeBase(path):
		return srcChanges, WorkTypeSuggestion{worktype.Analyze, fmt.Sprintf("%s is a code base", c.Label)}
	case meetingExts[ext] || containsAny(strings.ToLower(c.Label), meetingName):
		return srcMeeting, WorkTypeSuggestion{}
	case sheetExts[ext]:
		return srcSheet, WorkTypeSuggestion{}
	case c.InPlace && docsFolders[strings.ToLower(filepath.Base(path))]:
		return srcDocs, WorkTypeSuggestion{}
	}
	return srcOther, WorkTypeSuggestion{}
}

// isCodeBase reports whether a folder has a project file at its top (a few stats only).
func isCodeBase(path string) bool {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return false
	}
	for _, m := range codeMarkers {
		if _, err := os.Stat(filepath.Join(path, m)); err == nil {
			return true
		}
	}
	return false
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// checkWorkTypeChange validates a new work type; it may change only while the notebook has no outputs.
func checkWorkTypeChange(nb *notebook.Notebook, wt *string) error {
	if wt == nil || *wt == nb.Manifest.WorkType {
		return nil
	}
	if _, err := worktype.Get(*wt); err != nil {
		return &InvalidError{err.Error()}
	}
	if hasOutputs(nb) {
		return fmt.Errorf("%w: the work type can change only while the notebook has no outputs", ErrConflict)
	}
	return nil
}

// SuggestFor is SuggestWorkType for an open notebook (the CLI opens folders by path).
func SuggestFor(nb *notebook.Notebook) WorkTypeSuggestion {
	if hasOutputs(nb) {
		return WorkTypeSuggestion{}
	}
	if sug := suggestFrom(nb); sug.WorkType != nb.Manifest.WorkType {
		return sug
	}
	return WorkTypeSuggestion{}
}

// ChangeWorkType sets an open notebook's work type, only while it has no outputs.
func ChangeWorkType(nb *notebook.Notebook, wt string) error {
	if err := checkWorkTypeChange(nb, &wt); err != nil {
		return err
	}
	return nb.Update(func(m *notebook.Manifest) error {
		m.WorkType = wt
		return nil
	})
}
