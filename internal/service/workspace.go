package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// idPattern restricts notebook IDs (folder names in the workspace) so API
// callers can never address paths outside it.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// NotebookInfo is a notebook as listed to clients.
type NotebookInfo struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	WorkType   string               `json:"workType"`
	Context    []ContextInfo        `json:"context"`
	Settings   config.RunSettings   `json:"settings"`
	Created    string               `json:"createdAt"`
	Outputs    int                  `json:"outputs"`
	Memory     []string             `json:"memory"`
	Template   string               `json:"template,omitempty"`
	MCPServers []string             `json:"mcpServers"` // servers agent runs may use
	Commands   []config.CommandRule `json:"commands"`   // run_command allow-list
	Archived   bool                 `json:"archived"`
	Tags       []string             `json:"tags"`
	Pinned     bool                 `json:"pinned"`
	nb         *notebook.Notebook
}

// ContextInfo adds extraction stats to a context item.
type ContextInfo struct {
	notebook.ContextItem
	Sources         int `json:"sources"`
	EstimatedTokens int `json:"estimatedTokens"`
}

// Slug turns a display name into a notebook ID.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if b.Len() >= 48 {
			break
		}
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if s == "" {
		s = "notebook"
	}
	return s
}

// OpenNotebook opens a workspace notebook by ID.
func (s *Service) OpenNotebook(id string) (*notebook.Notebook, error) {
	if !idPattern.MatchString(id) {
		return nil, invalidf("notebook id %q is not valid", id)
	}
	dir := filepath.Join(s.Workspace, id)
	if _, err := os.Stat(filepath.Join(dir, notebook.ManifestFile)); err != nil {
		return nil, fmt.Errorf("notebook %s: %w", id, ErrNotFound)
	}
	return notebook.Open(dir)
}

// CreateNotebook creates a notebook in the workspace with a unique ID.
func (s *Service) CreateNotebook(name, workType string) (NotebookInfo, error) {
	if strings.TrimSpace(name) == "" {
		return NotebookInfo{}, invalidf("name is required")
	}
	if err := os.MkdirAll(s.Workspace, 0o700); err != nil {
		return NotebookInfo{}, fmt.Errorf("workspace: %w", err)
	}
	base := Slug(name)
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(s.Workspace, id)); errors.Is(err, os.ErrNotExist) {
			break
		}
		if i > 999 {
			return NotebookInfo{}, invalidf("too many notebooks named %q", name)
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	nb, err := notebook.Create(filepath.Join(s.Workspace, id), name, workType)
	if err != nil {
		if strings.Contains(err.Error(), "unknown work type") {
			return NotebookInfo{}, &InvalidError{err.Error()}
		}
		return NotebookInfo{}, err
	}
	return describe(id, nb, true), nil
}

// ListNotebooks returns workspace notebooks sorted by name.
func (s *Service) ListNotebooks() ([]NotebookInfo, error) {
	entries, err := os.ReadDir(s.Workspace)
	if errors.Is(err, os.ErrNotExist) {
		return []NotebookInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list workspace: %w", err)
	}
	out := []NotebookInfo{}
	for _, e := range entries {
		if !e.IsDir() || !idPattern.MatchString(e.Name()) {
			continue
		}
		nb, err := notebook.Open(filepath.Join(s.Workspace, e.Name()))
		if err != nil {
			continue // not a notebook, or unreadable: skip rather than fail the list
		}
		out = append(out, describe(e.Name(), nb, false))
	}
	sortPinnedFirst(out, func(n NotebookInfo) (bool, string) { return n.Pinned, n.Name })
	return out, nil
}

// Describe returns a notebook's details including context stats (which read
// every context source, so lists skip them).
func (s *Service) Describe(id string) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return NotebookInfo{}, err
	}
	return describe(id, nb, true), nil
}

func describe(id string, nb *notebook.Notebook, stats bool) NotebookInfo {
	info := NotebookInfo{
		ID: id, Name: nb.Manifest.Name, WorkType: nb.Manifest.WorkType, Settings: nb.Manifest.Settings,
		Created: nb.Manifest.CreatedAt, Context: []ContextInfo{}, nb: nb,
		Memory: append([]string{}, nb.Manifest.Memory...), Template: nb.Manifest.Template,
		MCPServers: append([]string{}, nb.Manifest.Agent.MCPServers...),
		Commands:   append([]config.CommandRule{}, nb.Manifest.Agent.Commands...),
		Archived:   nb.Manifest.Archived,
		Tags:       append([]string{}, nb.Manifest.Tags...),
		Pinned:     nb.Manifest.Pinned,
	}
	for _, c := range nb.Manifest.Context {
		ci := ContextInfo{ContextItem: c}
		if !stats || c.IsMCP() || c.IsURL() || c.IsGitDiff() { // read at run time
			info.Context = append(info.Context, ci)
			continue
		}
		if docs, err := extract.Path(nb.ContextPath(c)); err == nil {
			ci.Sources = len(docs)
			for _, d := range docs {
				ci.EstimatedTokens += d.Tokens()
			}
		}
		info.Context = append(info.Context, ci)
	}
	if outs, err := nb.ListOutputs(); err == nil {
		info.Outputs = len(outs)
	}
	return info
}

// AddContext attaches a local path to a notebook.
func (s *Service) AddContext(id, path string) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return NotebookInfo{}, err
	}
	if !filepath.IsAbs(path) {
		return NotebookInfo{}, invalidf("context path must be absolute")
	}
	if _, err := nb.AddContext(path); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	return describe(id, nb, true), nil
}

// SetContextEnabled toggles a context item.
func (s *Service) SetContextEnabled(id, contextID string, enabled bool) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return NotebookInfo{}, err
	}
	if err := nb.SetEnabled(contextID, enabled); err != nil {
		return NotebookInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return describe(id, nb, false), nil
}

// OutputInfo is an output with its notebook and rendered Markdown.
type OutputInfo struct {
	notebook.Output
	NotebookID   string `json:"notebookId"`
	NotebookName string `json:"notebookName"`
	Markdown     string `json:"markdown,omitempty"`
	Version      int    `json:"version"` // 1 for an original output, +1 for each revision
}

// ListOutputs lists outputs across notebooks (or one), newest first.
func (s *Service) ListOutputs(notebookID, workType string) ([]OutputInfo, error) {
	nbs, err := s.ListNotebooks()
	if err != nil {
		return nil, err
	}
	out := []OutputInfo{}
	for _, n := range nbs {
		if notebookID != "" && n.ID != notebookID {
			continue
		}
		outs, err := n.nb.ListOutputs()
		if err != nil {
			continue
		}
		vs := Versions(outs)
		for _, o := range outs {
			if workType == "" || o.WorkType == workType {
				o.Data = nil // list view: keep payloads small
				out = append(out, OutputInfo{Output: o, NotebookID: n.ID, NotebookName: n.Name, Version: vs[o.ID]})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// GetOutput returns one output with data and Markdown.
func (s *Service) GetOutput(notebookID, outputID string) (OutputInfo, error) {
	nb, err := s.openNotebookRef(notebookID) // an ID, or a folder the CLI opened
	if err != nil {
		return OutputInfo{}, err
	}
	outs, err := nb.ListOutputs()
	if err != nil {
		return OutputInfo{}, err
	}
	for _, o := range outs {
		if o.ID == outputID {
			mdPath := filepath.Join(nb.Dir, notebook.OutputsDir, o.ID+".md")
			md, _ := os.ReadFile(mdPath) //nolint:gosec // G304: ID comes from the notebook's own listing
			return OutputInfo{
				Output: o, NotebookID: notebookID, NotebookName: nb.Manifest.Name, Markdown: string(md),
				Version: Versions(outs)[o.ID],
			}, nil
		}
	}
	return OutputInfo{}, fmt.Errorf("output %s: %w", outputID, ErrNotFound)
}

// Settings returns the config. It holds only key references, never values.
func (s *Service) Settings() config.File { return s.cfg() }

// Models lists a provider's models (or all providers').
func (s *Service) Models(ctx context.Context, provider string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, p := range s.cfg().Providers {
		if provider != "" && p.Name != provider {
			continue
		}
		key := ""
		if p.APIKeyRef != "" {
			v, err := s.Secrets.Get(p.APIKeyRef)
			if err != nil {
				return nil, fmt.Errorf("provider %q api key: %w", p.Name, err)
			}
			key = v
		}
		ms, err := llm.NewProviderClient(p, key).Models(ctx)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", p.Name, err)
		}
		ids := make([]string, 0, len(ms))
		for _, m := range ms {
			ids = append(ids, m.ID)
		}
		out[p.Name] = ids
	}
	return out, nil
}

// NotebookUpdate changes a notebook's name, archived state, tags or pin; nil fields stay.
type NotebookUpdate struct {
	Name     *string   `json:"name,omitempty"`
	Archived *bool     `json:"archived,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
	Pinned   *bool     `json:"pinned,omitempty"`
}

// UpdateNotebook renames, archives or restores a workspace notebook.
func (s *Service) UpdateNotebook(id string, in NotebookUpdate) (NotebookInfo, error) {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return NotebookInfo{}, err
	}
	clean := ""
	if in.Name != nil {
		if clean, err = notebook.CheckName(*in.Name); err != nil {
			return NotebookInfo{}, &InvalidError{err.Error()}
		}
	}
	var tags []string
	if in.Tags != nil {
		if tags, err = notebook.NormalizeTags(*in.Tags); err != nil {
			return NotebookInfo{}, &InvalidError{err.Error()}
		}
	}
	err = nb.Update(func(m *notebook.Manifest) error {
		if in.Name != nil {
			m.Name = clean
		}
		if in.Tags != nil {
			m.Tags = tags
		}
		if in.Pinned != nil {
			m.Pinned = *in.Pinned
		}
		if in.Archived != nil {
			m.Archived = *in.Archived
		}
		return nil
	})
	if err != nil {
		return NotebookInfo{}, err
	}
	return describe(id, nb, false), nil
}

// DeleteNotebook removes a workspace notebook and everything in it. A
// notebook with a run in progress, paused or interrupted is a conflict.
func (s *Service) DeleteNotebook(id string) error {
	nb, err := s.OpenNotebook(id)
	if err != nil {
		return err
	}
	if err := notebook.Remove(nb.Dir); err != nil {
		if errors.Is(err, notebook.ErrBusy) {
			return fmt.Errorf("%w: %w", ErrConflict, err)
		}
		return err
	}
	return nil
}
