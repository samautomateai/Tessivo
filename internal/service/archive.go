package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/archive"
	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

// Audit kinds for archives.
const (
	auditExport = "notebook.export"
	auditImport = "notebook.import"

	researchSourcesDir = "sources" // research.sourcesDir
	auditKeyResearch   = "research"
)

// What an archive carries, by top-level name; everything else (runs, audit
// log, threads, suggestions, schedule state, research chats) stays home.
var (
	notebookParts = map[string]bool{notebook.PromptsDir: true, notebook.ContextDir: true, notebook.SchedulesFile: true}
	researchParts = map[string]bool{researchSourcesDir: true, "summaries": true, "notes": true}
)

// ExportNotebook writes a workspace notebook as an archive (NB-4): manifest,
// prompts, copied sources, schedules and, when asked, outputs. Sources that
// point at this computer (folders, code bases, git repositories) keep their
// label and kind but lose their path, marked to be relinked.
func (s *Service) ExportNotebook(id string, outputs bool, w io.Writer) error {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return err
	}
	m := nb.Manifest
	m.Context = append([]notebook.ContextItem{}, m.Context...)
	for i, c := range m.Context {
		if c.InPlace || c.IsGitDiff() {
			m.Context[i].Path, m.Context[i].NeedsRelink, m.Context[i].Enabled = "", true, false
		}
	}
	m.Revision = 0
	parts := notebookParts
	var includes []string
	if outputs {
		parts = withPart(parts, notebook.OutputsDir)
		includes = []string{notebook.OutputsDir}
	}
	err = s.writeArchive(w, nb.Dir, notebook.ManifestFile, m, parts, archive.Meta{
		Kind:     archive.KindNotebook,
		Includes: includes,
	})
	if err == nil {
		s.auditArchive(auditExport, id, map[string]any{"outputs": outputs})
	}
	return err
}

// ExportResearch writes a research notebook as an archive: manifest, copied
// sources, summaries and notes; never its chats.
func (s *Service) ExportResearch(id string, w io.Writer) error {
	nb, err := s.openResearch(id)
	if err != nil {
		return err
	}
	m := nb.Manifest
	m.Sources = append([]research.Source{}, m.Sources...)
	for i, src := range m.Sources {
		if src.InPlace {
			m.Sources[i].Path, m.Sources[i].NeedsRelink, m.Sources[i].Level = "", true, research.LevelOff
		}
	}
	m.Revision = 0
	err = s.writeArchive(w, nb.Dir, research.ManifestFile, m, researchParts, archive.Meta{Kind: archive.KindResearch})
	if err == nil {
		s.auditArchive(auditExport, id, map[string]any{auditKeyResearch: true})
	}
	return err
}

func withPart(parts map[string]bool, name string) map[string]bool {
	out := map[string]bool{name: true}
	for k := range parts {
		out[k] = true
	}
	return out
}

// writeArchive stages the parts and the edited manifest in a private temporary
// folder, then archives that folder.
func (s *Service) writeArchive(w io.Writer, dir, manifestName string, manifest any, parts map[string]bool,
	meta archive.Meta,
) error {
	stage, err := os.MkdirTemp("", "tessivo-export-*")
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err := copyParts(dir, stage, parts); err != nil {
		return err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stage, manifestName), b, 0o600); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	meta.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := archive.Write(w, stage, meta, func(string) bool { return true }); err != nil {
		return &InvalidError{err.Error()}
	}
	return nil
}

// copyParts copies the regular files under the named top-level parts of src.
func copyParts(src, dst string, parts map[string]bool) error {
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		if rel == "." || !parts[top] {
			if d.IsDir() && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		// Records of what the sender sent (deliveries) stay home.
		if !d.Type().IsRegular() || strings.HasSuffix(rel, deliveriesFile) {
			return nil
		}
		b, err := os.ReadFile(path) //nolint:gosec // G304: a file of the notebook being exported
		if err != nil {
			return fmt.Errorf("export %s: %w", rel, err)
		}
		// G703: target is dst joined with a path WalkDir found under src.
		return os.WriteFile(target, b, 0o600) //nolint:gosec // see above
	})
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}

func (s *Service) auditArchive(kind, subject string, data map[string]any) {
	if s.UserAuditPath == "" {
		return
	}
	if l, err := audit.Open(s.UserAuditPath, s.actor()); err == nil {
		_ = errors.Join(l.Append(kind, subject, data), l.Close())
	}
}

// archiveErr turns a refused archive into a 400.
func archiveErr(err error) error {
	refused := []error{archive.ErrFormat, archive.ErrTooLarge, archive.ErrUnsafePath}
	if slices.ContainsFunc(refused, func(e error) bool { return errors.Is(err, e) }) {
		return &InvalidError{err.Error()}
	}
	return err
}
