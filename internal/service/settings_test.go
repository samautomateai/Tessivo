package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// memStore is an in-memory keychain.
type memStore map[string]string

func (m memStore) Get(ref string) (string, error) {
	v, ok := m[ref]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}
func (m memStore) Set(ref, v string) error { m[ref] = v; return nil }
func (m memStore) Delete(ref string) error { delete(m, ref); return nil }

func TestUpdateSettingsSavesAndAppliesAtOnce(t *testing.T) {
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	if err := s.cfg().Save(s.ConfigPath); err != nil {
		t.Fatal(err)
	}
	next := s.Settings()
	next.Defaults.Model = "litellm/qwen"
	next.Limits.MaxTokensPerDay = 9000
	got, err := s.UpdateSettings(SettingsUpdate{
		Providers: next.Providers, Aliases: next.Aliases, Defaults: next.Defaults, Limits: next.Limits,
	})
	if err != nil || got.Defaults.Model != "litellm/qwen" || s.cfg().Limits.MaxTokensPerDay != 9000 {
		t.Fatalf("%v %+v", err, got.Defaults)
	}
	saved, err := config.Load(s.ConfigPath)
	if err != nil || saved.Limits.MaxTokensPerDay != 9000 {
		t.Fatalf("not saved: %v %+v", err, saved.Limits)
	}
	bad := next
	bad.Defaults.ThinkingEffort = "extreme"
	if _, err := s.UpdateSettings(SettingsUpdate{
		Providers: bad.Providers, Aliases: bad.Aliases,
		Defaults: bad.Defaults, Limits: bad.Limits,
	}); !IsInvalid(err) {
		t.Fatalf("invalid settings: %v", err)
	}
	if s.cfg().Defaults.ThinkingEffort == "extreme" {
		t.Fatal("refused settings were applied")
	}
}

func TestSecretsAreStoredButNeverReturned(t *testing.T) {
	s := newSvc(t, summary)
	store := memStore{}
	s.SecretStore = store
	if err := s.SetSecret("MY_KEY", "sk-test-123"); err != nil {
		t.Fatal(err)
	}
	if store["MY_KEY"] != "sk-test-123" {
		t.Fatal("not stored")
	}
	if ok, err := s.SecretSet("MY_KEY"); err != nil || !ok {
		t.Fatalf("status: %v %v", ok, err)
	}
	for _, bad := range [][2]string{{"bad name!", "v"}, {"OK", ""}, {"OK", strings.Repeat("x", 9000)}, {"OK", "a\nb"}} {
		if err := s.SetSecret(bad[0], bad[1]); !IsInvalid(err) {
			t.Errorf("SetSecret(%q, %d bytes): %v", bad[0], len(bad[1]), err)
		}
	}
	if err := s.DeleteSecret("MY_KEY"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.SecretSet("MY_KEY"); ok {
		t.Fatal("still set")
	}
	s.SecretStore = nil
	if err := s.SetSecret("MY_KEY", "v"); !errors.Is(err, ErrNoSecretStore) {
		t.Fatalf("no store: %v", err)
	}
}

func TestUpdateSettingsDesktopBlock(t *testing.T) {
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	if err := s.cfg().Save(s.ConfigPath); err != nil {
		t.Fatal(err)
	}
	cur := s.Settings()
	in := SettingsUpdate{Providers: cur.Providers, Aliases: cur.Aliases, Defaults: cur.Defaults, Limits: cur.Limits}
	in.Desktop = &config.DesktopConfig{CloseToTray: true}
	if got, err := s.UpdateSettings(in); err != nil || !got.Desktop.CloseToTray {
		t.Fatalf("%v %+v", err, got.Desktop)
	}
	in.Desktop = nil // an older client leaves the block alone
	if got, err := s.UpdateSettings(in); err != nil || !got.Desktop.CloseToTray {
		t.Fatalf("kept: %v %+v", err, got.Desktop)
	}
	saved, err := config.Load(s.ConfigPath)
	if err != nil || !saved.Desktop.CloseToTray {
		t.Fatalf("not saved: %v %+v", err, saved.Desktop)
	}
}
