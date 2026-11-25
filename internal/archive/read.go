package archive

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract checks every entry of the archive, then writes the notebook files
// into dest, which must not exist yet (it is created 0700, files 0600). It
// unpacks into a temporary folder beside dest and renames it at the end, so a
// refused or failed archive leaves nothing behind. It returns archive.json.
func Extract(r io.ReaderAt, size int64, dest string) (Meta, error) {
	if _, err := os.Lstat(dest); err == nil {
		return Meta{}, fmt.Errorf("import: %s already exists", dest)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return Meta{}, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	if int64(len(zr.File)) > maxEntries {
		return Meta{}, fmt.Errorf("%w: more than %d entries", ErrTooLarge, maxEntries)
	}
	meta, err := check(zr)
	if err != nil {
		return Meta{}, err
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return Meta{}, fmt.Errorf("import: %w", err)
	}
	tmp, err := os.MkdirTemp(parent, ".import-*")
	if err != nil {
		return Meta{}, fmt.Errorf("import: %w", err)
	}
	if err := unpack(zr, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return Meta{}, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.RemoveAll(tmp)
		return Meta{}, fmt.Errorf("import: %w", err)
	}
	return meta, nil
}

// check validates every name and reads archive.json.
func check(zr *zip.Reader) (Meta, error) {
	var meta *Meta
	for _, f := range zr.File {
		if f.Mode()&(os.ModeSymlink|os.ModeDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeIrregular) != 0 {
			return Meta{}, fmt.Errorf("%w: %q is a link or special file", ErrUnsafePath, f.Name)
		}
		if f.Name == MetaName {
			m, err := readMeta(f)
			if err != nil {
				return Meta{}, err
			}
			meta = &m
			continue
		}
		if _, err := safeRel(f.Name); err != nil {
			return Meta{}, err
		}
	}
	if meta == nil {
		return Meta{}, fmt.Errorf("%w: no %s", ErrFormat, MetaName)
	}
	return *meta, nil
}

func readMeta(f *zip.File) (Meta, error) {
	rc, err := f.Open()
	if err != nil {
		return Meta{}, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(io.LimitReader(rc, maxMeta+1))
	if err != nil || len(b) > maxMeta {
		return Meta{}, fmt.Errorf("%w: unreadable %s", ErrFormat, MetaName)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("%w: %s: %w", ErrFormat, MetaName, err)
	}
	if m.FormatVersion != FormatVersion {
		return Meta{}, fmt.Errorf("%w: format version %d (this Tessivo reads %d)", ErrFormat, m.FormatVersion,
			FormatVersion)
	}
	if m.Kind != KindNotebook && m.Kind != KindResearch {
		return Meta{}, fmt.Errorf("%w: kind %q", ErrFormat, m.Kind)
	}
	return m, nil
}

// safeRel returns the entry's path under Root, or ErrUnsafePath when the name
// is absolute, has a drive, a backslash or "..", or lies outside Root.
func safeRel(name string) (string, error) {
	bad := func(why string) error { return fmt.Errorf("%w: %q %s", ErrUnsafePath, name, why) }
	if strings.ContainsAny(name, `\:`) || strings.HasPrefix(name, "/") {
		return "", bad("is absolute or uses '\\' or ':'")
	}
	rel, ok := strings.CutPrefix(name, Root)
	if !ok {
		return "", bad("is outside " + Root)
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." || part == "." {
			return "", bad("has '.' or '..'")
		}
	}
	return rel, nil
}

// unpack writes each file under dir, counting bytes as they are read.
func unpack(zr *zip.Reader, dir string) error {
	var total int64
	for _, f := range zr.File {
		if f.Name == MetaName || strings.HasSuffix(f.Name, "/") {
			continue
		}
		rel, err := safeRel(f.Name)
		if err != nil || rel == "" {
			continue // checked already; an empty name is the Root folder itself
		}
		n, err := writeEntry(f, filepath.Join(dir, filepath.FromSlash(rel)), maxTotal-total)
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func writeEntry(f *zip.File, path string, budget int64) (int64, error) {
	limit := min(maxEntry, budget)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrFormat, f.Name, err)
	}
	defer func() { _ = rc.Close() }()
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: checked by safeRel
	if err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case n > limit:
		return n, fmt.Errorf("%w: %s is larger than allowed", ErrTooLarge, f.Name)
	case errors.Is(err, zip.ErrChecksum), errors.Is(err, zip.ErrFormat):
		return n, fmt.Errorf("%w: %s: %w", ErrFormat, f.Name, err)
	case err != nil:
		return n, fmt.Errorf("import %s: %w", f.Name, err)
	}
	return n, nil
}
