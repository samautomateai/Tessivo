package notebook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Checkpoints let a paused or interrupted agent run continue later. Each is
// runs/<id>.checkpoint.json; runs/<id>.lock marks the process working on it.

const checkpointSuffix = ".checkpoint.json"

// MaxCheckpoint bounds one checkpoint file. A variable so tests can lower it.
var MaxCheckpoint = 64 << 20

// LockRefresh is how often the holder refreshes its lock's mtime.
var LockRefresh = 30 * time.Second

// LockStale is how long an unrefreshed lock lasts before it counts as the
// leftover of a dead process.
var LockStale = 2 * time.Minute

var (
	// ErrCheckpointTooLarge means a checkpoint would exceed MaxCheckpoint.
	ErrCheckpointTooLarge = errors.New("checkpoint is too large")
	// ErrRunLocked means another process is working on the run.
	ErrRunLocked = errors.New("run is in use by another Tessivo process")
)

func (n *Notebook) runFile(runID, suffix string) (string, error) {
	if !safeName.MatchString(runID) {
		return "", fmt.Errorf("invalid run id %q", runID)
	}
	return filepath.Join(n.Dir, RunsDir, runID+suffix), nil
}

// SaveCheckpoint writes the run's checkpoint atomically (0600).
func (n *Notebook) SaveCheckpoint(runID string, v any) error {
	path, err := n.runFile(runID, checkpointSuffix)
	if err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	if len(b) > MaxCheckpoint {
		return fmt.Errorf("%w: %d bytes, at most %d", ErrCheckpointTooLarge, len(b), MaxCheckpoint)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return errors.Join(fmt.Errorf("replace checkpoint: %w", err), os.Remove(tmp))
	}
	return nil
}

// LoadCheckpoint reads the run's checkpoint into v. A missing checkpoint
// wraps os.ErrNotExist.
func (n *Notebook) LoadCheckpoint(runID string, v any) error {
	path, err := n.runFile(runID, checkpointSuffix)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("checkpoint %s: %w", runID, err)
	}
	if info.Size() > int64(MaxCheckpoint) {
		return fmt.Errorf("%w: %d bytes", ErrCheckpointTooLarge, info.Size())
	}
	return readJSON(path, v)
}

// RemoveCheckpoint deletes the run's checkpoint; a missing one is fine.
func (n *Notebook) RemoveCheckpoint(runID string) error {
	path, err := n.runFile(runID, checkpointSuffix)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove checkpoint: %w", err)
	}
	return nil
}

// ListCheckpoints returns the IDs of runs that have a checkpoint.
func (n *Notebook) ListCheckpoints() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(n.Dir, RunsDir))
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), checkpointSuffix); ok && !e.IsDir() {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// LoadRun reads runs/<id>.json.
func (n *Notebook) LoadRun(runID string) (Run, error) {
	path, err := n.runFile(runID, ".json")
	if err != nil {
		return Run{}, err
	}
	var r Run
	return r, readJSON(path, &r)
}

// LockRun takes the run's lock, or returns ErrRunLocked. A stale lock (not
// refreshed for LockStale) is taken over once. unlock releases it.
func (n *Notebook) LockRun(runID string) (unlock func(), err error) {
	path, err := n.runFile(runID, ".lock")
	if err != nil {
		return nil, err
	}
	if err := createLock(path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if info, statErr := os.Stat(path); statErr != nil || time.Since(info.ModTime()) < LockStale {
			return nil, ErrRunLocked
		}
		_ = os.Remove(path)
		if err := createLock(path); err != nil {
			return nil, ErrRunLocked
		}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go refreshLock(path, stop, done)
	return func() {
		close(stop)
		<-done
		_ = os.Remove(path)
	}, nil
}

func createLock(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: inside the notebook
	if err != nil {
		return fmt.Errorf("lock run: %w", err)
	}
	return f.Close() //nolint:wrapcheck // closing an empty file just created
}

func refreshLock(path string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	t := time.NewTicker(LockRefresh)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			_ = os.Chtimes(path, now, now)
		}
	}
}
