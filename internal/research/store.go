// Package research stores research notebooks: their sources, stored
// summaries, notes and the prompt and citation logic for conversations
// about them. It makes no model calls; the service layer does.
package research

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/format"
)

// Context levels: what a conversation sends of a source.
const (
	LevelFull    = "full"
	LevelSummary = "summary"
	LevelOff     = "off"
)

// Source kinds.
const (
	KindFile   = "file"
	KindFolder = "folder"
	KindMCP    = "mcp-resource"
)

// Note origins.
const (
	OriginHuman = "human"
	OriginAI    = "ai"
)

// Limits and the manifest file name.
const (
	MaxSources   = 200
	MaxNotes     = 500
	MaxNote      = 64 << 10
	maxURI       = 2048
	maxCopy      = 64 << 20
	ManifestFile = "research.json"
)

// Folders inside a research notebook.
const (
	sourcesDir   = "sources"
	summariesDir = "summaries"
	notesDir     = "notes"
	ChatsDir     = "chats"
)

// Errors callers map to statuses.
var (
	ErrNoSummary = errors.New("this source has no summary yet")
	ErrLimit     = errors.New("research notebook limit reached")
	ErrNotFound  = errors.New("not found")
)

// Source is one input. Path is set for files (relative, copied in) and
// folders (absolute, used in place); Server and URI for MCP resources.
// Tokens caches the source's size for the fingerprint in TokensFor.
type Source struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	InPlace   bool   `json:"inPlace,omitempty"`
	Server    string `json:"server,omitempty"`
	URI       string `json:"uri,omitempty"`
	Level     string `json:"level"`
	LevelSet  bool   `json:"levelSet,omitempty"`
	AddedAt   string `json:"addedAt"`
	Tokens    int    `json:"tokens,omitempty"`
	TokensFor string `json:"tokensFor,omitempty"`
}

// Manifest is research.json.
type Manifest struct {
	FormatVersion int                `json:"formatVersion"` // the folder format; see FormatVersion
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	CreatedAt     string             `json:"createdAt"`
	Settings      config.RunSettings `json:"settings"`
	Sources       []Source           `json:"sources"`
}

// Notebook is an opened research notebook folder.
type Notebook struct {
	Dir      string
	Manifest Manifest
}

var itemID = regexp.MustCompile(`^\d{8}T\d{6}-[0-9a-f]{6}$`)

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Create makes a research notebook in dir (which must not hold one).
func Create(dir, id, name, description string) (*Notebook, error) {
	if _, err := os.Stat(filepath.Join(dir, ManifestFile)); err == nil {
		return nil, fmt.Errorf("%s already holds a research notebook", dir)
	}
	for _, sub := range []string{sourcesDir, summariesDir, notesDir, ChatsDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return nil, fmt.Errorf("create research notebook: %w", err)
		}
	}
	nb := &Notebook{Dir: dir, Manifest: Manifest{
		ID: id, Name: name, Description: description, CreatedAt: now(), Sources: []Source{},
	}}
	return nb, nb.Save()
}

// Open loads a research notebook. Unknown levels read as full.
func Open(dir string) (*Notebook, error) {
	var m Manifest
	if err := readJSON(filepath.Join(dir, ManifestFile), 1<<20, &m); err != nil {
		return nil, fmt.Errorf("open research notebook: %w", err)
	}
	name := "research notebook " + filepath.Base(dir)
	if err := format.Upgrade(name, &m.FormatVersion, FormatVersion, upgrades, &m); err != nil {
		return nil, err
	}
	for i := range m.Sources {
		switch m.Sources[i].Level {
		case LevelFull, LevelSummary, LevelOff:
		default:
			m.Sources[i].Level, m.Sources[i].LevelSet = LevelFull, false
		}
		if m.Sources[i].Kind == KindFile && !copiedPath(m.Sources[i]) {
			m.Sources[i].Path = "" // hand-edited to point elsewhere: unreadable, never read or deleted
		}
	}
	if m.Sources == nil {
		m.Sources = []Source{}
	}
	return &Notebook{Dir: dir, Manifest: m}, nil
}

// List opens every research notebook under root, newest first; folders that
// are not notebooks, or cannot be read, are skipped.
func List(root string) ([]*Notebook, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list research notebooks: %w", err)
	}
	var out []*Notebook
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if nb, err := Open(filepath.Join(root, e.Name())); err == nil {
			out = append(out, nb)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Manifest.CreatedAt > out[j].Manifest.CreatedAt })
	return out, nil
}

// Save writes the manifest.
func (n *Notebook) Save() error {
	n.Manifest.FormatVersion = FormatVersion
	return writeJSON(filepath.Join(n.Dir, ManifestFile), n.Manifest)
}

// FormatVersion is the research folder format this Tessivo writes; raise it
// with an upgrade step in upgrades when the format changes incompatibly.
const FormatVersion = 1

// upgrades[i] turns a format i+1 manifest into format i+2.
var upgrades []func(*Manifest) error
