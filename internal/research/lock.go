package research

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// Research manifests are written like job notebook manifests (see
// internal/notebook/lock.go): under the folder's lock, with a revision, so the
// CLI and the desktop app can change the same research notebook at once.

// Update applies fn to the manifest as it is on disk now, under the lock, and
// saves it; on success the notebook's copy is the saved manifest.
func (n *Notebook) Update(fn func(m *Manifest) error) error {
	unlock, err := notebook.LockDir(n.Dir)
	if err != nil {
		return err
	}
	defer unlock()
	fresh, err := Open(n.Dir)
	if err != nil {
		return err
	}
	if err := fn(&fresh.Manifest); err != nil {
		return err
	}
	if err := fresh.write(); err != nil {
		return err
	}
	n.Manifest = fresh.Manifest
	return nil
}

// Save writes this copy, unless the manifest on disk moved on since it was
// read (notebook.ErrChanged).
func (n *Notebook) Save() error {
	unlock, err := notebook.LockDir(n.Dir)
	if err != nil {
		return err
	}
	defer unlock()
	var disk Manifest
	switch err := readJSON(filepath.Join(n.Dir, ManifestFile), 1<<20, &disk); {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("save research notebook: %w", err)
	case disk.Revision != n.Manifest.Revision:
		return notebook.ErrChanged
	}
	return n.write()
}

// write saves the manifest with the next revision; the caller holds the lock.
func (n *Notebook) write() error {
	m := n.Manifest
	m.FormatVersion = FormatVersion
	m.Revision++
	if err := writeJSON(filepath.Join(n.Dir, ManifestFile), m); err != nil {
		return err
	}
	n.Manifest = m
	return nil
}
