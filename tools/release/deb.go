package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Names inside the Linux packages.
const (
	appBinary  = "tessivo_desktop" // Flutter's BINARY_NAME
	exeTess    = "tess"
	appID      = "dev.tessivo.tessivo_desktop"
	debPackage = "tessivo"
	optDir     = "opt/tessivo"
	homepage   = "https://github.com/samautomateai/tessivo"
)

// execNames are bundle files that must be executable even when the bundle was
// copied through a file system without Unix modes.
var execNames = map[string]bool{appBinary: true, exeTess: true}

// debArch is Debian's name for a Go architecture.
var debArch = map[string]string{archAMD64: "amd64", archARM64: archARM64} // same names as Go

var debVersionOK = regexp.MustCompile(`^[0-9][A-Za-z0-9.+~-]*$`)

// debVersion turns a release version into a Debian one: v1.2.3 is 1.2.3, a
// development build such as dev-<sha> sorts before every release.
func debVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if debVersionOK.MatchString(v) {
		return v
	}
	clean := regexp.MustCompile(`[^A-Za-z0-9.+~]+`).ReplaceAllString(v, ".")
	return "0.0.0~" + strings.Trim(clean, ".")
}

// desktopEntry is the menu entry for the app (freedesktop.org format).
func desktopEntry(exec string) string {
	return "[Desktop Entry]\nType=Application\nName=Tessivo\n" +
		"Comment=Analyze, summarize, document and report with your own models\n" +
		"Exec=" + exec + "\nIcon=tessivo\nTerminal=false\nCategories=Office;Development;\n" +
		"StartupWMClass=" + appID + "\n"
}

// writeDeb packs the Linux bundle (tess already beside the app) into a .deb:
// the app in /opt/tessivo, tess and tessivo in /usr/bin, a menu entry and an
// icon. A .deb is an ar archive of debian-binary, control.tar.gz and
// data.tar.gz, written here with the standard library.
func writeDeb(dest, bundle, version, arch string, icon []byte) error {
	da, ok := debArch[arch]
	if !ok {
		return fmt.Errorf("deb: unknown arch %q", arch)
	}
	data, size, err := debData(bundle, icon)
	if err != nil {
		return err
	}
	control := fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: %s\n"+
		"Maintainer: Tessivo <%s/issues>\nInstalled-Size: %d\nSection: utils\nPriority: optional\n"+
		"Depends: libgtk-3-0t64 | libgtk-3-0, libayatana-appindicator3-1\nHomepage: %s\n"+
		"Description: Analyze, summarize, document and report with your own models\n"+
		" Tessivo is a desktop app and CLI (tess) that works on files, code and notes\n"+
		" with self-hosted or hosted OpenAI-compatible, Anthropic and Gemini models.\n",
		debPackage, debVersion(version), da, homepage, (size+1023)/1024, homepage)
	ctl, err := tarGz(func(tw *tar.Writer) error {
		return tarFile(tw, "./control", []byte(control), 0o644)
	})
	if err != nil {
		return err
	}
	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	for _, m := range []struct {
		name string
		body []byte
	}{{"debian-binary", []byte("2.0\n")}, {"control.tar.gz", ctl}, {"data.tar.gz", data}} {
		arMember(&out, m.name, m.body)
	}
	return writeAtomic(dest, out.Bytes())
}

// arMember appends one member in the common ar format dpkg reads.
func arMember(w *bytes.Buffer, name string, body []byte) {
	_, _ = fmt.Fprintf(w, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", name+"/", 0, 0, 0, "100644", len(body))
	w.Write(body)
	if len(body)%2 == 1 {
		w.WriteByte('\n')
	}
}

// debData is data.tar.gz and the installed size of its files.
func debData(bundle string, icon []byte) ([]byte, int64, error) {
	var size int64
	b, err := tarGz(func(tw *tar.Writer) error {
		for _, d := range []string{
			"./", "./opt/", "./" + optDir + "/", "./usr/", "./usr/bin/", "./usr/share/",
			"./usr/share/applications/", "./usr/share/icons/", "./usr/share/icons/hicolor/",
			"./usr/share/icons/hicolor/256x256/", "./usr/share/icons/hicolor/256x256/apps/",
		} {
			if err := tarDir(tw, d); err != nil {
				return err
			}
		}
		n, err := tarTree(tw, bundle, optDir)
		if err != nil {
			return err
		}
		size = n + int64(len(icon))
		for _, l := range [][2]string{{exeTess, exeTess}, {"tessivo", appBinary}} {
			h := &tar.Header{
				Typeflag: tar.TypeSymlink, Name: "./usr/bin/" + l[0], Linkname: "/" + optDir + "/" + l[1],
				Mode: 0o777, ModTime: time.Unix(0, 0),
			}
			if err := tw.WriteHeader(h); err != nil {
				return err //nolint:wrapcheck // wrapped by tarGz
			}
		}
		entry := []byte(desktopEntry("/" + optDir + "/" + appBinary))
		if err := tarFile(tw, "./usr/share/applications/"+appID+".desktop", entry, 0o644); err != nil {
			return err
		}
		return tarFile(tw, "./usr/share/icons/hicolor/256x256/apps/tessivo.png", icon, 0o644)
	})
	return b, size, err
}

// tarTree adds every entry under src below top, with root ownership and
// normalized modes, and returns the total size of its files.
func tarTree(tw *tar.Writer, src, top string) (int64, error) {
	var size int64
	err := walkFiles(src, top, func(p, name string, d fs.DirEntry) error {
		name = "./" + name // path.Join drops the "./" every other entry has
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		switch {
		case d.IsDir():
			return tarDir(tw, name+"/")
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			h := &tar.Header{Typeflag: tar.TypeSymlink, Name: name, Linkname: filepath.ToSlash(link), Mode: 0o777}
			return wrapTar(name, tw.WriteHeader(h))
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s: not a regular file", name)
		}
		mode := int64(0o644)
		if info.Mode().Perm()&0o111 != 0 || execNames[path.Base(name)] {
			mode = 0o755
		}
		size += info.Size()
		h := &tar.Header{Typeflag: tar.TypeReg, Name: name, Size: info.Size(), Mode: mode, ModTime: info.ModTime()}
		if err := tw.WriteHeader(h); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return copyFile(tw, p)
	})
	return size, err
}

func tarDir(tw *tar.Writer, name string) error {
	h := &tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755, ModTime: time.Unix(0, 0)}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("tar %s: %w", name, err)
	}
	return nil
}

func tarFile(tw *tar.Writer, name string, body []byte, mode int64) error {
	h := &tar.Header{Typeflag: tar.TypeReg, Name: name, Size: int64(len(body)), Mode: mode, ModTime: time.Unix(0, 0)}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("tar %s: %w", name, err)
	}
	if _, err := tw.Write(body); err != nil {
		return fmt.Errorf("tar %s: %w", name, err)
	}
	return nil
}

// tarGz runs fill on a tar writer and returns the gzipped archive.
func tarGz(fill func(tw *tar.Writer) error) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := errors.Join(fill(tw), tw.Close(), gz.Close()); err != nil {
		return nil, fmt.Errorf("tar.gz: %w", err)
	}
	return buf.Bytes(), nil
}

// writeAtomic writes a public release file through a temporary name.
func writeAtomic(dest string, body []byte) error {
	tmp := dest + ".tmp"
	if err := writePublic(tmp, body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil { //nolint:gosec // G703: dest is the release output path
		_ = os.Remove(tmp) //nolint:gosec // G703: dest is the release output path
		return fmt.Errorf("replace %s: %w", dest, err)
	}
	return nil
}

// readAll reads a small file from inside the repository.
func (a *app) readRepo(name string) ([]byte, error) {
	root, err := os.OpenRoot(a.root)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", a.root, err)
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return b, nil
}

func wrapTar(name string, err error) error {
	if err != nil {
		return fmt.Errorf("tar %s: %w", name, err)
	}
	return nil
}
