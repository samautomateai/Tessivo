package service

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// ListPrompts returns a notebook's saved prompts, sorted by name.
func (s *Service) ListPrompts(id string) ([]notebook.Prompt, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return nil, err
	}
	names, err := nb.ListPrompts()
	if err != nil {
		return nil, err
	}
	out := make([]notebook.Prompt, 0, len(names))
	for _, n := range names {
		if p, err := nb.LoadPrompt(n); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// SavePrompt creates or replaces a saved prompt.
func (s *Service) SavePrompt(id string, p notebook.Prompt) (notebook.Prompt, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return notebook.Prompt{}, err
	}
	if err := p.Check(); err != nil {
		return notebook.Prompt{}, &InvalidError{err.Error()}
	}
	return p, nb.SavePrompt(p)
}

// DeletePrompt removes a saved prompt; one a schedule runs is a conflict.
func (s *Service) DeletePrompt(id, name string) error {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return err
	}
	list, err := nb.Schedules()
	if err != nil {
		return err
	}
	for _, sch := range list {
		if sch.PromptName == name {
			return fmt.Errorf("%w: schedule %s runs prompt %q; change or remove it first", ErrConflict, sch.ID, name)
		}
	}
	if err := nb.DeletePrompt(name); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("prompt %s: %w", name, ErrNotFound)
		}
		return &InvalidError{err.Error()}
	}
	return nil
}
