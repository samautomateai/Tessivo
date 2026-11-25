// Package notebook stores a notebook as a portable folder: a manifest,
// prompts, copied context, run records and validated outputs.
package notebook

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/atomicfile"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/format"
	"github.com/samautomateai/tessivo/internal/gitdiff"
	"github.com/samautomateai/tessivo/internal/webfetch"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// ManifestFile is the notebook's manifest file name.
const ManifestFile = "notebook.json"

// Folder names inside a notebook.
const (
	PromptsDir = "prompts"
	ContextDir = "context"
	RunsDir    = "runs"
	OutputsDir = "outputs"
	// ThreadsDir and SuggestionsDir hold one file per output (0700, created on first use).
	ThreadsDir     = "threads"
	SuggestionsDir = "suggestions"
)

var idPattern = regexp.MustCompile(`^\d{8}T\d{6}-[0-9a-f]{6}$`)

// ValidID reports whether id has the NewID format, so it is safe in a path.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// ContextItem is one input attached to a notebook.
type ContextItem struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Path    string `json:"path"`    // relative to the notebook for copied files; absolute for references
	InPlace bool   `json:"inPlace"` // code bases and folders are referenced, not copied
	Enabled bool   `json:"enabled"`
	AddedAt string `json:"addedAt"`
	// Kind is "" for a file or folder, or KindMCPResource for a resource read from
	// an MCP server on every run (Server and URI; Path is empty).
	Kind   string `json:"kind,omitempty"`
	Server string `json:"server,omitempty"`
	URI    string `json:"uri,omitempty"`
	// NeedsRelink marks a source that pointed at the sender's disk (an imported
	// archive drops the path); it is left out of runs until relinked.
	NeedsRelink bool `json:"needsRelink,omitempty"`
}

// KindMCPResource marks a context item read from an MCP server.
const KindMCPResource = "mcp-resource"

// MaxResourceURI bounds an MCP resource URI.
const MaxResourceURI = 2048

// IsMCP reports whether the item is an MCP resource.
func (c ContextItem) IsMCP() bool { return c.Kind == KindMCPResource }

// Manifest is the notebook.json document.
type Manifest struct {
	FormatVersion int                `json:"formatVersion"` // the folder format; see FormatVersion
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	WorkType      string             `json:"workType"`
	CreatedAt     string             `json:"createdAt"`
	Settings      config.RunSettings `json:"settings"`
	Context       []ContextItem      `json:"context"`
	Memory        []string           `json:"memory,omitempty"`   // pinned facts included in every run
	Template      string             `json:"template,omitempty"` // "id@version" this notebook came from
	Agent         AgentSettings      `json:"agent,omitzero"`
	Archived      bool               `json:"archived,omitempty"` // listed apart; schedules do not run
	Tags          []string           `json:"tags,omitempty"`     // NB-5; see NormalizeTags
	Pinned        bool               `json:"pinned,omitempty"`   // listed first
	// Revision counts saves, so a save of a stale copy is refused (lock.go).
	Revision int `json:"revision,omitempty"`
}

// AgentSettings are the notebook's agent-mode settings.
type AgentSettings struct {
	Tools map[string]string `json:"tools,omitempty"` // tool name or "prefix*" → allow, ask or block
	// MCPServers names the configured MCP servers whose tools agent runs may use.
	MCPServers []string `json:"mcpServers,omitempty"`
	// Commands is the allow-list for run_command: matching calls run without asking.
	Commands []config.CommandRule `json:"commands,omitempty"`
}

// Notebook is an opened notebook folder.
type Notebook struct {
	Dir      string
	Manifest Manifest
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// NewID returns a sortable, unique ID such as 20261120T221500-3f9a1c.
func NewID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}

// Create makes a new notebook folder. dir must not already contain a notebook.
func Create(dir, name, workType string) (*Notebook, error) {
	if _, err := worktype.Get(workType); err != nil {
		return nil, err
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	if _, err := os.Stat(filepath.Join(dir, ManifestFile)); err == nil {
		return nil, fmt.Errorf("%s already contains a notebook", dir)
	}
	for _, sub := range []string{PromptsDir, ContextDir, RunsDir, OutputsDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			return nil, fmt.Errorf("create notebook: %w", err)
		}
	}
	nb := &Notebook{Dir: dir, Manifest: Manifest{
		ID: NewID(), Name: name, WorkType: workType, CreatedAt: now(), Context: []ContextItem{},
	}}
	if err := nb.Save(); err != nil {
		return nil, err
	}
	return nb, nil
}

// Open loads an existing notebook.
func Open(dir string) (*Notebook, error) {
	var m Manifest
	if err := readJSON(filepath.Join(dir, ManifestFile), &m); err != nil {
		return nil, fmt.Errorf("open notebook %s: %w", dir, err)
	}
	if err := format.Upgrade("notebook "+dir, &m.FormatVersion, FormatVersion, upgrades, &m); err != nil {
		return nil, err
	}
	if _, err := worktype.Get(m.WorkType); err != nil {
		return nil, fmt.Errorf("open notebook %s: %w", dir, err)
	}
	return &Notebook{Dir: dir, Manifest: m}, nil
}

// AddContext attaches a file (copied into the notebook) or a folder
// (referenced in place, e.g. a code base) and saves the manifest.
func (n *Notebook) AddContext(src string) (ContextItem, error) {
	abs, err := filepath.Abs(src)
	if err != nil {
		return ContextItem{}, fmt.Errorf("context path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ContextItem{}, fmt.Errorf("context %s: %w", src, err)
	}
	item := ContextItem{ID: NewID(), Label: filepath.Base(abs), Enabled: true, AddedAt: now()}
	if info.IsDir() {
		item.Path, item.InPlace = abs, true
	} else {
		rel := filepath.Join(ContextDir, item.ID+"-"+sanitize(filepath.Base(abs)))
		if err := copyFile(abs, filepath.Join(n.Dir, rel)); err != nil {
			return ContextItem{}, err
		}
		item.Path = rel
	}
	return item, n.addItem(item)
}

// SetEnabled toggles a context item by ID or label.
func (n *Notebook) SetEnabled(idOrLabel string, enabled bool) error {
	return n.Update(func(m *Manifest) error {
		for i, c := range m.Context {
			if c.ID == idOrLabel || c.Label == idOrLabel {
				m.Context[i].Enabled = enabled
				return nil
			}
		}
		return fmt.Errorf("no context item %q", idOrLabel)
	})
}

// ContextPath returns the absolute path of a context item.
func (n *Notebook) ContextPath(c ContextItem) string {
	if c.IsMCP() || c.IsURL() || c.IsGitDiff() {
		return "" // read from its server, the web or git, never as files
	}
	if c.InPlace || filepath.IsAbs(c.Path) {
		return c.Path
	}
	return filepath.Join(n.Dir, c.Path)
}

func sanitize(name string) string {
	if safeName.MatchString(name) {
		return name
	}
	out := []rune{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) > 64 {
		out = out[len(out)-64:]
	}
	return string(out)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src) //nolint:gosec // G304: the user chose this file to attach
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		return fmt.Errorf("copy into notebook: %w", err)
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path) //nolint:gosec // G304: paths are inside the notebook folder
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return atomicfile.Write(path, append(b, '\n'), 0o600)
}

// AddMCPResource attaches a resource of an MCP server; it is read on every run.
// The label defaults to the URI.
func (n *Notebook) AddMCPResource(server, uri, label string) (ContextItem, error) {
	if server == "" || uri == "" || len(uri) > MaxResourceURI || !utf8.ValidString(uri) {
		return ContextItem{}, fmt.Errorf("mcp resource: want a server and a URI of at most %d bytes", MaxResourceURI)
	}
	if label == "" {
		label = uri
	}
	item := ContextItem{
		ID: NewID(), Label: label, Enabled: true, AddedAt: now(), Kind: KindMCPResource, Server: server, URI: uri,
	}
	return item, n.addItem(item)
}

// KindURL marks a web page read on every run (URI; Path is empty).
const KindURL = "url"

// MaxTextSource bounds pasted text added as a source.
const MaxTextSource = 2 << 20

// IsURL reports whether the item is a web page.
func (c ContextItem) IsURL() bool { return c.Kind == KindURL }

// AddURL attaches a web page, fetched on every run. The label defaults to
// the host and path.
func (n *Notebook) AddURL(raw, label string) (ContextItem, error) {
	if err := webfetch.Check(raw); err != nil {
		return ContextItem{}, err
	}
	if label == "" {
		u, _ := url.Parse(raw)
		label = strings.TrimSuffix(u.Host+u.Path, "/")
	}
	item := ContextItem{ID: NewID(), Label: label, Enabled: true, AddedAt: now(), Kind: KindURL, URI: raw}
	return item, n.addItem(item)
}

// AddText saves pasted text as a Markdown file in the notebook and attaches it.
func (n *Notebook) AddText(label, text string) (ContextItem, error) {
	if strings.TrimSpace(text) == "" || len(text) > MaxTextSource {
		return ContextItem{}, fmt.Errorf("text must be 1 to %d bytes", MaxTextSource)
	}
	if strings.TrimSpace(label) == "" {
		label = "Pasted text"
	}
	id := NewID()
	rel := filepath.Join(ContextDir, id+"-"+sanitize(label)+".md")
	if err := os.WriteFile(filepath.Join(n.Dir, rel), []byte(strings.TrimRight(text, "\n")+"\n"), 0o600); err != nil {
		return ContextItem{}, fmt.Errorf("save text: %w", err)
	}
	item := ContextItem{ID: id, Label: label, Path: rel, Enabled: true, AddedAt: now()}
	return item, n.addItem(item)
}

// FormatVersion is the notebook folder format this Tessivo writes. Raise it
// with an upgrade step in upgrades when the format changes incompatibly.
const FormatVersion = 1

// upgrades[i] turns a format i+1 manifest into format i+2.
var upgrades []func(*Manifest) error

// KindGitDiff marks a repository's changes against a base (Path: the
// repository, URI: the base), read with git on every run.
const KindGitDiff = "git-diff"

// IsGitDiff reports whether the item is a repository's changes.
func (c ContextItem) IsGitDiff() bool { return c.Kind == KindGitDiff }

// AddGitDiff attaches a repository's changes since base (read on every run).
func (n *Notebook) AddGitDiff(repo, base, label string) (ContextItem, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return ContextItem{}, fmt.Errorf("repository: %w", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return ContextItem{}, fmt.Errorf("repository %s: not a folder", repo)
	}
	if err := gitdiff.CheckBase(base); err != nil {
		return ContextItem{}, err
	}
	if label == "" {
		label = filepath.Base(abs) + " changes since " + base
	}
	item := ContextItem{
		ID: NewID(), Label: label, Enabled: true, AddedAt: now(), Kind: KindGitDiff, Path: abs,
		InPlace: true, URI: base,
	}
	return item, n.addItem(item)
}

// addItem appends a context item to the manifest as it is on disk now.
func (n *Notebook) addItem(item ContextItem) error {
	return n.Update(func(m *Manifest) error {
		m.Context = append(m.Context, item)
		return nil
	})
}
