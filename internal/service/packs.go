package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/pack"
)

// Audit kinds for packs.
const (
	auditPackAdd     = "pack.add"
	auditPackRefresh = "pack.refresh"
	auditPackRemove  = "pack.remove"
	packsDirName     = "packs"
)

// PackInfo is a pack as the API shows it.
type PackInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Publisher   string   `json:"publisher,omitempty"`
	Version     string   `json:"version,omitempty"`
	Source      string   `json:"source"`
	Kind        string   `json:"kind"`
	Branch      string   `json:"branch,omitempty"`
	FetchedAt   string   `json:"fetchedAt,omitempty"` // when a zip or git pack was last copied or refreshed
	Error       string   `json:"error,omitempty"`     // the last refresh or load error
	Templates   int      `json:"templates"`
	Prompts     int      `json:"prompts"`
	Servers     int      `json:"servers"`
	Problems    []string `json:"problems"`
	README      string   `json:"readme,omitempty"`
}

func (s *Service) packsDir() (string, error) {
	if s.ConfigPath == "" {
		return "", errors.New("packs: no config folder")
	}
	return filepath.Join(filepath.Dir(s.ConfigPath), packsDirName), nil
}

// packDir is where a pack's files are: its own folder, or its copy.
func (s *Service) packDir(src config.PackSource) string {
	if src.Kind == pack.KindFolder {
		return src.Source
	}
	dir, err := s.packsDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, src.ID)
}

// AddPack adds a pack from a folder (read in place), a .zip file or a git URL
// (copied into the packs folder). Its ID comes from its manifest.
func (s *Service) AddPack(ctx context.Context, source, branch string) (PackInfo, error) {
	kind, err := pack.KindOf(source)
	if err != nil {
		return PackInfo{}, &InvalidError{err.Error()}
	}
	if len(s.cfg().Packs) >= config.MaxPacks {
		return PackInfo{}, invalidf("at most %d packs", config.MaxPacks)
	}
	src := config.PackSource{Source: source, Kind: kind, Branch: branch}
	if kind == pack.KindFolder {
		p, err := pack.Load(source)
		if err != nil {
			return PackInfo{}, &InvalidError{err.Error()}
		}
		src.ID = p.Manifest.ID
	} else if src.ID, err = s.fetchPack(ctx, src); err != nil {
		return PackInfo{}, err
	}
	if err := s.savePacks(func(list []config.PackSource) ([]config.PackSource, error) {
		if slices.ContainsFunc(list, func(p config.PackSource) bool { return p.ID == src.ID }) {
			return nil, fmt.Errorf("%w: pack %q is already added", ErrConflict, src.ID)
		}
		return append(list, src), nil
	}); err != nil {
		if kind != pack.KindFolder && !errors.Is(err, ErrConflict) {
			_ = os.RemoveAll(s.packDir(src))
		}
		return PackInfo{}, err
	}
	s.auditArchive(auditPackAdd, src.ID, map[string]any{"kind": kind})
	return s.packInfo(src), nil
}

// fetchPack copies a zip or git pack into the packs folder under its ID.
func (s *Service) fetchPack(ctx context.Context, src config.PackSource) (string, error) {
	root, err := s.packsDir()
	if err != nil {
		return "", err
	}
	tmp := filepath.Join(root, ".incoming-"+notebook.NewID())
	if err := pack.Fetch(ctx, src.Kind, src.Source, src.Branch, tmp); err != nil {
		return "", &InvalidError{err.Error()}
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	p, err := pack.Load(tmp)
	if err != nil {
		return "", &InvalidError{err.Error()}
	}
	id := p.Manifest.ID
	if slices.ContainsFunc(s.cfg().Packs, func(x config.PackSource) bool { return x.ID == id }) {
		return "", fmt.Errorf("%w: pack %q is already added", ErrConflict, id)
	}
	if err := os.RemoveAll(filepath.Join(root, id)); err != nil { // a copy left from a removed pack
		return "", fmt.Errorf("pack: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(root, id)); err != nil {
		return "", fmt.Errorf("pack: %w", err)
	}
	return id, nil
}

// Packs lists the added packs with what each holds.
func (s *Service) Packs() []PackInfo {
	out := []PackInfo{}
	for _, src := range s.cfg().Packs {
		out = append(out, s.packInfo(src))
	}
	return out
}

func (s *Service) packInfo(src config.PackSource) PackInfo {
	info := PackInfo{
		ID: src.ID, Name: src.ID, Source: src.Source, Kind: src.Kind, Branch: src.Branch,
		Problems: []string{},
	}
	dir := s.packDir(src)
	p, err := pack.Load(dir)
	s.packMu.Lock()
	info.Error = s.packErrs[src.ID]
	s.packMu.Unlock()
	if err != nil {
		info.Error = strings.TrimSpace(info.Error + " " + err.Error())
		return info
	}
	m := p.Manifest
	info.Name, info.Description, info.Publisher, info.Version = m.Name, m.Description, m.Publisher, m.Version
	info.Templates, info.Prompts, info.Servers = len(p.Templates), len(p.Prompts), len(p.MCP)
	info.Problems = append(info.Problems, p.Problems...)
	info.README = p.README
	if src.Kind != pack.KindFolder {
		info.FetchedAt = fetchedAt(dir)
	}
	return info
}

// fetchedAt is when a copied pack last changed: the last git fetch, else its manifest.
func fetchedAt(dir string) string {
	for _, name := range []string{
		filepath.Join(".git", "FETCH_HEAD"), filepath.Join(".git", "HEAD"),
		pack.ManifestFile,
	} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return st.ModTime().UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// RefreshPacks brings git packs up to date; a pack that fails keeps its last
// good copy and reports the error. Folder packs are always current; zip packs
// change only when added again.
func (s *Service) RefreshPacks(ctx context.Context) []PackInfo {
	for _, src := range s.cfg().Packs {
		if src.Kind != pack.KindGit {
			continue
		}
		err := pack.Refresh(ctx, s.packDir(src), src.Branch)
		s.packMu.Lock()
		if s.packErrs == nil {
			s.packErrs = map[string]string{}
		}
		if err != nil {
			s.packErrs[src.ID] = "refresh failed: " + err.Error()
		} else {
			delete(s.packErrs, src.ID)
		}
		s.packMu.Unlock()
		s.auditArchive(auditPackRefresh, src.ID, map[string]any{"ok": err == nil})
	}
	return s.Packs()
}

// RemovePack drops a pack, its copy (never a folder pack's own folder) and
// any trust given to its MCP servers.
func (s *Service) RemovePack(id string) error {
	var removed config.PackSource
	err := s.savePacks(func(list []config.PackSource) ([]config.PackSource, error) {
		i := slices.IndexFunc(list, func(p config.PackSource) bool { return p.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("pack %q: %w", id, ErrNotFound)
		}
		removed = list[i]
		return slices.Delete(slices.Clone(list), i, i+1), nil
	})
	if err != nil {
		return err
	}
	if removed.Kind != pack.KindFolder {
		if err := os.RemoveAll(s.packDir(removed)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("pack: %w", err)
		}
	}
	s.auditArchive(auditPackRemove, id, nil)
	return nil
}

// savePacks edits the pack list (and drops trust of removed packs) in the
// config file and in memory, under the config lock.
func (s *Service) savePacks(edit func([]config.PackSource) ([]config.PackSource, error)) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	next, err := edit(s.Config.Packs)
	if err != nil {
		return err
	}
	if err := config.CheckPacks(next); err != nil {
		return &InvalidError{err.Error()}
	}
	trusted := map[string]string{}
	for k, v := range s.Config.MCP.Trusted {
		packID, _, _ := strings.Cut(k, "/")
		if slices.ContainsFunc(next, func(p config.PackSource) bool { return p.ID == packID }) {
			trusted[k] = v
		}
	}
	mcp := s.Config.MCP
	mcp.Trusted = trusted
	if err := s.saveConfigLocked(func(f *config.File) { f.Packs, f.MCP.Trusted = next, trusted }); err != nil {
		return err
	}
	s.Config.Packs, s.Config.MCP = next, mcp
	return nil
}

// saveConfigLocked applies edit to the config file (the defaults when there is
// none yet) and saves it; the caller holds cfgMu.
func (s *Service) saveConfigLocked(edit func(*config.File)) error {
	if s.ConfigPath == "" {
		return nil
	}
	f, err := config.Load(s.ConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		f, err = config.Default(), nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	edit(&f)
	if err := f.Save(s.ConfigPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
