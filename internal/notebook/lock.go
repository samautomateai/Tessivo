package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// The CLI and the desktop app (its core) can change the same notebook at the
// same time. Manifest writes therefore take a lock file in the notebook
// folder, and every save bumps a revision: Update re-reads the manifest under
// the lock, so a change made meanwhile by the other process is kept, and a
// Save of a stale copy is refused instead of overwriting it.

// ErrChanged means the manifest changed on disk since this copy was read.
var ErrChanged = errors.New("the notebook was changed elsewhere (another tess or the app); reload and try again")

// lockFile is created exclusively by the process changing the manifest.
const lockFile = ".manifest.lock"

// Lock timing: how long to wait for another writer, how often to look, and
// when a lock left by a crashed process is broken. A write holds the lock for
// milliseconds, so a lock this old is stale.
var (
	lockWait  = 10 * time.Second
	lockPoll  = 10 * time.Millisecond
	lockStale = 30 * time.Second
)

// LockDir takes dir's manifest lock; call the returned func to release it.
// Research notebooks use it too.
func LockDir(dir string) (func(), error) {
	path := filepath.Join(dir, lockFile)
	deadline := time.Now().Add(lockWait)
	for {
		// G304: the lock is a fixed name inside the notebook.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: see above
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			_ = f.Close()
			return func() { unlockFile(path) }, nil
		}
		// On Windows a lock being deleted by its holder reports "access is denied", not "exists".
		if !errors.Is(err, os.ErrExist) && !errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("lock notebook: %w", err)
		}
		if fi, serr := os.Stat(path); serr == nil && time.Since(fi.ModTime()) > lockStale {
			_ = os.Remove(path) // left by a process that died while writing
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock notebook: another process has held %s for over %s", lockFile, lockWait)
		}
		time.Sleep(lockPoll)
	}
}

// Update applies fn to the manifest as it is on disk now, under the lock, and
// saves it. On success the notebook's copy is the saved manifest; when fn
// fails nothing is written and the copy is left as it was.
func (n *Notebook) Update(fn func(m *Manifest) error) error {
	unlock, err := LockDir(n.Dir)
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

// Save writes this copy of the manifest, unless the manifest on disk has
// moved on since it was read (ErrChanged); prefer Update, which cannot lose a
// change made elsewhere.
func (n *Notebook) Save() error {
	unlock, err := LockDir(n.Dir)
	if err != nil {
		return err
	}
	defer unlock()
	var disk Manifest
	switch err := readJSON(filepath.Join(n.Dir, ManifestFile), &disk); {
	case errors.Is(err, os.ErrNotExist): // a new notebook
	case err != nil:
		return fmt.Errorf("save notebook: %w", err)
	case disk.Revision != n.Manifest.Revision:
		return ErrChanged
	}
	return n.write()
}

// write saves the manifest with the next revision; the caller holds the lock.
func (n *Notebook) write() error {
	if err := n.Manifest.Settings.Validate(); err != nil {
		return fmt.Errorf("notebook settings: %w", err)
	}
	m := n.Manifest
	m.FormatVersion = FormatVersion
	m.Revision++
	if err := writeJSON(filepath.Join(n.Dir, ManifestFile), m); err != nil {
		return err
	}
	n.Manifest = m
	return nil
}

// unlockFile removes the lock, retrying briefly: on Windows the remove fails
// while a waiting process has the file open to look at it.
func unlockFile(path string) {
	for range 100 {
		if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(lockPoll)
	}
}
