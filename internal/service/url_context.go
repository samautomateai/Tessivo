package service

import (
	"context"
	"fmt"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/webfetch"
)

// AddURLContext attaches a web page to a notebook; it is fetched on every run.
func (s *Service) AddURLContext(notebookID, rawURL, label string) (NotebookInfo, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if _, err := nb.AddURL(rawURL, label); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	return describe(notebookID, nb, true), nil
}

// AddTextContext saves pasted text as a source of the notebook.
func (s *Service) AddTextContext(notebookID, label, text string) (NotebookInfo, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if _, err := nb.AddText(label, text); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	return describe(notebookID, nb, true), nil
}

// fetchPage reads a URL source now; a page that cannot be read fails the run, naming it.
func fetchPage(ctx context.Context, c notebook.ContextItem) (extract.Document, error) {
	d, err := webfetch.Fetch(ctx, c.URI)
	if err != nil {
		return extract.Document{}, fmt.Errorf("url source %s: %w", c.Label, err)
	}
	return d, nil
}
