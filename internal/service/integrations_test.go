package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

func TestIntegrationsAddListRemove(t *testing.T) {
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	s.Secrets = secrets.Map{"GH_TOKEN": "t0ken"}
	gh := config.Integration{Name: "gh", Kind: config.IntegrationGitHub, Project: "acme/shop", TokenRef: "GH_TOKEN"}
	info, err := s.AddIntegration(gh)
	if err != nil || !info.TokenSet {
		t.Fatalf("add: %v %+v", err, info)
	}
	if _, err := s.AddIntegration(gh); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := s.AddIntegration(config.Integration{Name: "x", Kind: "trello"}); !IsInvalid(err) {
		t.Fatalf("invalid: %v", err)
	}
	jira := config.Integration{
		Name: "jira", Kind: config.IntegrationJira, BaseURL: "https://j.test", Project: "OPS",
		TokenRef: "JIRA_TOKEN",
	}
	if info, err := s.AddIntegration(jira); err != nil || info.TokenSet {
		t.Fatalf("a missing token shows as not set: %v %+v", err, info)
	}
	if list := s.ListIntegrations(); len(list) != 2 || list[0].Name != "gh" {
		t.Fatalf("list: %+v", list)
	}
	saved, err := config.Load(s.ConfigPath)
	if err != nil || len(saved.Integrations) != 2 {
		t.Fatalf("saved: %v %+v", err, saved.Integrations)
	}
	if err := s.RemoveIntegration("gh"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveIntegration("gh"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove twice: %v", err)
	}
	if _, err := s.integration("jira"); err != nil {
		t.Fatal(err)
	}
}
