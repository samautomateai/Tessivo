package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notebookDir makes a small notebook folder with files to keep and to leave out.
func notebookDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"notebook.json":       `{"formatVersion":1,"id":"ops","name":"Ops"}`,
		"prompts/review.json": `{"name":"review","text":"Assess the risk."}`,
		"context/notes.md":    "# Notes\nship in March",
		"runs/r1.json":        `{"id":"r1"}`,
		"audit.jsonl":         `{"kind":"run"}`,
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func keep(rel string) bool { return !strings.HasPrefix(rel, "runs/") && rel != "audit.jsonl" }

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	meta := Meta{Kind: KindNotebook, TessivoVersion: "dev", Includes: []string{"prompts"}}
	if err := Write(&buf, notebookDir(t), meta, keep); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "imported")
	got, err := Extract(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dest)
	if err != nil || got.Kind != KindNotebook || got.FormatVersion != FormatVersion {
		t.Fatalf("%v %+v", err, got)
	}
	b, err := os.ReadFile(filepath.Join(dest, "context", "notes.md"))
	if err != nil || !strings.Contains(string(b), "March") {
		t.Fatalf("notes: %q %v", b, err)
	}
	for _, gone := range []string{"runs/r1.json", "audit.jsonl", "archive.json"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Errorf("%s should not be extracted: %v", gone, err)
		}
	}
}

func TestWriteRefusesSecrets(t *testing.T) {
	dir := notebookDir(t)
	secret := "token: ghp_" + strings.Repeat("a", 36)
	if err := os.WriteFile(filepath.Join(dir, "context", "env.txt"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Write(&bytes.Buffer{}, dir, Meta{Kind: KindNotebook}, keep)
	if err == nil || !strings.Contains(err.Error(), "context/env.txt") {
		t.Fatalf("want a refusal naming the file, got %v", err)
	}
}

type entry struct {
	name string
	body string
	mode os.FileMode
}

func zipOf(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(e.body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var goodMeta = entry{name: MetaName, body: `{"kind":"notebook","formatVersion":1}`}

func TestExtractRefusesHostileArchives(t *testing.T) {
	big := strings.Repeat("a", 2048)
	cases := map[string]struct {
		zip  []byte
		want error
	}{
		"dot dot":   {zipOf(t, goodMeta, entry{name: "notebook/../../x", body: "x"}), ErrUnsafePath},
		"absolute":  {zipOf(t, goodMeta, entry{name: "/etc/x", body: "x"}), ErrUnsafePath},
		"drive":     {zipOf(t, goodMeta, entry{name: `C:\x`, body: "x"}), ErrUnsafePath},
		"backslash": {zipOf(t, goodMeta, entry{name: `notebook\..\x`, body: "x"}), ErrUnsafePath},
		"symlink": {
			zipOf(t, goodMeta, entry{name: "notebook/link", body: "/etc", mode: os.ModeSymlink}), ErrUnsafePath,
		},
		"top level": {zipOf(t, goodMeta, entry{name: "evil/x", body: "x"}), ErrUnsafePath},
		"no meta":   {zipOf(t, entry{name: "notebook/notebook.json", body: "{}"}), ErrFormat},
		"version 2": {zipOf(t, entry{name: MetaName, body: `{"kind":"notebook","formatVersion":2}`}), ErrFormat},
		"bad kind":  {zipOf(t, entry{name: MetaName, body: `{"kind":"evil","formatVersion":1}`}), ErrFormat},
		"too big":   {zipOf(t, goodMeta, entry{name: "notebook/big.txt", body: big}), ErrTooLarge},
		"not a zip": {[]byte("hello"), ErrFormat},
		"many entries": {
			zipOf(t, goodMeta, entry{name: "notebook/a"}, entry{name: "notebook/b"}, entry{name: "notebook/c"}),
			ErrTooLarge,
		},
	}
	defer func(e, n, total int64) { maxEntry, maxEntries, maxTotal = e, n, total }(maxEntry, maxEntries, maxTotal)
	maxEntry, maxEntries, maxTotal = 1024, 3, 4096
	for name, c := range cases {
		dest := filepath.Join(t.TempDir(), "imported")
		_, err := Extract(bytes.NewReader(c.zip), int64(len(c.zip)), dest)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
		if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
			t.Errorf("%s: dest must not exist after a refusal", name)
		}
		parent, _ := os.ReadDir(filepath.Dir(dest))
		if len(parent) != 0 {
			t.Errorf("%s: left %d temporary entries behind", name, len(parent))
		}
	}
}

func TestExtractRefusesAnExistingDestination(t *testing.T) {
	b := zipOf(t, goodMeta, entry{name: "notebook/notebook.json", body: "{}"})
	dest := t.TempDir()
	if _, err := Extract(bytes.NewReader(b), int64(len(b)), dest); err == nil {
		t.Fatal("extracting over an existing folder must fail")
	}
}
