package research

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// readJSON decodes a file of at most limit bytes.
func readJSON(path string, limit int64, v any) error {
	f, err := os.Open(path) //nolint:gosec // G304: paths are built from validated IDs inside the notebook
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if int64(len(b)) > limit {
		return fmt.Errorf("%s is larger than %d bytes", filepath.Base(path), limit)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

// writeJSON writes v atomically with mode 0600.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return writeFile(path, append(b, '\n'))
}

// writeFile writes b to path atomically with mode 0600.
func writeFile(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp.Name(), 0o600)); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	return nil
}

// copyFile copies src to dst (at most maxCopy bytes) with mode 0600.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // G304: the user chose this file as a source
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(src), err)
	}
	defer func() { _ = in.Close() }()
	b, err := io.ReadAll(io.LimitReader(in, maxCopy+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(src), err)
	}
	if len(b) > maxCopy {
		return fmt.Errorf("%s is larger than %d MiB", filepath.Base(src), maxCopy>>20)
	}
	return writeFile(dst, b)
}
