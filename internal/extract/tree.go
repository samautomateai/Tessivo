package extract

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	jobs, err := collect(root, opt, patterns)
	if err != nil {
		return nil, err
	}
	return extractAll(jobs), nil
}

type treeJob struct{ path, rel string }

// collect walks the tree and returns the candidate files, sorted by path.
func collect(root string, opt TreeOptions, patterns []string) ([]treeJob, error) {
	var jobs []treeJob
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
		if !candidate(rel, d, patterns, opt.MaxBytes) {
			return nil
		}
		jobs = append(jobs, treeJob{path, rel})
		if len(jobs) >= opt.MaxFiles {
			return errLimit
		}
		return nil
	})
	if err != nil && !errors.Is(err, errLimit) {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].rel < jobs[j].rel })
	return jobs, nil
}

// extractAll reads files with a bounded worker pool, keeping input order.
// Unreadable or binary files are skipped.
func extractAll(jobs []treeJob) []Document {
	results := make([]*Document, len(jobs))
	workers := min(max(runtime.GOMAXPROCS(0), 2), 16, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				if doc, err := File(jobs[i].path); err == nil {
					doc.Source = jobs[i].rel
					results[i] = &doc
				}
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	docs := make([]Document, 0, len(jobs))
	for _, d := range results {
		if d != nil {
			docs = append(docs, *d)
		}
	}
	return docs
}

func candidate(rel string, d fs.DirEntry, patterns []string, maxBytes int64) bool {
	if ignored(rel, d.Name(), patterns) || KindOf(d.Name()) == "" {
		return false
	}
	info, err := d.Info()
	return err == nil && info.Size() <= maxBytes && info.Mode().IsRegular()
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
