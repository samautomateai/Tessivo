// Package agenttools holds the agent's built-in tools. Every path a tool
// touches is resolved through Roots, so the agent can read only the folders
// and files the run was given.
package agenttools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrOutside means a path resolves outside the run's roots.
var ErrOutside = errors.New("outside the run's folders")

// Roots are the folders (and single files) a run may read.
type Roots struct {
	dirs  []string // resolved folder roots
	files []string // resolved file roots
	// The same roots as given (absolute, cleaned, not resolved), for the name-only
	// check that runs before anything touches the filesystem.
	givenDirs, givenFiles []string
}

// NewRoots resolves each path (absolute, symlinks evaluated). Folders become
// roots; files are allowed exactly.
func NewRoots(paths ...string) (*Roots, error) {
	if len(paths) == 0 {
		return nil, errors.New("agent tools need at least one folder or file")
	}
	r := &Roots{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("root %s: %w", p, err)
		}
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("root %s: %w", p, err)
		}
		info, err := os.Stat(real)
		if err != nil {
			return nil, fmt.Errorf("root %s: %w", p, err)
		}
		if info.IsDir() {
			r.dirs, r.givenDirs = append(r.dirs, real), append(r.givenDirs, filepath.Clean(abs))
		} else {
			r.files, r.givenFiles = append(r.files, real), append(r.givenFiles, filepath.Clean(abs))
		}
	}
	return r, nil
}

// Paths lists the resolved roots, folders first.
func (r *Roots) Paths() []string {
	return append(append([]string(nil), r.dirs...), r.files...)
}

// foreignAbs matches absolute paths in another OS's form, which must never be
// taken as relative names: drive letters and UNC shares.
var foreignAbs = regexp.MustCompile(`^([A-Za-z]:|\\\\|//)`)

// Resolve returns the real path for p, or ErrOutside. Relative paths are taken
// against the first folder root; "" means that root. The path need not exist.
func (r *Roots) Resolve(p string) (string, error) {
	if p == "" || p == "." {
		if len(r.dirs) == 0 {
			return "", fmt.Errorf("%q: %w", p, ErrOutside)
		}
		return r.dirs[0], nil
	}
	if foreignAbs.MatchString(p) && !filepath.IsAbs(p) {
		return "", fmt.Errorf("%q: %w", p, ErrOutside)
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		if len(r.dirs) == 0 {
			return "", fmt.Errorf("%q: %w", p, ErrOutside)
		}
		p = filepath.Join(r.dirs[0], p)
	}
	p = filepath.Clean(p)
	// Refuse by name first, so a path elsewhere (e.g. a UNC share) is never touched.
	if !within(p, r.dirs, r.files) && !within(p, r.givenDirs, r.givenFiles) {
		return "", fmt.Errorf("%q: %w", p, ErrOutside)
	}
	real, err := evalExisting(p)
	if err != nil {
		return "", fmt.Errorf("%q: %w", p, err)
	}
	if within(real, r.dirs, r.files) {
		return real, nil
	}
	return "", fmt.Errorf("%q: %w", p, ErrOutside)
}

// within reports whether p is one of files or inside one of dirs.
func within(p string, dirs, files []string) bool {
	for _, f := range files {
		if p == f {
			return true
		}
	}
	for _, d := range dirs {
		rel, err := filepath.Rel(d, p)
		up := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		if err == nil && !filepath.IsAbs(rel) && !up {
			return true
		}
	}
	return false
}

// evalSymlinks is filepath.EvalSymlinks; a variable so tests can see what gets touched.
var evalSymlinks = filepath.EvalSymlinks

// evalExisting evaluates symlinks on the longest existing prefix of p and
// appends the rest, so new paths inside a root still resolve.
func evalExisting(p string) (string, error) {
	rest := ""
	cur := p
	for {
		real, err := evalSymlinks(cur)
		if err == nil {
			return filepath.Join(real, rest), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("resolve: %w", err)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}
