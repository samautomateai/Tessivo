package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/research"
)

// ResearchDirName is the research notebooks folder next to the workspace.
const ResearchDirName = "research"

// Summary states shown for a source.
const (
	SummaryReady    = "ready"
	SummaryFailed   = "failed"
	SummaryPending  = "pending"
	SummaryUpdating = "updating"
)

// ResearchInfo is a research notebook as the API shows it.
type ResearchInfo struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	CreatedAt   string               `json:"createdAt"`
	Settings    config.RunSettings   `json:"settings"`
	Sources     []ResearchSourceInfo `json:"sources"`
	Notes       int                  `json:"notes"`
}

// ResearchSourceInfo is a source with its summary state.
type ResearchSourceInfo struct {
	research.Source
	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`
}

// ContextTotals is what a question would send.
type ContextTotals struct {
	Sources   int      `json:"sources"`
	Summaries int      `json:"summaries"`
	Notes     int      `json:"notes"`
	Tokens    int      `json:"tokens"`
	Cut       []string `json:"cut"`
}

// researchLocks serializes manifest updates per research notebook.
var researchLocks sync.Map // id → *sync.Mutex

func researchLock(id string) *sync.Mutex {
	m, _ := researchLocks.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex) //nolint:forcetypeassert // only *sync.Mutex is stored
}

func (s *Service) researchRoot() (string, error) {
	if s.Workspace == "" {
		return "", errors.New("research: no workspace configured")
	}
	return filepath.Join(filepath.Dir(filepath.Clean(s.Workspace)), ResearchDirName), nil
}

func (s *Service) openResearch(id string) (*research.Notebook, error) {
	if !idPattern.MatchString(id) {
		return nil, invalidf("research notebook id %q is not valid", id)
	}
	root, err := s.researchRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, id)
	if _, err := os.Stat(filepath.Join(dir, research.ManifestFile)); err != nil {
		return nil, fmt.Errorf("research notebook %s: %w", id, ErrNotFound)
	}
	return research.Open(dir)
}

// updateResearch opens a notebook, applies edit and saves, under its lock.
func (s *Service) updateResearch(id string, edit func(*research.Notebook) error) (*research.Notebook, error) {
	m := researchLock(id)
	m.Lock()
	defer m.Unlock()
	nb, err := s.openResearch(id)
	if err != nil {
		return nil, err
	}
	if err := edit(nb); err != nil {
		return nil, researchErr(err)
	}
	return nb, nil
}

func researchErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, research.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, research.ErrNoSummary), errors.Is(err, research.ErrLimit):
		return &InvalidError{err.Error()}
	default:
		return err
	}
}

// ListResearch lists research notebooks, newest first.
func (s *Service) ListResearch() ([]ResearchInfo, error) {
	root, err := s.researchRoot()
	if err != nil {
		return nil, err
	}
	nbs, err := research.List(root)
	if err != nil {
		return nil, err
	}
	out := make([]ResearchInfo, 0, len(nbs))
	for _, nb := range nbs {
		out = append(out, s.researchInfo(nb))
	}
	return out, nil
}

// CreateResearch makes a research notebook with a unique ID from its name.
func (s *Service) CreateResearch(name, description string) (ResearchInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ResearchInfo{}, invalidf("name is required")
	}
	root, err := s.researchRoot()
	if err != nil {
		return ResearchInfo{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return ResearchInfo{}, fmt.Errorf("research folder: %w", err)
	}
	base := Slug(name)
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(root, id)); errors.Is(err, fs.ErrNotExist) {
			break
		}
		if i > 999 {
			return ResearchInfo{}, invalidf("too many research notebooks named %q", name)
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	nb, err := research.Create(filepath.Join(root, id), id, name, strings.TrimSpace(description))
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(nb), nil
}

// GetResearch returns a notebook with each source's summary state; changed
// sources at full or summary are queued for a new summary.
func (s *Service) GetResearch(ctx context.Context, id string) (ResearchInfo, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.checkedInfo(ctx, nb), nil
}

// UpdateResearch changes a notebook's name, description or settings.
func (s *Service) UpdateResearch(id string, name, description *string, settings *config.RunSettings,
) (ResearchInfo, error) {
	if settings != nil {
		if err := settings.Validate(); err != nil {
			return ResearchInfo{}, &InvalidError{err.Error()}
		}
	}
	nb, err := s.updateResearch(id, func(nb *research.Notebook) error {
		if name != nil {
			if strings.TrimSpace(*name) == "" {
				return invalidf("name is required")
			}
			nb.Manifest.Name = strings.TrimSpace(*name)
		}
		if description != nil {
			nb.Manifest.Description = strings.TrimSpace(*description)
		}
		if settings != nil {
			nb.Manifest.Settings = *settings
		}
		return nb.Save()
	})
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(nb), nil
}

// DeleteResearch removes a research notebook and everything in it.
func (s *Service) DeleteResearch(id string) error {
	nb, err := s.openResearch(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(nb.Dir); err != nil {
		return fmt.Errorf("delete research notebook: %w", err)
	}
	return nil
}

// researchInfo is the API view from what is stored, reading no source.
func (s *Service) researchInfo(nb *research.Notebook) ResearchInfo {
	return describeResearch(nb, func(src research.Source) (string, string) {
		state, msg, _ := storedState(nb, src)
		return state, msg
	})
}

// checkedInfo is the API view with each source at full or summary checked
// against its content: pending and changed sources are queued.
func (s *Service) checkedInfo(ctx context.Context, nb *research.Notebook) ResearchInfo {
	return describeResearch(nb, func(src research.Source) (string, string) {
		state, msg, sum := storedState(nb, src)
		switch {
		case state == SummaryPending:
			s.enqueueSummary(nb.Manifest.ID, src.ID)
		case state == SummaryReady && src.Level != research.LevelOff:
			if read, err := s.readResearchSource(ctx, nb, src); err == nil && read.fingerprint != sum.Fingerprint {
				s.enqueueSummary(nb.Manifest.ID, src.ID)
				return SummaryUpdating, ""
			}
		}
		return state, msg
	})
}

func describeResearch(nb *research.Notebook, state func(research.Source) (string, string)) ResearchInfo {
	info := ResearchInfo{
		ID: nb.Manifest.ID, Name: nb.Manifest.Name, Description: nb.Manifest.Description,
		CreatedAt: nb.Manifest.CreatedAt, Settings: nb.Manifest.Settings, Sources: []ResearchSourceInfo{},
	}
	for _, src := range nb.Manifest.Sources {
		st, msg := state(src)
		info.Sources = append(info.Sources, ResearchSourceInfo{Source: src, Summary: st, Error: msg})
	}
	if notes, err := nb.Notes(); err == nil {
		info.Notes = len(notes)
	}
	return info
}

// storedState is a source's summary state from its summary file alone.
func storedState(nb *research.Notebook, src research.Source) (state, msg string, sum research.Summary) {
	sum, ok, err := nb.Summary(src.ID)
	switch {
	case err != nil:
		return SummaryFailed, err.Error(), sum
	case !ok:
		return SummaryPending, "", sum
	case sum.Status == research.StatusFailed:
		return SummaryFailed, sum.Error, sum
	}
	return SummaryReady, "", sum
}

// AddResearchSource adds a file or folder (path) or an MCP resource (server
// and uri), then summarizes it: in the caller with wait (the CLI), else in
// the background.
func (s *Service) AddResearchSource(ctx context.Context, id, path, server, uri string, wait bool,
) (ResearchInfo, error) {
	if (path == "") == (server == "") {
		return ResearchInfo{}, invalidf("give a path, or an MCP server and URI")
	}
	if server != "" {
		if _, ok := s.mcpConfig().Find(server); !ok {
			return ResearchInfo{}, invalidf("mcp server %q is not configured; add it first", server)
		}
	} else if !filepath.IsAbs(path) {
		return ResearchInfo{}, invalidf("source path must be absolute")
	}
	var added research.Source
	nb, err := s.updateResearch(id, func(nb *research.Notebook) error {
		var err error
		if server != "" {
			added, err = nb.AddMCP(server, uri)
		} else {
			added, err = nb.AddPath(path)
		}
		if err != nil && !errors.Is(err, research.ErrLimit) {
			return &InvalidError{err.Error()}
		}
		return err
	})
	if err != nil {
		return ResearchInfo{}, err
	}
	if wait {
		if err := s.summarize(ctx, id, added.ID); err != nil {
			return ResearchInfo{}, err
		}
		if nb, err = s.openResearch(id); err != nil {
			return ResearchInfo{}, err
		}
	} else {
		s.enqueueSummary(id, added.ID)
	}
	return s.researchInfo(nb), nil
}

// SetSourceLevel sets a source's level as the user's choice.
func (s *Service) SetSourceLevel(id, sourceID, level string) (ResearchInfo, error) {
	nb, err := s.updateResearch(id, func(nb *research.Notebook) error {
		sum, ok, err := nb.Summary(sourceID)
		if err != nil {
			return err
		}
		if err := nb.SetLevel(sourceID, level, ok && sum.Status == research.StatusReady); err != nil {
			if errors.Is(err, research.ErrNotFound) {
				return err
			}
			return &InvalidError{err.Error()}
		}
		return nil
	})
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(nb), nil
}

// RemoveResearchSource removes a source, its copy and its summary.
func (s *Service) RemoveResearchSource(id, sourceID string) (ResearchInfo, error) {
	nb, err := s.updateResearch(id, func(nb *research.Notebook) error { return nb.RemoveSource(sourceID) })
	if err != nil {
		return ResearchInfo{}, err
	}
	return s.researchInfo(nb), nil
}

// RefreshSummary summarizes a source again if its content changed.
func (s *Service) RefreshSummary(ctx context.Context, id, sourceID string, wait bool) error {
	nb, err := s.openResearch(id)
	if err != nil {
		return err
	}
	if _, _, err := nb.Find(sourceID); err != nil {
		return researchErr(err)
	}
	if wait {
		return s.summarize(ctx, id, sourceID)
	}
	s.enqueueSummary(id, sourceID)
	return nil
}

// ResearchSource returns a source's full text (files joined with their
// paths) and its summary text ("" when there is none).
func (s *Service) ResearchSource(ctx context.Context, id, sourceID string) (text, summary string, err error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return "", "", err
	}
	src, _, err := nb.Find(sourceID)
	if err != nil {
		return "", "", researchErr(err)
	}
	read, err := s.readResearchSource(ctx, nb, src)
	if err != nil {
		return "", "", err
	}
	if sum, ok, _ := nb.Summary(sourceID); ok {
		summary = sum.Text
	}
	return read.flat(), summary, nil
}

// ResearchNotes lists a notebook's notes.
func (s *Service) ResearchNotes(id string) ([]research.Note, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return nil, err
	}
	notes, err := nb.Notes()
	return notes, researchErr(err)
}

// SaveResearchNote creates (empty ID) or replaces a note.
func (s *Service) SaveResearchNote(id string, n research.Note) (research.Note, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return research.Note{}, err
	}
	saved, err := nb.SaveNote(n)
	if err != nil && !errors.Is(err, research.ErrNotFound) {
		return research.Note{}, &InvalidError{err.Error()}
	}
	return saved, researchErr(err)
}

// DeleteResearchNote deletes a note.
func (s *Service) DeleteResearchNote(id, noteID string) error {
	nb, err := s.openResearch(id)
	if err != nil {
		return err
	}
	return researchErr(nb.DeleteNote(noteID))
}

// sourceRead is a source's content as read now.
type sourceRead struct {
	text        string          // files and MCP resources
	files       []research.File // folders
	fingerprint string
	tokens      int
}

func (r sourceRead) flat() string {
	if len(r.files) == 0 {
		return r.text
	}
	var sb strings.Builder
	for _, f := range r.files {
		sb.WriteString("== " + f.Path + "\n" + f.Text + "\n\n")
	}
	return sb.String()
}

// readResearchSource extracts a source and fingerprints what it read.
func (s *Service) readResearchSource(ctx context.Context, nb *research.Notebook, src research.Source,
) (sourceRead, error) {
	var r sourceRead
	switch src.Kind {
	case research.KindMCP:
		sess, err := s.connectMCP(ctx, src.Server)
		if err != nil {
			return r, fmt.Errorf("mcp resource %s %s: %w", src.Server, src.URI, err)
		}
		defer func() { _ = sess.Close() }()
		text, _, err := sess.ReadResource(ctx, src.URI)
		if err != nil {
			return r, fmt.Errorf("mcp resource %s %s: %w", src.Server, src.URI, sess.err(err))
		}
		r.text, r.fingerprint = sess.redact(text), research.TextFingerprint(text)
	default:
		docs, err := extract.Path(nb.SourcePath(src))
		if err != nil {
			return r, fmt.Errorf("source %s: %w", src.Label, err)
		}
		if src.Kind == research.KindFolder {
			for _, d := range docs {
				r.files = append(r.files, research.File{Path: filepath.ToSlash(d.Source), Text: d.Text})
			}
			r.fingerprint = research.FolderFingerprint(docs)
		} else if len(docs) > 0 {
			r.text = docs[0].Text
			if r.fingerprint, err = research.FileFingerprint(nb.SourcePath(src)); err != nil {
				return r, err
			}
		}
	}
	r.tokens = extract.EstimateTokens(r.flat())
	return r, nil
}
