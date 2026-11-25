package pack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samautomateai/tessivo/internal/atomicfile"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/template"
)

// Init writes an empty pack into dir (created if missing): the manifest, an
// empty server list, the item folders and a README to fill in.
func Init(dir, id, name string) error {
	m := Manifest{FormatVersion: FormatVersion, ID: id, Name: name}
	if err := m.Check(); err != nil {
		return err
	}
	if err := writeAll(dir, m, nil, nil, []config.MCPServer{}); err != nil {
		return err
	}
	readme := "# " + name + "\n\nWhat this pack is for and who keeps it up to date.\n"
	return writeFile(dir, ReadmeFile, []byte(readme))
}

// Export writes a pack with the given items into dir. Every item is checked
// first; MCP servers may hold credentials only as ${secret:NAME} references.
func Export(dir string, m Manifest, ts []template.Template, ps []Prompt, servers []config.MCPServer) error {
	m.FormatVersion = FormatVersion
	errs := []error{m.Check()}
	for _, t := range ts {
		if err := t.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("template %s: %w", t.ID, err))
		}
	}
	for _, p := range ps {
		errs = append(errs, p.Check())
	}
	errs = append(errs, config.MCPConfig{Servers: servers}.Check())
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("pack export: %w", err)
	}
	if servers == nil {
		servers = []config.MCPServer{}
	}
	return writeAll(dir, m, ts, ps, servers)
}

func writeAll(dir string, m Manifest, ts []template.Template, ps []Prompt, servers []config.MCPServer) error {
	for _, sub := range []string{TemplatesDir, PromptsDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return fmt.Errorf("pack: %w", err)
		}
	}
	files := map[string]any{ManifestFile: m, MCPFile: servers}
	for _, t := range ts {
		t.Source, t.UpdatedAt = "", ""
		files[TemplatesDir+"/"+t.ID+".json"] = t
	}
	for _, p := range ps {
		files[PromptsDir+"/"+p.Name+".json"] = p
	}
	for rel, v := range files {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("pack %s: %w", rel, err)
		}
		if err := writeFile(dir, rel, append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(dir, rel string, b []byte) error {
	if err := atomicfile.Write(filepath.Join(dir, filepath.FromSlash(rel)), b, 0o600); err != nil {
		return fmt.Errorf("pack %s: %w", rel, err)
	}
	return nil
}
