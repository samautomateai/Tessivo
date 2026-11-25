package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// Integration kinds: issue trackers, wikis and folders outputs are sent to (REP-4, REP-5).
const (
	IntegrationGitHub     = "github"
	IntegrationGitLab     = "gitlab"
	IntegrationJira       = "jira"
	IntegrationConfluence = "confluence"
	IntegrationNotion     = "notion"
	IntegrationFolder     = "folder"
)

// MaxIntegrations bounds the integrations in a config.
const MaxIntegrations = 32

// Integration is a place outputs are delivered to. The token is never stored
// here: TokenRef names a secret in the OS keychain or the environment.
type Integration struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	BaseURL  string `json:"base_url,omitempty"`  // default for github and notion; required otherwise
	Project  string `json:"project,omitempty"`   // github owner/repo, gitlab path or ID, jira project key
	TokenRef string `json:"token_ref,omitempty"` // required except for folders
	// User makes Jira and Confluence use Basic auth (Atlassian Cloud: the account e-mail); empty: Bearer.
	User      string `json:"user,omitempty"`
	IssueType string `json:"issue_type,omitempty"` // jira; default Task
	Space     string `json:"space,omitempty"`      // confluence space key
	Parent    string `json:"parent,omitempty"`     // confluence parent page ID (optional); notion parent page ID
	Path      string `json:"path,omitempty"`       // folder: an absolute path
	// AllowInsecureHTTP permits plain http:// to a non-loopback host.
	AllowInsecureHTTP bool `json:"allow_insecure_http,omitempty"`
}

var (
	githubRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
	gitlabProj = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+){0,9}$`)
	jiraKey    = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)
	spaceKey   = regexp.MustCompile(`^[A-Za-z0-9~_-]{1,64}$`)
	pageID     = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	issueType  = regexp.MustCompile(`^[\p{L}\p{N} _-]{1,64}$`)
)

// URL is the base URL with the kind's default filled in, without a trailing slash.
func (in Integration) URL() string {
	u := strings.TrimRight(in.BaseURL, "/")
	if u != "" {
		return u
	}
	switch in.Kind {
	case IntegrationGitHub:
		return "https://api.github.com"
	case IntegrationNotion:
		return "https://api.notion.com"
	}
	return ""
}

// ForIssues reports whether the integration takes issues (otherwise pages or files).
func (in Integration) ForIssues() bool {
	return in.Kind == IntegrationGitHub || in.Kind == IntegrationGitLab || in.Kind == IntegrationJira
}

// Check validates one integration.
func (in Integration) Check() error {
	if !mcpName.MatchString(in.Name) {
		return fmt.Errorf("integration name %q: use 1-32 lower-case letters, digits, '-' or '_'", in.Name)
	}
	if in.Kind == IntegrationFolder {
		if !filepath.IsAbs(in.Path) {
			return fmt.Errorf("integration %s: path must be an absolute folder", in.Name)
		}
		return nil
	}
	if err := in.checkRemote(); err != nil {
		return fmt.Errorf("integration %s: %w", in.Name, err)
	}
	return nil
}

func (in Integration) checkRemote() error {
	if err := in.checkKind(); err != nil {
		return err
	}
	if !secretRef.MatchString(in.TokenRef) {
		return errors.New("token_ref must name the secret that holds the token")
	}
	raw := in.URL()
	u, err := url.Parse(raw)
	if raw == "" || err != nil || u.Host == "" || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS) {
		return errors.New("base_url must be an http:// or https:// URL")
	}
	if u.User != nil || u.RawQuery != "" {
		return errors.New("base_url must not carry credentials or a query; use token_ref")
	}
	if u.Scheme == schemeHTTP && !in.AllowInsecureHTTP && !isLoopback(u.Hostname()) {
		return fmt.Errorf("plain http to %s would send the token unencrypted; use https", u.Hostname())
	}
	if len(in.User) > 254 || strings.ContainsAny(in.User, ":\r\n") {
		return errors.New("user must be an account name or e-mail without ':'")
	}
	return nil
}

func (in Integration) checkKind() error {
	switch in.Kind {
	case IntegrationGitHub:
		if !githubRepo.MatchString(in.Project) {
			return errors.New("project must be owner/repo")
		}
	case IntegrationGitLab:
		if !gitlabProj.MatchString(in.Project) {
			return errors.New("project must be a project path (group/name) or ID")
		}
	case IntegrationJira:
		if !jiraKey.MatchString(in.Project) {
			return errors.New("project must be a Jira project key such as OPS")
		}
		if in.IssueType != "" && !issueType.MatchString(in.IssueType) {
			return errors.New("issue_type must be an issue type name such as Task")
		}
	case IntegrationConfluence:
		if !spaceKey.MatchString(in.Space) {
			return errors.New("space must be a Confluence space key")
		}
		if in.Parent != "" && !pageID.MatchString(in.Parent) {
			return errors.New("parent must be a page ID")
		}
	case IntegrationNotion:
		if !pageID.MatchString(in.Parent) {
			return errors.New("parent must be the ID of the Notion page to publish under")
		}
	default:
		return fmt.Errorf("kind %q: want github, gitlab, jira, confluence, notion or folder", in.Kind)
	}
	return nil
}

// CheckIntegrations validates a list: each entry, unique names, at most MaxIntegrations.
func CheckIntegrations(list []Integration) error {
	if len(list) > MaxIntegrations {
		return fmt.Errorf("integrations: at most %d", MaxIntegrations)
	}
	var errs []error
	seen := map[string]bool{}
	for _, in := range list {
		if seen[in.Name] {
			errs = append(errs, fmt.Errorf("integrations: duplicate name %q", in.Name))
		}
		seen[in.Name] = true
		errs = append(errs, in.Check())
	}
	return errors.Join(errs...)
}

// FindIntegration returns the integration with this name.
func FindIntegration(list []Integration, name string) (Integration, bool) {
	for _, in := range list {
		if in.Name == name {
			return in, true
		}
	}
	return Integration{}, false
}
