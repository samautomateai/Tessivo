package research

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/notebook"
)

var unsafeName = strings.NewReplacer("/", "_", `\`, "_", ":", "_", "*", "_", "?", "_", `"`, "_",
	"<", "_", ">", "_", "|", "_")

func (n *Notebook) checkRoom() error {
	if len(n.Manifest.Sources) >= MaxSources {
		return fmt.Errorf("%w: at most %d sources", ErrLimit, MaxSources)
	}
	return nil
}

func (n *Notebook) addSource(s Source) (Source, error) {
	if err := n.checkRoom(); err != nil {
		return Source{}, err
	}
	if s.ID == "" {
		s.ID = notebook.NewID()
	}
	s.Level, s.AddedAt = LevelFull, now()
	n.Manifest.Sources = append(n.Manifest.Sources, s)
	return s, n.Save()
}

// AddPath adds a file (copied into sources/) or a folder (used in place).
func (n *Notebook) AddPath(path string) (Source, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Source{}, fmt.Errorf("source path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Source{}, fmt.Errorf("source %s: %w", path, err)
	}
	if info.IsDir() {
		return n.addSource(Source{Label: filepath.Base(abs), Kind: KindFolder, Path: abs, InPlace: true})
	}
	if err := n.checkRoom(); err != nil {
		return Source{}, err
	}
	id := notebook.NewID()
	rel := filepath.Join(sourcesDir, id+"-"+unsafeName.Replace(filepath.Base(abs)))
	if err := copyFile(abs, filepath.Join(n.Dir, rel)); err != nil {
		return Source{}, err
	}
	return n.addSource(Source{ID: id, Label: filepath.Base(abs), Kind: KindFile, Path: rel})
}

// AddMCP adds an MCP resource, read when it is summarized or sent.
func (n *Notebook) AddMCP(server, uri string) (Source, error) {
	if server == "" || uri == "" || len(uri) > maxURI || !utf8.ValidString(uri) {
		return Source{}, fmt.Errorf("mcp resource: want a server and a URI of at most %d bytes", maxURI)
	}
	return n.addSource(Source{Label: uri, Kind: KindMCP, Server: server, URI: uri})
}

// Find returns the source with id and its index.
func (n *Notebook) Find(id string) (Source, int, error) {
	for i, s := range n.Manifest.Sources {
		if s.ID == id {
			return s, i, nil
		}
	}
	return Source{}, -1, fmt.Errorf("source %s: %w", id, ErrNotFound)
}

// SetLevel sets a source's level as the user's choice. Summary needs a
// summary (hasSummary).
func (n *Notebook) SetLevel(id, level string, hasSummary bool) error {
	_, i, err := n.Find(id)
	if err != nil {
		return err
	}
	switch level {
	case LevelFull, LevelOff:
	case LevelSummary:
		if !hasSummary {
			return ErrNoSummary
		}
	default:
		return fmt.Errorf("level %q: want full, summary or off", level)
	}
	n.Manifest.Sources[i].Level, n.Manifest.Sources[i].LevelSet = level, true
	return n.Save()
}

// RemoveSource drops a source with its copy and summary.
func (n *Notebook) RemoveSource(id string) error {
	s, i, err := n.Find(id)
	if err != nil {
		return err
	}
	n.Manifest.Sources = append(n.Manifest.Sources[:i], n.Manifest.Sources[i+1:]...)
	if err := n.Save(); err != nil {
		return err
	}
	var errs []error
	if p := n.SourcePath(s); s.Kind == KindFile && p != "" {
		errs = append(errs, removeIfExists(p))
	}
	if itemID.MatchString(s.ID) {
		errs = append(errs, removeIfExists(filepath.Join(n.Dir, summariesDir, s.ID+".json")))
	}
	return errors.Join(errs...)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", filepath.Base(path), err)
	}
	return nil
}

// SourcePath is where a file or folder source is read from; "" for MCP.
func (n *Notebook) SourcePath(s Source) string {
	switch s.Kind {
	case KindFolder:
		return s.Path
	case KindFile:
		if !copiedPath(s) {
			return ""
		}
		return filepath.Join(n.Dir, s.Path)
	default:
		return ""
	}
}

// copiedPath reports whether a file source's path is its own copy,
// sources/<id>-<name>, written with either separator.
func copiedPath(s Source) bool {
	p := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(s.Path, `\`, "/")))
	return s.ID != "" && filepath.IsLocal(p) && filepath.Dir(p) == sourcesDir &&
		strings.HasPrefix(filepath.Base(p), s.ID+"-")
}
