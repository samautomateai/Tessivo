package service

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/keychain"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// SettingsUpdate is the part of the config the desktop edits: providers,
// aliases, run defaults, token limits and desktop preferences. MCP servers
// have their own routes.
type SettingsUpdate struct {
	Providers []config.Provider  `json:"providers"`
	Aliases   []config.Alias     `json:"aliases"`
	Defaults  config.RunSettings `json:"defaults"`
	Limits    config.Limits      `json:"limits"`
	// Desktop is left unchanged when absent (older clients).
	Desktop *config.DesktopConfig `json:"desktop,omitempty"`
}

// UpdateSettings validates the change, writes it to the config file and
// applies it at once; nothing changes when it is refused.
func (s *Service) UpdateSettings(in SettingsUpdate) (config.File, error) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	next := s.Config
	next.Providers, next.Aliases, next.Defaults, next.Limits = in.Providers, in.Aliases, in.Defaults, in.Limits
	if in.Desktop != nil {
		next.Desktop = *in.Desktop
	}
	if next.Providers == nil {
		next.Providers = []config.Provider{}
	}
	if next.Aliases == nil {
		next.Aliases = []config.Alias{}
	}
	if err := next.Validate(); err != nil {
		return config.File{}, &InvalidError{err.Error()}
	}
	if s.ConfigPath != "" {
		f, err := config.Load(s.ConfigPath)
		if errors.Is(err, fs.ErrNotExist) {
			f, err = next, nil
		}
		if err != nil {
			return config.File{}, fmt.Errorf("settings: read config: %w", err)
		}
		f.Providers, f.Aliases, f.Defaults, f.Limits = next.Providers, next.Aliases, next.Defaults, next.Limits
		f.Desktop = next.Desktop
		if err := f.Save(s.ConfigPath); err != nil {
			return config.File{}, &InvalidError{err.Error()}
		}
	}
	s.Config = next
	return next, nil
}

// ErrNoSecretStore means this core has no keychain to write to.
var ErrNoSecretStore = errors.New("no keychain available; set the key with tess secret set or an environment variable")

// SecretStore writes API keys (the OS keychain in tess serve).
type SecretStore interface {
	Get(ref string) (string, error)
	Set(ref, value string) error
	Delete(ref string) error
}

func checkSecret(ref, value string, needValue bool) error {
	if err := keychain.ValidRef(ref); err != nil {
		return &InvalidError{err.Error()}
	}
	if needValue && (value == "" || len(value) > keychain.MaxSecretBytes || strings.ContainsAny(value, "\r\n")) {
		return invalidf("the key must be one line of 1 to %d bytes", keychain.MaxSecretBytes)
	}
	return nil
}

// SetSecret stores an API key; the value is never returned or logged.
func (s *Service) SetSecret(ref, value string) error {
	if err := checkSecret(ref, value, true); err != nil {
		return err
	}
	if s.SecretStore == nil {
		return ErrNoSecretStore
	}
	if err := s.SecretStore.Set(ref, value); err != nil {
		return fmt.Errorf("store %s: %w", ref, errors.New(secrets.RedactPatterns(err.Error())))
	}
	return nil
}

// SecretSet reports whether a key is available (keychain or environment).
func (s *Service) SecretSet(ref string) (bool, error) {
	if err := checkSecret(ref, "", false); err != nil {
		return false, err
	}
	if s.Secrets != nil {
		if v, err := s.Secrets.Get(ref); err == nil && v != "" {
			return true, nil
		}
	}
	if s.SecretStore != nil {
		if v, err := s.SecretStore.Get(ref); err == nil && v != "" {
			return true, nil
		}
	}
	return false, nil
}

// DeleteSecret removes a stored key.
func (s *Service) DeleteSecret(ref string) error {
	if err := checkSecret(ref, "", false); err != nil {
		return err
	}
	if s.SecretStore == nil {
		return ErrNoSecretStore
	}
	if err := s.SecretStore.Delete(ref); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return fmt.Errorf("delete %s: %w", ref, errors.New(secrets.RedactPatterns(err.Error())))
	}
	return nil
}
