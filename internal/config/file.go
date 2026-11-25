package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/atomicfile"
)

// Fallback triggers accepted in Alias.FallbackOn.
const (
	TriggerTimeout         = "timeout"
	TriggerRateLimit       = "rate_limit"
	TriggerServerError     = "server_error"
	TriggerContextOverflow = "context_overflow"
	TriggerInvalidOutput   = "invalid_output"
)

var validTriggers = map[string]bool{
	TriggerTimeout: true, TriggerRateLimit: true, TriggerServerError: true,
	TriggerContextOverflow: true, TriggerInvalidOutput: true,
}

// DefaultTriggers are used when an alias does not list any.
var DefaultTriggers = []string{TriggerTimeout, TriggerRateLimit, TriggerServerError, TriggerContextOverflow}

// Provider is an OpenAI-compatible endpoint such as a LiteLLM proxy.
// The API key is never stored here: APIKeyRef names the secret to look up.
type Provider struct {
	Name string `json:"name"`
	// Kind is the API: "openai" (default: any OpenAI-compatible endpoint), or the
	// native "anthropic" and "gemini" APIs, whose base_url may be left empty.
	Kind           string            `json:"kind,omitempty"`
	BaseURL        string            `json:"base_url"`
	APIKeyRef      string            `json:"api_key_ref,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	// AllowInsecureHTTP permits plain http:// to a non-loopback host (e.g. a
	// LiteLLM proxy on a trusted LAN). Off by default: keys travel in headers.
	AllowInsecureHTTP bool `json:"allow_insecure_http,omitempty"`
	// MaxConcurrent caps requests in flight to this provider (0: no cap), e.g.
	// for a local server or a low-tier key; extra requests wait their turn.
	MaxConcurrent int `json:"max_concurrent,omitempty"`
}

// ModelRef points at one model on one provider.
type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Alias is a friendly model name (e.g. "smart") with an ordered fallback chain.
type Alias struct {
	Name       string     `json:"name"`
	Chain      []ModelRef `json:"chain"`
	FallbackOn []string   `json:"fallback_on,omitempty"`
	// MaxRetries is how often a model is retried on a rate limit, server error or
	// timeout before falling back; nil means DefaultRetries, 0 turns retries off.
	MaxRetries *int `json:"max_retries,omitempty"`
}

// Triggers returns the alias's fallback triggers or the defaults.
func (a Alias) Triggers() []string {
	if len(a.FallbackOn) == 0 {
		return DefaultTriggers
	}
	return a.FallbackOn
}

// Limits caps token use.
type Limits struct {
	MaxTokensPerRun int `json:"max_tokens_per_run,omitempty"`
	MaxTokensPerDay int `json:"max_tokens_per_day,omitempty"`
}

// File is the on-disk application config shared by the CLI and desktop app.
type File struct {
	// WorkspaceDir holds notebooks managed by the desktop app (default ~/Tessivo/notebooks).
	WorkspaceDir string        `json:"workspace_dir,omitempty"`
	Providers    []Provider    `json:"providers"`
	Aliases      []Alias       `json:"aliases"`
	Defaults     RunSettings   `json:"defaults"`
	Limits       Limits        `json:"limits"`
	Agent        AgentConfig   `json:"agent,omitzero"`
	MCP          MCPConfig     `json:"mcp,omitzero"`
	Desktop      DesktopConfig `json:"desktop,omitzero"`
	// Packs are the team packs added (internal/pack); their content stays in the packs folder.
	Packs []PackSource `json:"packs,omitempty"`
}

// Default returns a starter config pointing at a local LiteLLM proxy.
func Default() File {
	return File{
		Providers: []Provider{{
			Name: "litellm", BaseURL: "http://localhost:4000/v1", APIKeyRef: "TESSIVO_LITELLM_KEY",
			TimeoutSeconds: 120,
		}},
		Aliases: []Alias{},
		Defaults: RunSettings{
			Temperature: Ptr(0.2), MaxOutputTokens: Ptr(8000), ThinkingEffort: ThinkingMedium, Mode: ModePipeline,
		},
		Agent: AgentConfig{}.WithDefaults(),
	}
}

// Workspace returns WorkspaceDir or the default ~/Tessivo/notebooks.
func (f File) Workspace() (string, error) {
	if f.WorkspaceDir != "" {
		return f.WorkspaceDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, "Tessivo", "notebooks"), nil
}

// DefaultPath is the per-user config location, e.g. ~/.config/tessivo/config.json.
func DefaultPath() (string, error) {
	if p := os.Getenv("TESSIVO_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "tessivo", "config.json"), nil
}

// Load reads and validates a config file.
func Load(path string) (File, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: the config path is chosen by the user
	if err != nil {
		return File{}, fmt.Errorf("read config: %w", err)
	}
	var f File
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := f.Validate(); err != nil {
		return File{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return f, nil
}

// Save validates and writes the config atomically with owner-only permissions.
func (f File) Save(path string) error {
	if err := f.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	// tess and the app's core can save the config at the same time.
	if err := atomicfile.Write(path, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// Validate checks references and values across the whole file.
func (f File) Validate() error {
	var errs []error
	providers := map[string]bool{}
	for i, p := range f.Providers {
		switch {
		case p.Name == "":
			errs = append(errs, fmt.Errorf("providers[%d]: name is required", i))
		case providers[p.Name]:
			errs = append(errs, fmt.Errorf("providers[%d]: duplicate name %q", i, p.Name))
		}
		providers[p.Name] = true
		if err := validateBaseURL(p); err != nil {
			errs = append(errs, err)
		}
		if p.MaxConcurrent < 0 || p.MaxConcurrent > 64 {
			errs = append(errs, fmt.Errorf("provider %q: max_concurrent must be 0 to 64", p.Name))
		}
	}
	aliases := map[string]bool{}
	for _, a := range f.Aliases {
		errs = append(errs, validateAlias(a, providers, aliases)...)
		aliases[a.Name] = true
	}
	if err := CheckPacks(f.Packs); err != nil {
		errs = append(errs, err)
	}
	if err := f.Defaults.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("defaults: %w", err))
	}
	errs = append(errs, f.Agent.Validate(), f.MCP.Check(), f.Desktop.Validate())
	return errors.Join(errs...)
}

func validateBaseURL(p Provider) error {
	switch p.Kind {
	case "", KindOpenAI, KindAnthropic, KindGemini:
	default:
		return fmt.Errorf("provider %q: kind %q: want openai, anthropic or gemini", p.Name, p.Kind)
	}
	u, err := url.Parse(p.URL())
	if err != nil || u.Host == "" || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS) {
		return fmt.Errorf("provider %q: base_url must be an http:// or https:// URL", p.Name)
	}
	if u.User != nil {
		return fmt.Errorf("provider %q: base_url must not embed credentials; use api_key_ref", p.Name)
	}
	if u.Scheme == schemeHTTP && !p.AllowInsecureHTTP && !isLoopback(u.Hostname()) {
		return fmt.Errorf("provider %q: plain http to %s would send the API key unencrypted; "+
			"use https or set allow_insecure_http", p.Name, u.Hostname())
	}
	return nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateAlias(a Alias, providers, seen map[string]bool) []error {
	var errs []error
	if a.Name == "" || strings.Contains(a.Name, "/") {
		errs = append(errs, fmt.Errorf("alias %q: name is required and must not contain '/'", a.Name))
	}
	if seen[a.Name] {
		errs = append(errs, fmt.Errorf("alias %q: duplicate", a.Name))
	}
	if len(a.Chain) == 0 {
		errs = append(errs, fmt.Errorf("alias %q: chain needs at least one model", a.Name))
	}
	for _, m := range a.Chain {
		if !providers[m.Provider] || m.Model == "" {
			errs = append(errs, fmt.Errorf("alias %q: unknown provider %q or empty model", a.Name, m.Provider))
		}
	}
	for _, t := range a.FallbackOn {
		if !validTriggers[t] {
			errs = append(errs, fmt.Errorf("alias %q: unknown fallback trigger %q", a.Name, t))
		}
	}
	if r := a.Retries(); r < 0 || r > 10 {
		errs = append(errs, fmt.Errorf("alias %q: max_retries must be 0 to 10", a.Name))
	}
	return errs
}

// Provider returns the named provider.
func (f File) Provider(name string) (Provider, bool) {
	for _, p := range f.Providers {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}

// ResolveModel turns an alias name or "provider/model" into a fallback chain.
func (f File) ResolveModel(name string) (Alias, error) {
	for _, a := range f.Aliases {
		if a.Name == name {
			return a, nil
		}
	}
	if prov, model, ok := strings.Cut(name, "/"); ok {
		if _, found := f.Provider(prov); found && model != "" {
			return Alias{Name: name, Chain: []ModelRef{{Provider: prov, Model: model}}}, nil
		}
	}
	return Alias{}, fmt.Errorf("unknown model %q: not an alias or provider/model", name)
}

// Provider kinds.
const (
	KindOpenAI    = "openai"
	KindAnthropic = "anthropic"
	KindGemini    = "gemini"
)

// URL is the provider's base URL, or its API's official URL when a native
// provider leaves base_url empty.
func (p Provider) URL() string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	switch p.Kind {
	case KindAnthropic:
		return "https://api.anthropic.com"
	case KindGemini:
		return "https://generativelanguage.googleapis.com"
	}
	return ""
}

// DefaultRetries is how often a model is retried when an alias sets no max_retries.
const DefaultRetries = 2

// Retries is the alias's max_retries, or DefaultRetries when unset.
func (a Alias) Retries() int {
	if a.MaxRetries == nil {
		return DefaultRetries
	}
	return *a.MaxRetries
}
