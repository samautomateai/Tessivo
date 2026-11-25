package service

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/samautomateai/tessivo/internal/config"
)

// IntegrationInfo is an integration with whether its token is available
// (never the token itself).
type IntegrationInfo struct {
	config.Integration
	TokenSet bool `json:"tokenSet"`
}

// ListIntegrations returns the configured integrations, in config order.
func (s *Service) ListIntegrations() []IntegrationInfo {
	list := s.cfg().Integrations
	out := make([]IntegrationInfo, 0, len(list))
	for _, in := range list {
		out = append(out, s.integrationInfo(in))
	}
	return out
}

func (s *Service) integrationInfo(in config.Integration) IntegrationInfo {
	info := IntegrationInfo{Integration: in, TokenSet: in.Kind == config.IntegrationFolder}
	if in.TokenRef != "" && s.Secrets != nil {
		if v, err := s.Secrets.Get(in.TokenRef); err == nil && v != "" {
			info.TokenSet = true
		}
	}
	return info
}

// AddIntegration adds an integration; the name must be new.
func (s *Service) AddIntegration(in config.Integration) (IntegrationInfo, error) {
	if err := in.Check(); err != nil {
		return IntegrationInfo{}, &InvalidError{err.Error()}
	}
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if _, ok := config.FindIntegration(s.Config.Integrations, in.Name); ok {
		return IntegrationInfo{}, fmt.Errorf("%w: integration %q already exists", ErrConflict, in.Name)
	}
	next := append(slices.Clone(s.Config.Integrations), in)
	if err := config.CheckIntegrations(next); err != nil {
		return IntegrationInfo{}, &InvalidError{err.Error()}
	}
	if err := s.saveIntegrations(next); err != nil {
		return IntegrationInfo{}, err
	}
	return s.integrationInfo(in), nil
}

// RemoveIntegration deletes an integration; what was already sent stays where it went.
func (s *Service) RemoveIntegration(name string) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	next := slices.DeleteFunc(slices.Clone(s.Config.Integrations), func(in config.Integration) bool {
		return in.Name == name
	})
	if len(next) == len(s.Config.Integrations) {
		return fmt.Errorf("integration %q: %w", name, ErrNotFound)
	}
	return s.saveIntegrations(next)
}

// integration returns a configured integration by name.
func (s *Service) integration(name string) (config.Integration, error) {
	in, ok := config.FindIntegration(s.cfg().Integrations, name)
	if !ok {
		return config.Integration{}, fmt.Errorf("integration %q: %w", name, ErrNotFound)
	}
	return in, nil
}

// saveIntegrations writes the list to the config file and the live config; s.cfgMu is held.
func (s *Service) saveIntegrations(next []config.Integration) error {
	if s.ConfigPath != "" {
		f, err := config.Load(s.ConfigPath)
		if errors.Is(err, fs.ErrNotExist) {
			f, err = config.Default(), nil
		}
		if err != nil {
			return fmt.Errorf("integrations: read config: %w", err)
		}
		f.Integrations = next
		if err := f.Save(s.ConfigPath); err != nil {
			return fmt.Errorf("integrations: save config: %w", err)
		}
	}
	s.Config.Integrations = next
	return nil
}
