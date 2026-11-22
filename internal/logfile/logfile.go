// Package logfile is the core's log: a size-capped file that rotates into a
// few numbered copies, with every write redacted of credential-shaped text.
package logfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// Writer appends to path and rotates it to path.1 … path.keep at max bytes.
type Writer struct {
	mu   sync.Mutex
	path string
	max  int64
	keep int
	f    *os.File
	size int64
}

// Open opens (or creates, 0600, in a 0700 folder) the log at path.
func Open(path string, maxBytes int64, keep int) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("log folder: %w", err)
	}
	w := &Writer{path: path, max: maxBytes, keep: keep}
	return w, w.open()
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("open log: %w", err)
	}
	w.f, w.size = f, info.Size()
	return nil
}

// Write redacts p and appends it, rotating first when it would not fit.
func (w *Writer) Write(p []byte) (int, error) {
	clean := []byte(secrets.RedactPatterns(string(p)))
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(clean)) > w.max {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(clean)
	w.size += int64(n)
	if err != nil {
		return 0, fmt.Errorf("write log: %w", err)
	}
	return len(p), nil
}

func (w *Writer) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("rotate log: %w", err)
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.keep))
	for i := w.keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	if w.keep > 0 {
		_ = os.Rename(w.path, w.path+".1")
	} else {
		_ = os.Remove(w.path)
	}
	return w.open()
}

// Close closes the file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("close log: %w", err)
	}
	return nil
}

// Tail returns at most the last n bytes of the file at path; a missing file is empty.
func Tail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the core's own log path
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	if off := info.Size() - n; off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return nil, fmt.Errorf("read log: %w", err)
		}
	}
	b, err := io.ReadAll(io.LimitReader(f, n))
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	return b, nil
}
