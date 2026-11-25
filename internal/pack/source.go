package pack

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/gitdiff"
)

// Where a pack comes from.
const (
	KindFolder = "folder" // read in place
	KindZip    = "zip"    // unpacked into the packs folder
	KindGit    = "git"    // cloned into the packs folder and refreshed
)

// Limits on fetching.
const (
	maxZipEntries = 2000
	maxCloneBytes = 64 << 20 // the clone with its .git folder
	GitTimeout    = 60 * time.Second
	gitQuiet      = "--quiet"
)

// allowLocalGit lets tests clone from a folder; real sources must be https or ssh.
var allowLocalGit = false

var gitURL = regexp.MustCompile(`^(https://[A-Za-z0-9.-]+(:[0-9]+)?/[^\s]+|git@[A-Za-z0-9.-]+:[A-Za-z0-9._/-]+)$`)

// KindOf says what source is: a git URL (https:// or git@host:path), a pack
// folder or a .zip file. Other URLs (file://, ext::, plain http) are refused.
func KindOf(source string) (string, error) {
	if gitURL.MatchString(source) {
		return KindGit, nil
	}
	if strings.Contains(source, "://") || strings.Contains(source, "::") {
		return "", fmt.Errorf("pack source %q: use an https:// or git@ URL, a folder or a .zip file", source)
	}
	if !filepath.IsAbs(source) {
		return "", fmt.Errorf("pack source %q: want an absolute path", source)
	}
	info, err := os.Stat(source)
	switch {
	case err != nil:
		return "", fmt.Errorf("pack source: %w", err)
	case info.IsDir() && allowLocalGit && isRepo(source):
		return KindGit, nil
	case info.IsDir():
		return KindFolder, nil
	case info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(source), ".zip"):
		return KindZip, nil
	}
	return "", fmt.Errorf("pack source %q: not a folder or a .zip file", source)
}

func isRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Fetch copies a zip or git pack into dest, which must not exist; on failure
// nothing is left. Folder packs are read in place and need no fetch.
func Fetch(ctx context.Context, kind, source, branch, dest string) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("pack: %s already exists", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	var err error
	switch kind {
	case KindZip:
		err = unzip(source, dest)
	case KindGit:
		err = clone(ctx, source, branch, dest)
	default:
		return fmt.Errorf("pack: a %s pack is read in place", kind)
	}
	if err != nil {
		_ = os.RemoveAll(dest)
	}
	return err
}

func clone(ctx context.Context, url, branch, dest string) error {
	if branch != "" {
		if err := gitdiff.CheckBase(branch); err != nil {
			return fmt.Errorf("pack branch: %w", err)
		}
	}
	if !gitURL.MatchString(url) && !allowLocalGit {
		return fmt.Errorf("pack source %q: use an https:// or git@ URL", url)
	}
	ctx, cancel := context.WithTimeout(ctx, GitTimeout)
	defer cancel()
	args := []string{"-c", "protocol.ext.allow=never", "clone", gitQuiet, "--depth=1", "--single-branch", "--no-tags"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, "--", url, filepath.Base(dest))
	if _, err := gitdiff.Run(ctx, filepath.Dir(dest), args...); err != nil {
		return fmt.Errorf("pack clone: %w", err)
	}
	return checkCloneSize(dest)
}

// Refresh brings a cloned pack up to date with its remote branch (the
// remote's default when branch is empty).
func Refresh(ctx context.Context, dir, branch string) error {
	ref := "HEAD"
	if branch != "" {
		if err := gitdiff.CheckBase(branch); err != nil {
			return fmt.Errorf("pack branch: %w", err)
		}
		ref = branch
	}
	ctx, cancel := context.WithTimeout(ctx, GitTimeout)
	defer cancel()
	steps := [][]string{
		{"-c", "protocol.ext.allow=never", "fetch", gitQuiet, "--depth=1", "--no-tags", "origin", ref},
		{"reset", gitQuiet, "--hard", "FETCH_HEAD"},
	}
	for _, args := range steps {
		if _, err := gitdiff.Run(ctx, dir, args...); err != nil {
			return fmt.Errorf("pack refresh: %w", err)
		}
	}
	return checkCloneSize(dir)
}

func checkCloneSize(dir string) error {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("pack: %w", err)
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	if total > maxCloneBytes {
		return fmt.Errorf("pack repository is larger than %d MB", maxCloneBytes>>20)
	}
	return nil
}

// unzip unpacks a zipped pack into dest. The pack may sit at the zip's root
// or in one top-level folder. Names may not leave dest; links are refused;
// sizes are counted while reading.
func unzip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("pack zip: %w", err)
	}
	defer func() { _ = zr.Close() }()
	if len(zr.File) > maxZipEntries {
		return fmt.Errorf("pack zip: more than %d entries", maxZipEntries)
	}
	prefix := zipPrefix(zr.File)
	var total int64
	for _, f := range zr.File {
		rel, ok := strings.CutPrefix(f.Name, prefix)
		if !ok || rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		if f.Mode()&os.ModeType != 0 || !filepath.IsLocal(rel) || strings.ContainsAny(rel, `\:`) {
			return fmt.Errorf("pack zip: unsafe entry %q", f.Name)
		}
		n, err := unzipFile(f, filepath.Join(dest, filepath.FromSlash(rel)), maxBytes-total)
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

// zipPrefix is "<folder>/" when the manifest sits in one top-level folder.
func zipPrefix(files []*zip.File) string {
	for _, f := range files {
		if f.Name == ManifestFile {
			return ""
		}
	}
	for _, f := range files {
		if dir, name, ok := strings.Cut(f.Name, "/"); ok && name == ManifestFile {
			return dir + "/"
		}
	}
	return ""
}

func unzipFile(f *zip.File, path string, budget int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, fmt.Errorf("pack zip: %w", err)
	}
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("pack zip %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: checked local name
	if err != nil {
		return 0, fmt.Errorf("pack zip: %w", err)
	}
	n, err := io.Copy(out, io.LimitReader(rc, budget+1))
	err = errors.Join(err, out.Close())
	if n > budget {
		return n, fmt.Errorf("pack is larger than %d KB", maxBytes>>10)
	}
	if err != nil {
		return n, fmt.Errorf("pack zip %s: %w", f.Name, err)
	}
	return n, nil
}
