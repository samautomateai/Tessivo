package config

import (
	"strings"
	"testing"
)

func TestIntegrationCheck(t *testing.T) {
	abs := t.TempDir()
	good := []Integration{
		{Name: "gh", Kind: IntegrationGitHub, Project: "acme/shop", TokenRef: "GH_TOKEN"},
		{Name: "ghe", Kind: IntegrationGitHub, BaseURL: "https://git.acme.local/api/v3", Project: "a/b", TokenRef: "T"},
		{Name: "gl", Kind: IntegrationGitLab, BaseURL: "https://gitlab.acme.local", Project: "team/app", TokenRef: "T"},
		{
			Name: "jira", Kind: IntegrationJira, BaseURL: "https://acme.atlassian.net", Project: "OPS", TokenRef: "T",
			User: "me@acme.test",
		},
		{
			Name: "wiki", Kind: IntegrationConfluence, BaseURL: "https://acme.atlassian.net/wiki", Space: "ENG",
			TokenRef: "T", User: "me@acme.test",
		},
		{Name: "notes", Kind: IntegrationNotion, Parent: "0123456789abcdef0123456789abcdef", TokenRef: "T"},
		{Name: "share", Kind: IntegrationFolder, Path: abs},
		{Name: "local", Kind: IntegrationGitLab, BaseURL: "http://localhost:8080", Project: "x/y", TokenRef: "T"},
	}
	for _, in := range good {
		if err := in.Check(); err != nil {
			t.Errorf("%s: %v", in.Name, err)
		}
	}
	gl := func(base string) Integration {
		return Integration{Name: "x", Kind: IntegrationGitLab, BaseURL: base, Project: "a", TokenRef: "T"}
	}
	bad := map[string]Integration{
		"name":        {Name: "Bad Name", Kind: IntegrationGitHub, Project: "a/b", TokenRef: "T"},
		"kind":        {Name: "x", Kind: "trello", Project: "a/b", TokenRef: "T"},
		"plain http":  gl("http://gitlab.acme.test"),
		"credentials": gl("https://u:p@gitlab.test"),
		"no token":    {Name: "x", Kind: IntegrationGitHub, Project: "a/b"},
		"bad ref":     {Name: "x", Kind: IntegrationGitHub, Project: "a/b", TokenRef: "has space"},
		"github repo": {Name: "x", Kind: IntegrationGitHub, Project: "just-a-name", TokenRef: "T"},
		"jira key": {
			Name: "x", Kind: IntegrationJira, BaseURL: "https://j.test", Project: "ops team",
			TokenRef: "T",
		},
		"no base":     {Name: "x", Kind: IntegrationJira, Project: "OPS", TokenRef: "T"},
		"no space":    {Name: "x", Kind: IntegrationConfluence, BaseURL: "https://c.test", TokenRef: "T"},
		"notion page": {Name: "x", Kind: IntegrationNotion, Parent: "../x", TokenRef: "T"},
		"rel folder":  {Name: "x", Kind: IntegrationFolder, Path: "reports"},
	}
	for what, in := range bad {
		if err := in.Check(); err == nil {
			t.Errorf("%s: want an error", what)
		}
	}
}

func TestIntegrationsInTheConfig(t *testing.T) {
	f := Default()
	f.Integrations = []Integration{
		{Name: "gh", Kind: IntegrationGitHub, Project: "a/b", TokenRef: "T"},
		{Name: "gh", Kind: IntegrationGitHub, Project: "c/d", TokenRef: "T"},
	}
	if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate names: %v", err)
	}
	if _, ok := FindIntegration(f.Integrations, "gh"); !ok {
		t.Fatal("find")
	}
	if in := (Integration{Kind: IntegrationGitHub}); in.URL() != "https://api.github.com" {
		t.Fatalf("github default: %s", in.URL())
	}
	if !(Integration{Kind: IntegrationJira}).ForIssues() || (Integration{Kind: IntegrationNotion}).ForIssues() {
		t.Fatal("ForIssues")
	}
}
