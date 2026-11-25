package service

import (
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

// sortPinnedFirst orders pinned items first, each group by name (ignoring case).
func sortPinnedFirst[T any](items []T, key func(T) (bool, string)) {
	sort.SliceStable(items, func(i, j int) bool {
		pi, ni := key(items[i])
		pj, nj := key(items[j])
		if pi != pj {
			return pi
		}
		return strings.ToLower(ni) < strings.ToLower(nj)
	})
}

// ListNotebooksTagged lists the workspace notebooks that have tag (any case).
func (s *Service) ListNotebooksTagged(tag string) ([]NotebookInfo, error) {
	all, err := s.ListNotebooks()
	if err != nil {
		return nil, err
	}
	out := []NotebookInfo{}
	for _, n := range all {
		if (notebook.Manifest{Tags: n.Tags}).HasTag(tag) {
			out = append(out, n)
		}
	}
	return out, nil
}

// OrganizeResearch sets a research notebook's tags or pin; nil fields stay.
func (s *Service) OrganizeResearch(id string, tags *[]string, pinned *bool) (ResearchInfo, error) {
	var clean []string
	if tags != nil {
		var err error
		if clean, err = notebook.NormalizeTags(*tags); err != nil {
			return ResearchInfo{}, &InvalidError{err.Error()}
		}
	}
	nb, err := s.updateResearch(id, func(nb *research.Notebook) error {
		if tags != nil {
			nb.Manifest.Tags = clean
		}
		if pinned != nil {
			nb.Manifest.Pinned = *pinned
		}
		return nb.Save()
	})
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(nb), nil
}
