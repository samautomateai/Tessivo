package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// Write writes archive.json and then every regular file under dir that
// include accepts (by its slash-separated path relative to dir) under Root.
// Links and other special files are skipped. A text file that looks like it
// holds a secret stops the export with an error naming it.
func Write(w io.Writer, dir string, meta Meta, include func(rel string) bool) error {
	if meta.FormatVersion == 0 {
		meta.FormatVersion = FormatVersion
	}
	zw := zip.NewWriter(w)
	head, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("archive meta: %w", err)
	}
	if err := add(zw, MetaName, head); err != nil {
		return err
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil // folders are implied by their files; links and devices are not exported
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("archive %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if !include(rel) {
			return nil
		}
		body, err := os.ReadFile(path) //nolint:gosec // G304: a file inside the notebook being exported
		if err != nil {
			return fmt.Errorf("archive %s: %w", rel, err)
		}
		if utf8.Valid(body) && secrets.RedactPatterns(string(body)) != string(body) {
			return fmt.Errorf("%s looks like it contains a secret; remove it before exporting", rel)
		}
		return add(zw, Root+rel, body)
	})
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}

func add(zw *zip.Writer, name string, body []byte) error {
	f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	return nil
}
