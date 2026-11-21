package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// MaxMCPServers bounds the configured MCP servers.
const MaxMCPServers = 50

// MCPConfig lists the MCP servers notebooks can attach.
type MCPConfig struct {
	Servers []MCPServer `json:"servers,omitempty"`
}

// MCPServer is one server: a program to start (stdio) or a URL (Streamable
// HTTP). Env and header values are text that may hold ${secret:NAME}
// placeholders; credentials themselves never go in the config.
type MCPServer struct {
	Name              string            `json:"name"`
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	URL               string            `json:"url,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	AllowInsecureHTTP bool              `json:"allow_insecure_http,omitempty"`
}

var (
	mcpName       = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	placeholder   = regexp.MustCompile(`\$\{secret:([^}]*)\}`)
	secretRef     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	sensitiveName = regexp.MustCompile(`(?i)token|key|secret|passw|auth|cookie|credential`)
	schemeOnly    = regexp.MustCompile(`(?i)^\s*(bearer|basic|token)?\s*$`)
)

// Check validates every server and that names are unique.
func (c MCPConfig) Check() error {
	if len(c.Servers) > MaxMCPServers {
		return fmt.Errorf("mcp: at most %d servers", MaxMCPServers)
	}
	var errs []error
	seen := map[string]bool{}
	for _, s := range c.Servers {
		if seen[s.Name] {
			errs = append(errs, fmt.Errorf("mcp server %q: duplicate name", s.Name))
		}
		seen[s.Name] = true
		errs = append(errs, s.Check())
	}
	return errors.Join(errs...)
}

// Check validates one server: its name, exactly one of command or URL, a
// safe URL, and no literal credentials in env or headers.
func (s MCPServer) Check() error {
	if !mcpName.MatchString(s.Name) {
		return fmt.Errorf("mcp server %q: names use a-z, 0-9, '_' and '-' (at most 32)", s.Name)
	}
	if (s.Command == "") == (s.URL == "") {
		return fmt.Errorf("mcp server %q: give exactly one of command or url", s.Name)
	}
	if s.URL != "" {
		u, err := url.Parse(s.URL)
		switch {
		case err != nil || u.Host == "" || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS):
			return fmt.Errorf("mcp server %q: url must be http:// or https://", s.Name)
		case u.User != nil:
			return fmt.Errorf("mcp server %q: url must not embed credentials; use a header", s.Name)
		case u.Scheme == schemeHTTP && !s.AllowInsecureHTTP && !isLoopback(u.Hostname()):
			return fmt.Errorf("mcp server %q: plain http to %s is refused; use https or set allow_insecure_http",
				s.Name, u.Hostname())
		}
	}
	for kind, values := range map[string]map[string]string{"env": s.Env, "header": s.Headers} {
		for k, v := range values {
			if err := checkValue(k, v); err != nil {
				return fmt.Errorf("mcp server %q: %s %s: %w", s.Name, kind, k, err)
			}
		}
	}
	return nil
}

// checkValue refuses malformed placeholders and credentials written as text:
// credential-shaped text anywhere, and for a sensitive name (token, key,
// secret, password, auth) anything besides placeholders and a scheme word.
func checkValue(name, v string) error {
	for _, m := range placeholder.FindAllStringSubmatch(v, -1) {
		if !secretRef.MatchString(m[1]) {
			return fmt.Errorf("invalid secret reference %q", m[1])
		}
	}
	literal := placeholder.ReplaceAllString(v, "")
	sensitive := sensitiveName.MatchString(name) && !schemeOnly.MatchString(literal)
	if secrets.RedactPatterns(literal) != literal || sensitive {
		return errors.New("looks like a credential; store it as a secret and use ${secret:NAME}")
	}
	return nil
}

// Find returns the server named name.
func (c MCPConfig) Find(name string) (MCPServer, bool) {
	for _, s := range c.Servers {
		if s.Name == name {
			return s, true
		}
	}
	return MCPServer{}, false
}

// ExpandSecrets replaces each ${secret:NAME} with get(NAME). Errors name the
// reference, never a value.
func ExpandSecrets(v string, get func(ref string) (string, error)) (string, error) {
	var firstErr error
	out := placeholder.ReplaceAllStringFunc(v, func(m string) string {
		ref := strings.TrimSuffix(strings.TrimPrefix(m, "${secret:"), "}")
		if !secretRef.MatchString(ref) {
			firstErr = errors.Join(firstErr, fmt.Errorf("invalid secret reference %q", ref))
			return ""
		}
		val, err := get(ref)
		if err != nil {
			firstErr = errors.Join(firstErr, fmt.Errorf("secret %s is not set", ref))
			return ""
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// URL schemes the config accepts.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)
