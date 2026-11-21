package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// archive packs dir into dest under a top folder named after dir.
func archive(dir, dest, goos string) error {
	return archiveAs(dir, filepath.Base(dir), dest, goos)
}

// archiveAs packs src into dest under the top folder top: zip for Windows,
// tar.gz (modes and symlinks kept) otherwise. dest is written atomically.
func archiveAs(src, top, dest, goos string) (err error) {
	tmp := dest + ".tmp"
	f, err := os.Create(tmp) //nolint:gosec // G304: dest is the release output path
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if goos == goosWindows {
		err = writeZip(f, src, top)
	} else {
		err = writeTarGz(f, src, top)
	}
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dest, err)
	}
	if err = os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("replace %s: %w", dest, err)
	}
	return nil
}

// walkFiles calls fn for every entry under src (not src itself) with its
// slash-separated name inside the archive.
func walkFiles(src, top string, fn func(p, name string, d fs.DirEntry) error) error {
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		return fn(p, path.Join(top, filepath.ToSlash(rel)), d)
	})
	if err != nil {
		return fmt.Errorf("pack %s: %w", src, err)
	}
	return nil
}

func writeZip(w io.Writer, src, top string) error {
	zw := zip.NewWriter(w)
	err := walkFiles(src, top, func(p, name string, d fs.DirEntry) error {
		if d.IsDir() {
			_, err := zw.Create(name + "/")
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlinks cannot go into a zip", name)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name, h.Method = name, zip.Deflate
		dst, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		return copyFile(dst, p)
	})
	return errors.Join(err, zw.Close())
}

func writeTarGz(w io.Writer, src, top string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	err := walkFiles(src, top, func(p, name string, d fs.DirEntry) error {
		info, err := d.Info()
		if err != nil {
			return err
		}
		link := ""
		if d.Type()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		h.Name = name
		if d.IsDir() {
			h.Name += "/"
		}
		h.Uname, h.Gname, h.Uid, h.Gid = "", "", 0, 0
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(tw, p)
	})
	return errors.Join(err, tw.Close(), gz.Close())
}

func copyFile(dst io.Writer, p string) error {
	f, err := os.Open(p) //nolint:gosec // G304: files under the bundle being packed
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(dst, f)
	return err
}

// sums writes SHA256SUMS (sha256sum format) for every file in dir.
func sums(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list %s: %w", dir, err)
	}
	var lines []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "SHA256SUMS" {
			continue
		}
		h := sha256.New()
		if err := copyFile(h, filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("hash %s: %w", e.Name(), err)
		}
		lines = append(lines, hex.EncodeToString(h.Sum(nil))+"  "+e.Name())
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i][66:] < lines[j][66:] })
	body := strings.Join(lines, "\n") + "\n"
	if err := writePublic(filepath.Join(dir, "SHA256SUMS"), []byte(body)); err != nil {
		return fmt.Errorf("write SHA256SUMS: %w", err)
	}
	return nil
}

// writePublic writes a file that ships in a public release (world-readable).
func writePublic(p string, data []byte) error {
	return os.WriteFile(p, data, 0o644) //nolint:gosec // G306: release files are meant to be readable by everyone
}
