package service

import (
	"os"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// SetNotebookCommands replaces a notebook's run_command allow-list.
func (s *Service) SetNotebookCommands(notebookID string, rules []config.CommandRule) (NotebookInfo, error) {
	if err := config.CheckCommandRules(rules); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if err := nb.Update(func(m *notebook.Manifest) error { m.Agent.Commands = rules; return nil }); err != nil {
		return NotebookInfo{}, err
	}
	return describe(notebookID, nb, false), nil
}

// commandEnv is the environment for run_command: the core's own, without
// secret-looking variables or the configured key names.
func (s *Service) commandEnv() []string {
	var keys []string
	for _, p := range s.cfg().Providers {
		if p.APIKeyRef != "" {
			keys = append(keys, p.APIKeyRef)
		}
	}
	return agenttools.CommandEnv(os.Environ(), keys)
}
