// Package atomicfile writes files so readers see the old or the new content,
// never a mix, even when several processes (tess and the desktop app's core)
// write the same file at once: each writer uses its own temporary file in the
// same folder, syncs it and renames it over the target.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Write replaces path with data, with mode perm.
func Write(path string, data []byte, perm os.FileMode) error {
	name := filepath.Base(path)
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+name+".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr, os.Chmod(tmpName, perm)); err != nil {
		return errors.Join(fmt.Errorf("write %s: %w", name, err), os.Remove(tmpName))
	}
	if err := rename(tmpName, path); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", name, err), os.Remove(tmpName))
	}
	return nil
}

// Rename retries: replacing a file another process is replacing or reading
// at that moment fails for a moment on some systems (Windows says "access is
// denied"); it succeeds once the other side is done.
const (
	renameTries = 50
	renamePause = 10 * time.Millisecond
)

// osRename is os.Rename; a variable so tests can simulate a busy target.
var osRename = os.Rename

func rename(from, to string) error {
	var err error
	for range renameTries {
		if err = osRename(from, to); err == nil {
			return nil
		}
		time.Sleep(renamePause)
	}
	return err
}
