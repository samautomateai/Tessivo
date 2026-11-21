package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/mcp"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// mcpTestTimeout bounds TestMCPServer: connect plus three listings.
const mcpTestTimeout = 60 * time.Second

// MCPServerInfo is a configured server as the API shows it.
type MCPServerInfo struct {
	config.MCPServer
}

// MCP's own types stay inside the service; these describe what a server offers.
type (
	// MCPToolInfo is one tool.
	MCPToolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	// MCPResourceInfo is one resource.
	MCPResourceInfo struct {
		URI      string `json:"uri"`
		Name     string `json:"name,omitempty"`
		MimeType string `json:"mimeType,omitempty"`
	}
	// MCPPromptInfo is one prompt.
	MCPPromptInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
)

// MCPTestResult is what a server offers.
type MCPTestResult struct {
	Tools     []MCPToolInfo     `json:"tools"`
	Resources []MCPResourceInfo `json:"resources"`
	Prompts   []MCPPromptInfo   `json:"prompts"`
}

// mcpConfig returns a copy of the configured servers.
func (s *Service) mcpConfig() config.MCPConfig {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return config.MCPConfig{Servers: slices.Clone(s.Config.MCP.Servers)}
}

// MCPServers lists the configured servers.
func (s *Service) MCPServers() []MCPServerInfo {
	cfg := s.mcpConfig()
	out := make([]MCPServerInfo, 0, len(cfg.Servers))
	for _, srv := range cfg.Servers {
		out = append(out, MCPServerInfo{MCPServer: srv})
	}
	return out
}

// AddMCPServer validates and saves a server.
func (s *Service) AddMCPServer(srv config.MCPServer) (MCPServerInfo, error) {
	if err := srv.Check(); err != nil {
		return MCPServerInfo{}, &InvalidError{err.Error()}
	}
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if _, ok := s.Config.MCP.Find(srv.Name); ok {
		return MCPServerInfo{}, fmt.Errorf("%w: mcp server %q already exists", ErrConflict, srv.Name)
	}
	next := s.Config.MCP
	next.Servers = append(slices.Clone(next.Servers), srv)
	if err := next.Check(); err != nil {
		return MCPServerInfo{}, &InvalidError{err.Error()}
	}
	if err := s.saveMCP(next); err != nil {
		return MCPServerInfo{}, err
	}
	return MCPServerInfo{MCPServer: srv}, nil
}

// RemoveMCPServer deletes a server no workspace notebook attaches.
func (s *Service) RemoveMCPServer(name string) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if _, ok := s.Config.MCP.Find(name); !ok {
		return fmt.Errorf("mcp server %q: %w", name, ErrNotFound)
	}
	if users := s.notebooksUsingMCP(name); len(users) > 0 {
		return fmt.Errorf("%w: mcp server %q is attached to %s; detach it first", ErrConflict, name,
			strings.Join(users, ", "))
	}
	next := config.MCPConfig{}
	for _, srv := range s.Config.MCP.Servers {
		if srv.Name != name {
			next.Servers = append(next.Servers, srv)
		}
	}
	return s.saveMCP(next)
}

func (s *Service) notebooksUsingMCP(name string) []string {
	nbs, err := s.ListNotebooks()
	if err != nil {
		return nil
	}
	var users []string
	for _, n := range nbs {
		if slices.Contains(n.nb.Manifest.Agent.MCPServers, name) {
			users = append(users, n.Name)
		}
	}
	return users
}

// saveMCP writes next into the config file (only its mcp block) and memory.
// The caller holds cfgMu.
func (s *Service) saveMCP(next config.MCPConfig) error {
	if s.ConfigPath != "" {
		f, err := config.Load(s.ConfigPath)
		if errors.Is(err, fs.ErrNotExist) {
			f, err = config.Default(), nil
		}
		if err != nil {
			return fmt.Errorf("mcp: read config: %w", err)
		}
		f.MCP = next
		if err := f.Save(s.ConfigPath); err != nil {
			return fmt.Errorf("mcp: save config: %w", err)
		}
	}
	s.Config.MCP = next
	return nil
}

// TestMCPServer connects to a server and lists its tools, resources and
// prompts. A server without resources or prompts lists none.
func (s *Service) TestMCPServer(ctx context.Context, name string) (MCPTestResult, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpTestTimeout)
	defer cancel()
	c, err := s.connectMCP(ctx, name)
	if err != nil {
		return MCPTestResult{}, err
	}
	defer func() { _ = c.Close() }()
	tools, err := c.ListTools(ctx)
	if err != nil {
		return MCPTestResult{}, s.mcpErr(err)
	}
	res := MCPTestResult{Tools: []MCPToolInfo{}, Resources: []MCPResourceInfo{}, Prompts: []MCPPromptInfo{}}
	for _, t := range tools {
		res.Tools = append(res.Tools, MCPToolInfo{Name: t.Name, Description: t.Description})
	}
	if list, err := c.ListResources(ctx); err == nil {
		for _, r := range list {
			res.Resources = append(res.Resources, MCPResourceInfo{URI: r.URI, Name: r.Name, MimeType: r.MimeType})
		}
	}
	if list, err := c.ListPrompts(ctx); err == nil {
		for _, p := range list {
			res.Prompts = append(res.Prompts, MCPPromptInfo{Name: p.Name, Description: p.Description})
		}
	}
	return res, nil
}

// connectMCP resolves a server's placeholders and connects. Errors are
// redacted, including the values the placeholders resolved to.
func (s *Service) connectMCP(ctx context.Context, name string) (*mcp.Client, error) {
	srv, ok := s.mcpConfig().Find(name)
	if !ok {
		return nil, fmt.Errorf("mcp server %q: %w", name, ErrNotFound)
	}
	var used []string
	get := func(ref string) (string, error) {
		if s.Secrets == nil {
			return "", fmt.Errorf("%w: %s", secrets.ErrNotFound, ref)
		}
		v, err := s.Secrets.Get(ref)
		if err == nil {
			used = append(used, v)
		}
		return v, err
	}
	resolved := mcp.Server{
		Name: srv.Name, Command: srv.Command, Args: srv.Args, URL: srv.URL, AllowInsecureHTTP: srv.AllowInsecureHTTP,
	}
	var err error
	if resolved.Env, err = expandAll(srv.Env, get); err != nil {
		return nil, fmt.Errorf("mcp server %q: %w", name, err)
	}
	if resolved.Headers, err = expandAll(srv.Headers, get); err != nil {
		return nil, fmt.Errorf("mcp server %q: %w", name, err)
	}
	c, err := mcp.Connect(ctx, resolved)
	if err != nil {
		return nil, s.mcpErr(err, used...)
	}
	return c, nil
}

func expandAll(in map[string]string, get func(string) (string, error)) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		x, err := config.ExpandSecrets(v, get)
		if err != nil {
			return nil, err
		}
		out[k] = x
	}
	return out, nil
}

// mcpErr redacts an MCP error: provider keys, the given secret values and
// credential-shaped text.
func (s *Service) mcpErr(err error, values ...string) error {
	msg := secrets.Redact(s.redactor()(err.Error()), values...)
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", msg, context.DeadlineExceeded)
	}
	return errors.New(msg)
}
