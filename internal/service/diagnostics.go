package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/samautomateai/tessivo/internal/logfile"
	"github.com/samautomateai/tessivo/internal/research"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// maxLogInBundle is how much of the end of the core log a bundle carries.
const maxLogInBundle = 2 << 20

// Diagnostics writes a zip to attach to a bug report: the version and OS,
// the config (key references only, redacted), whether each key is set,
// counts and token use, and the end of the core log. It never includes key
// values, notebook names, prompts or content.
func (s *Service) Diagnostics(w io.Writer, version, logPath string) error {
	zw := zip.NewWriter(w)
	cfg, err := json.MarshalIndent(s.cfg(), "", "  ")
	if err != nil {
		return fmt.Errorf("diagnostics: %w", err)
	}
	logTail, err := logfile.Tail(logPath, maxLogInBundle)
	if err != nil {
		logTail = []byte("could not read the log: " + err.Error())
	}
	parts := []struct{ name, data string }{
		{"about.txt", fmt.Sprintf("tessivo %s\nos %s/%s\ngo %s\ntime %s\n", version, runtime.GOOS, runtime.GOARCH,
			runtime.Version(), s.now().UTC().Format("2006-01-02T15:04:05Z"))},
		{"checks.txt", s.setupChecks()},
		{"config.json", string(cfg)},
		{"core.log", string(logTail)},
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return fmt.Errorf("diagnostics: %w", err)
		}
		if _, err := f.Write([]byte(secrets.RedactPatterns(p.data))); err != nil {
			return fmt.Errorf("diagnostics: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("diagnostics: %w", err)
	}
	return nil
}

// setupChecks reports setup and counts without names or content.
func (s *Service) setupChecks() string {
	var sb strings.Builder
	cfg := s.cfg()
	fmt.Fprintf(&sb, "providers: %d\naliases: %d\ndefault model set: %v\n", len(cfg.Providers), len(cfg.Aliases),
		cfg.Defaults.Model != "")
	for _, p := range cfg.Providers {
		if p.APIKeyRef == "" {
			continue
		}
		state := "not set"
		if ok, _ := s.SecretSet(p.APIKeyRef); ok {
			state = "set"
		}
		fmt.Fprintf(&sb, "key %s: %s\n", p.APIKeyRef, state)
	}
	if nbs, err := s.ListNotebooks(); err == nil {
		fmt.Fprintf(&sb, "notebooks: %d\n", len(nbs))
	} else {
		fmt.Fprintf(&sb, "notebooks: error %v\n", err)
	}
	if root, err := s.researchRoot(); err == nil {
		if rnbs, err := research.List(root); err == nil {
			fmt.Fprintf(&sb, "research notebooks: %d\n", len(rnbs))
		}
	}
	fmt.Fprintf(&sb, "mcp servers: %d\n", len(cfg.MCP.Servers))
	if u, err := s.Usage(); err == nil {
		fmt.Fprintf(&sb, "tokens today: %d (day limit %d, run limit %d)\n", u.Tokens, u.DayLimit, u.RunLimit)
	}
	return sb.String()
}
