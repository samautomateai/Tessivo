package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestProfilesSaveUseDelete(t *testing.T) {
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	if _, err := s.SaveProfile("work"); err != nil {
		t.Fatal(err)
	}
	home := s.Settings()
	home.Providers = []config.Provider{{Name: "local", BaseURL: "http://localhost:11434/v1"}}
	home.Aliases = []config.Alias{}
	home.Defaults.Model = "local/llama"
	if _, err := s.UpdateSettings(SettingsUpdate{
		Providers: home.Providers, Aliases: home.Aliases,
		Defaults: home.Defaults, Limits: home.Limits,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveProfile("home"); err != nil {
		t.Fatal(err)
	}
	info, err := s.UseProfile("work")
	if err != nil || info.Active != "work" || s.Settings().Defaults.Model != "litellm/m" {
		t.Fatalf("use work: %v %+v %s", err, info, s.Settings().Defaults.Model)
	}
	// An edit to the active profile is kept when switching away and back.
	cur := s.Settings()
	cur.Limits.MaxTokensPerDay = 12345
	if _, err := s.UpdateSettings(SettingsUpdate{
		Providers: cur.Providers, Aliases: cur.Aliases,
		Defaults: cur.Defaults, Limits: cur.Limits,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UseProfile("home"); err != nil || s.Settings().Defaults.Model != "local/llama" {
		t.Fatalf("use home: %v", err)
	}
	if _, err := s.UseProfile("work"); err != nil || s.Settings().Limits.MaxTokensPerDay != 12345 {
		t.Fatalf("the edit came back with work: %v %+v", err, s.Settings().Limits)
	}
	saved, _ := config.Load(s.ConfigPath)
	if saved.Profile != "work" || len(saved.Profiles) != 2 {
		t.Fatalf("saved: %q %d", saved.Profile, len(saved.Profiles))
	}
	if _, err := s.UseProfile("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := s.SaveProfile("Bad Name"); !IsInvalid(err) {
		t.Fatalf("bad name: %v", err)
	}
	info, err = s.DeleteProfile("work")
	if err != nil || info.Active != "" || len(info.Profiles) != 1 || s.Settings().Defaults.Model != "litellm/m" {
		t.Fatalf("deleting the active one keeps the settings: %v %+v", err, info)
	}
}

func TestUseProfileRefusesAnInvalidOne(t *testing.T) {
	s := newSvc(t, summary)
	broken := []config.Provider{{Name: "x", BaseURL: "ftp://x"}}
	s.Config.Profiles = []config.Profile{{Name: "broken", Providers: broken}}
	before := s.Settings().Defaults.Model
	if _, err := s.UseProfile("broken"); !IsInvalid(err) || s.Settings().Defaults.Model != before {
		t.Fatalf("an invalid profile changes nothing: %v", err)
	}
}
