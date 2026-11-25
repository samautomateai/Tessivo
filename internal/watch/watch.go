// Package watch fingerprints folders and files so changes can be noticed
// without an OS file watcher (SRC-8): the fingerprint covers paths, sizes and
// modification times, never contents, and reads at most MaxFiles entries.
package watch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MaxFiles bounds the files a fingerprint looks at.
const MaxFiles = 5000

var maxFiles = MaxFiles

// maxEntries bounds the folders and files a walk visits, so a deep tree of
// folders with few files is not walked in full every minute.
var maxEntries = 4 * MaxFiles

// skipDirs are folders whose churn is not a change to the sources.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "__pycache__": true}

var errEnough = errors.New("enough files")

// Fingerprint returns a digest of path: for a file its size and time, for a
// folder those of the first MaxFiles files below it (in walk order).
func Fingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("watch: %w", err)
	}
	h := sha256.New()
	line := func(rel string, fi fs.FileInfo) {
		_, _ = fmt.Fprintf(h, "%s\t%d\t%d\n", rel, fi.Size(), fi.ModTime().UnixNano())
	}
	if !info.IsDir() {
		line(filepath.Base(path), info)
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	n, seen := 0, 0
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable entry is skipped, not a failure of the whole folder
		}
		if seen++; seen > maxEntries {
			return errEnough
		}
		if d.IsDir() && p != path && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if n >= maxFiles {
			return errEnough
		}
		fi, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // vanished while walking
		}
		rel, _ := filepath.Rel(path, p)
		line(filepath.ToSlash(rel), fi)
		n++
		return nil
	})
	if err != nil && !errors.Is(err, errEnough) {
		return "", fmt.Errorf("watch: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
