package extract

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TreeOptions controls folder and code-base walking.
type TreeOptions struct {
	MaxFiles   int      // default 2000
	MaxBytes   int64    // per file; default 256 KiB
	Exclude    []string // extra glob patterns (matched against base name and slash path)
	IncludeAll bool     // ignore .gitignore
}

var alwaysSkip = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".dart_tool": true, ".idea": true, ".vscode": true, "__pycache__": true, ".venv": true,
	"bin": true, "obj": true, ".next": true, "coverage": true,
}

// Tree walks root and extracts every supported text file, respecting
// .gitignore patterns at the root and skipping build output, vendored code
// and binaries. Results are sorted by path.
func Tree(root string, opt TreeOptions) ([]Document, error) {
	if opt.MaxFiles <= 0 {
		opt.MaxFiles = 2000
	}
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = 256 << 10
	}
	patterns := append([]string{}, opt.Exclude...)
	if !opt.IncludeAll {
		patterns = append(patterns, readIgnore(filepath.Join(root, ".gitignore"))...)
	}
	var docs []Document
	errLimit := errors.New("file limit reached")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path != root && (alwaysSkip[d.Name()] || ignored(rel, d.Name(), patterns)) {
				return filepath.SkipDir
			}
			return nil
		}
		doc, ok := treeFile(path, rel, d, patterns, opt.MaxBytes)
		if !ok {
			return nil
		}
		docs = append(docs, doc)
		if len(docs) >= opt.MaxFiles {
			return errLimit
		}
		return nil
	})
	if err != nil && !errors.Is(err, errLimit) {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Source < docs[j].Source })
	return docs, nil
}

func treeFile(path, rel string, d fs.DirEntry, patterns []string, maxBytes int64) (Document, bool) {
	if ignored(rel, d.Name(), patterns) || KindOf(d.Name()) == "" {
		return Document{}, false
	}
	info, err := d.Info()
	if err != nil || info.Size() > maxBytes || !info.Mode().IsRegular() {
		return Document{}, false
	}
	doc, err := File(path)
	if err != nil {
		return Document{}, false
	}
	doc.Source = rel
	return doc, true
}

func readIgnore(path string) []string {
	f, err := os.Open(path) //nolint:gosec // G304: .gitignore inside the user's chosen folder
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		out = append(out, strings.TrimSuffix(strings.TrimPrefix(line, "/"), "/"))
	}
	return out
}

// ignored matches simple gitignore-style patterns against the base name and
// the slash-separated relative path (or any of its leading directories).
func ignored(rel, base string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

func sortStrings(s []string) { sort.Strings(s) }
