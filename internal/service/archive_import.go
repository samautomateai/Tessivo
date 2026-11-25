package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/archive"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

// ImportNotebook adds a notebook from an archive as a new notebook: a new ID,
// "(imported)" after a name already in use, schedules turned off, and every
// source that could point outside the notebook (in place, absolute, git)
// marked to be relinked, so an archive can never make a run read this
// computer's files.
func (s *Service) ImportNotebook(r io.ReaderAt, size int64) (NotebookInfo, error) {
	parts := withPart(notebookParts, notebook.OutputsDir)
	incoming, err := s.extractTo(r, size, s.Workspace, archive.KindNotebook, parts,
		notebook.ManifestFile)
	if err != nil {
		return NotebookInfo{}, err
	}
	defer func() { _ = os.RemoveAll(incoming) }()
	nb, err := notebook.Open(incoming)
	if err != nil {
		return NotebookInfo{}, invalidf("archive notebook: %v", err)
	}
	existing, _ := s.ListNotebooks()
	name := nb.Manifest.Name
	for _, e := range existing {
		if strings.EqualFold(e.Name, name) {
			name += " (imported)"
			break
		}
	}
	id, err := freeID(s.Workspace, name)
	if err != nil {
		return NotebookInfo{}, err
	}
	err = nb.Update(func(m *notebook.Manifest) error {
		m.ID, m.Name, m.Archived, m.Pinned = id, name, false, false
		for i, c := range m.Context {
			if c.InPlace || c.IsGitDiff() || (c.Path != "" && !filepath.IsLocal(c.Path)) {
				m.Context[i].Path, m.Context[i].InPlace = "", true
				m.Context[i].NeedsRelink, m.Context[i].Enabled = true, false
			}
		}
		return nil
	})
	if err != nil {
		return NotebookInfo{}, invalidf("archive notebook: %v", err)
	}
	if err := disableSchedules(nb); err != nil {
		return NotebookInfo{}, err
	}
	dest := filepath.Join(s.Workspace, id)
	if err := os.Rename(incoming, dest); err != nil {
		return NotebookInfo{}, fmt.Errorf("import: %w", err)
	}
	s.auditArchive(auditImport, id, map[string]any{"name": name})
	moved, err := notebook.Open(dest)
	if err != nil {
		return NotebookInfo{}, err
	}
	return describe(id, moved, false), nil
}

// ImportResearch adds a research notebook from an archive, like ImportNotebook;
// in-place sources come in off and marked to be relinked.
func (s *Service) ImportResearch(r io.ReaderAt, size int64) (ResearchInfo, error) {
	root, err := s.researchRoot()
	if err != nil {
		return ResearchInfo{}, err
	}
	incoming, err := s.extractTo(r, size, root, archive.KindResearch, researchParts, research.ManifestFile)
	if err != nil {
		return ResearchInfo{}, err
	}
	defer func() { _ = os.RemoveAll(incoming) }()
	nb, err := research.Open(incoming)
	if err != nil {
		return ResearchInfo{}, invalidf("archive research notebook: %v", err)
	}
	existing, _ := s.ListResearch()
	name := nb.Manifest.Name
	for _, e := range existing {
		if strings.EqualFold(e.Name, name) {
			name += " (imported)"
			break
		}
	}
	id, err := freeID(root, name)
	if err != nil {
		return ResearchInfo{}, err
	}
	nb.Manifest.ID, nb.Manifest.Name, nb.Manifest.Pinned = id, name, false
	for i, src := range nb.Manifest.Sources {
		if src.InPlace || (src.Path != "" && !filepath.IsLocal(src.Path)) {
			nb.Manifest.Sources[i].Path, nb.Manifest.Sources[i].InPlace = "", true
			nb.Manifest.Sources[i].NeedsRelink, nb.Manifest.Sources[i].Level = true, research.LevelOff
		}
	}
	if err := nb.Save(); err != nil {
		return ResearchInfo{}, invalidf("archive research notebook: %v", err)
	}
	dest := filepath.Join(root, id)
	if err := os.Rename(incoming, dest); err != nil {
		return ResearchInfo{}, fmt.Errorf("import: %w", err)
	}
	s.auditArchive(auditImport, id, map[string]any{"name": name, auditKeyResearch: true})
	moved, err := research.Open(dest)
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(moved), nil
}

// extractTo unpacks an archive of kind into a hidden folder under root and
// removes anything but the manifest and the expected parts.
func (s *Service) extractTo(r io.ReaderAt, size int64, root, kind string, parts map[string]bool, manifest string,
) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("import: %w", err)
	}
	incoming := filepath.Join(root, ".incoming-"+notebook.NewID())
	meta, err := archive.Extract(r, size, incoming)
	if err != nil {
		return "", archiveErr(err)
	}
	if meta.Kind != kind {
		_ = os.RemoveAll(incoming)
		return "", invalidf("this archive holds a %s notebook, not a %s one", meta.Kind, kind)
	}
	entries, err := os.ReadDir(incoming)
	if err != nil {
		_ = os.RemoveAll(incoming)
		return "", fmt.Errorf("import: %w", err)
	}
	for _, e := range entries {
		if e.Name() != manifest && !parts[e.Name()] {
			_ = os.RemoveAll(filepath.Join(incoming, e.Name()))
		}
	}
	return incoming, nil
}

// freeID returns an unused folder name under root made from name.
func freeID(root, name string) (string, error) {
	base := Slug(name)
	id := base
	for i := 2; i <= 999; i++ {
		if _, err := os.Stat(filepath.Join(root, id)); os.IsNotExist(err) {
			return id, nil
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return "", invalidf("too many notebooks named %q", name)
}

func disableSchedules(nb *notebook.Notebook) error {
	list, err := nb.Schedules()
	if err != nil || len(list) == 0 {
		return nil //nolint:nilerr // no or unreadable schedules: nothing to turn off
	}
	for i := range list {
		list[i].Enabled = false
	}
	if err := nb.SaveSchedules(list); err != nil {
		return fmt.Errorf("import schedules: %w", err)
	}
	return nil
}

// RelinkContext points a source that needs relinking at a folder or file on
// this computer and turns it back on.
func (s *Service) RelinkContext(id, contextID, path string) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return NotebookInfo{}, err
	}
	if !filepath.IsAbs(path) {
		return NotebookInfo{}, invalidf("path must be absolute")
	}
	if _, err := os.Stat(path); err != nil {
		return NotebookInfo{}, invalidf("%s: %v", path, err)
	}
	found := false
	err = nb.Update(func(m *notebook.Manifest) error {
		for i, c := range m.Context {
			if c.ID == contextID && c.NeedsRelink {
				m.Context[i].Path, m.Context[i].NeedsRelink, m.Context[i].Enabled = filepath.Clean(path), false, true
				found = true
			}
		}
		return nil
	})
	if err != nil {
		return NotebookInfo{}, err
	}
	if !found {
		return NotebookInfo{}, invalidf("no source %q needs relinking", contextID)
	}
	return describe(id, nb, false), nil
}
