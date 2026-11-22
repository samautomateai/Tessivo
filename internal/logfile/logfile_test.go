package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatesAndKeepsAFewFiles(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(filepath.Join(dir, "core.log"), 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("x", 39) + "\n"
	for range 12 {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"core.log", "core.log.1", "core.log.2"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Size() > 100 {
			t.Fatalf("%s: %v (%d bytes)", name, err, info.Size())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "core.log.3")); !os.IsNotExist(err) {
		t.Fatalf("kept more than 2 old files: %v", err)
	}
}

func TestRedactsSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.log")
	w, err := Open(path, 1<<20, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("request failed: Authorization: Bearer sk-abcdefghijklmnop1234\n"))
	_ = w.Close()
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "sk-abcdefghijklmnop1234") {
		t.Fatalf("secret in log: %s", b)
	}
}

func TestTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.log")
	_ = os.WriteFile(path, []byte(strings.Repeat("a", 50)+"END"), 0o600)
	got, err := Tail(path, 10)
	if err != nil || string(got) != "aaaaaaaEND" {
		t.Fatalf("%v %q", err, got)
	}
	if got, err := Tail(filepath.Join(t.TempDir(), "none"), 10); err != nil || len(got) != 0 {
		t.Fatalf("missing log: %v %q", err, got)
	}
}
