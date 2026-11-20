package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/template"
)

// TemplateInfo adds parent-update details to a template.
type TemplateInfo struct {
	template.Template
	ParentUpdate   int      `json:"parentUpdate,omitempty"` // newer parent version, if any
	ChangedPrompts []string `json:"changedPrompts,omitempty"`
}

func (s *Service) templateErr(err error) error {
	if errors.Is(err, template.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return &InvalidError{err.Error()}
}

// ListTemplates returns built-in and user templates, optionally by work type.
func (s *Service) ListTemplates(workType string) ([]TemplateInfo, error) {
	list, err := s.Templates.List()
	if err != nil {
		return nil, err
	}
	out := []TemplateInfo{}
	for _, t := range list {
		if workType != "" && t.WorkType != workType {
			continue
		}
		info := TemplateInfo{Template: t}
		info.ParentUpdate, info.ChangedPrompts, _ = s.Templates.ParentUpdate(t)
		out = append(out, info)
	}
	return out, nil
}

// GetTemplate returns one template.
func (s *Service) GetTemplate(id string) (TemplateInfo, error) {
	t, err := s.Templates.Get(id)
	if err != nil {
		return TemplateInfo{}, s.templateErr(err)
	}
	info := TemplateInfo{Template: t}
	info.ParentUpdate, info.ChangedPrompts, _ = s.Templates.ParentUpdate(t)
	return info, nil
}

// CreateFromTemplate creates a workspace notebook set up from a template.
func (s *Service) CreateFromTemplate(templateID, name string) (NotebookInfo, error) {
	t, err := s.Templates.Get(templateID)
	if err != nil {
		return NotebookInfo{}, s.templateErr(err)
	}
	if strings.TrimSpace(name) == "" {
		name = t.Name
	}
	info, err := s.CreateNotebook(name, t.WorkType)
	if err != nil {
		return NotebookInfo{}, err
	}
	nb, err := s.OpenNotebook(info.ID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if err := template.Apply(nb, t); err != nil {
		return NotebookInfo{}, err
	}
	return describe(info.ID, nb, false), nil
}

// SaveAsTemplate captures a workspace notebook as a user template.
func (s *Service) SaveAsTemplate(notebookID, id, name, description string) (TemplateInfo, error) {
	nb, err := s.OpenNotebook(notebookID)
	if err != nil {
		return TemplateInfo{}, err
	}
	if id == "" {
		id = Slug(name)
	}
	t, err := template.FromNotebook(nb, id, name, description)
	if err != nil {
		return TemplateInfo{}, err
	}
	saved, err := s.Templates.Save(t)
	if err != nil {
		return TemplateInfo{}, &InvalidError{err.Error()}
	}
	return TemplateInfo{Template: saved}, nil
}

// ExtendTemplate copies a template under a new id, recording its parent.
func (s *Service) ExtendTemplate(parentID, id, name string) (TemplateInfo, error) {
	parent, err := s.Templates.Get(parentID)
	if err != nil {
		return TemplateInfo{}, s.templateErr(err)
	}
	if id == "" {
		id = Slug(name)
	}
	saved, err := s.Templates.Save(template.Extend(parent, id, name))
	if err != nil {
		return TemplateInfo{}, &InvalidError{err.Error()}
	}
	return TemplateInfo{Template: saved}, nil
}

// DeleteTemplate removes a user template.
func (s *Service) DeleteTemplate(id string) error {
	if err := s.Templates.Delete(id); err != nil {
		return s.templateErr(err)
	}
	return nil
}

// MaxMemoryItem bounds one pinned fact.
const MaxMemoryItem = 1000

// SetMemory replaces a notebook's pinned memory facts.
func (s *Service) SetMemory(notebookID string, facts []string) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if len(facts) > template.MaxMemory {
		return NotebookInfo{}, invalidf("at most %d memory items", template.MaxMemory)
	}
	clean := make([]string, 0, len(facts))
	for _, f := range facts {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if len(f) > MaxMemoryItem {
			return NotebookInfo{}, invalidf("memory items are limited to %d characters", MaxMemoryItem)
		}
		clean = append(clean, f)
	}
	nb.Manifest.Memory = clean
	if err := nb.Save(); err != nil {
		return NotebookInfo{}, err
	}
	return describe(notebookID, nb, false), nil
}
