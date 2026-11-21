package extract

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
)

// Limits for Office (zip-based) documents. Declared sizes inside the archive
// are never trusted: every read is capped as it happens.
const (
	MaxArchiveBytes      = 64 << 20  // compressed file size we accept
	MaxArchiveEntries    = 10_000    // entries in the zip directory
	MaxEntryBytes        = 32 << 20  // decompressed bytes read from one part
	MaxArchiveTotalBytes = 128 << 20 // decompressed bytes read across all parts
	MaxCompressionRatio  = 200       // decompressed / compressed, per part
)

// maxEntries is MaxArchiveEntries; tests lower it.
var maxEntries = MaxArchiveEntries

// ErrArchiveLimit reports a zip bomb or an oversized document.
var ErrArchiveLimit = errors.New("archive exceeds safety limits")

// safeZip reads parts of an Office document with bounded decompression.
type safeZip struct {
	files map[string]*zip.File
	used  int64
}

func openZip(b []byte) (*safeZip, error) {
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("not a valid Office document: %w", err)
	}
	if len(r.File) > maxEntries {
		return nil, fmt.Errorf("%w: %d entries", ErrArchiveLimit, len(r.File))
	}
	z := &safeZip{files: make(map[string]*zip.File, len(r.File))}
	for _, f := range r.File {
		// Normalise names; we only ever look parts up, never write them to disk.
		z.files[path.Clean("/" + f.Name)[1:]] = f
	}
	return z, nil
}

func (z *safeZip) has(name string) bool {
	_, ok := z.files[name]
	return ok
}

// read returns a part's bytes, enforcing per-part, total and ratio limits.
func (z *safeZip) read(name string) ([]byte, error) {
	f, ok := z.files[name]
	if !ok {
		return nil, fmt.Errorf("missing part %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open part %s: %w", name, err)
	}
	defer func() { _ = rc.Close() }()
	budget := min(int64(MaxEntryBytes), MaxArchiveTotalBytes-z.used)
	ratioCap := int64(f.CompressedSize64)*MaxCompressionRatio + 1<<20 //nolint:gosec // G115: sizes are < 64 MiB
	budget = min(budget, ratioCap)
	data, err := io.ReadAll(io.LimitReader(rc, budget+1))
	if err != nil {
		return nil, fmt.Errorf("read part %s: %w", name, err)
	}
	if int64(len(data)) > budget {
		return nil, fmt.Errorf("%w: part %s decompresses past its limit", ErrArchiveLimit, name)
	}
	z.used += int64(len(data))
	return data, nil
}
