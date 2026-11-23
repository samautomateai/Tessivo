package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxName is the longest notebook name, in characters.
const MaxName = 200

// ErrBusy means a run of the notebook is executing, paused for an approval or
// interrupted; it must finish or be cancelled first.
var ErrBusy = errors.New("notebook has a run in progress, waiting for approval or interrupted")

// CheckName trims a notebook name and checks it is 1–MaxName printable characters.
func CheckName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", errors.New("name is required")
	case utf8.RuneCountInString(name) > MaxName:
		return "", fmt.Errorf("name is longer than %d characters", MaxName)
	case strings.ContainsFunc(name, unicode.IsControl):
		return "", errors.New("name contains control characters")
	}
	return name, nil
}

// Rename sets the notebook's display name; its folder keeps its name.
func (n *Notebook) Rename(name string) error {
	clean, err := CheckName(name)
	if err != nil {
		return err
	}
	return n.Update(func(m *Manifest) error {
		m.Name = clean
		return nil
	})
}

// SetArchived archives or restores the notebook. Archived notebooks are
// listed apart and their schedules do not run.
func (n *Notebook) SetArchived(archived bool) error {
	return n.Update(func(m *Manifest) error {
		m.Archived = archived
		return nil
	})
}

// Busy reports whether a run holds a fresh lock or a paused or interrupted
// run still has its checkpoint.
func (n *Notebook) Busy() bool {
	entries, err := os.ReadDir(filepath.Join(n.Dir, RunsDir))
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, checkpointSuffix) {
			return true
		}
		if strings.HasSuffix(name, ".lock") {
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) < LockStale {
				return true
			}
		}
	}
	return false
}

// Remove deletes a notebook folder. It refuses a folder without a readable
// manifest (so a mistyped path never deletes other files) and a busy notebook.
func Remove(dir string) error {
	nb, err := Open(dir)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	if nb.Busy() {
		return fmt.Errorf("remove %s: %w", nb.Manifest.Name, ErrBusy)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", nb.Manifest.Name, err)
	}
	return nil
}
