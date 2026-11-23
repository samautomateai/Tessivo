package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/testutil"
)

func TestConcurrentWritersNeverMix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	contents := [][]byte{bytes.Repeat([]byte("a"), 64<<10), bytes.Repeat([]byte("b"), 64<<10)}
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if err := Write(path, contents[w%2], 0o600); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	got, err := os.ReadFile(path)
	if err != nil || (!bytes.Equal(got, contents[0]) && !bytes.Equal(got, contents[1])) {
		t.Fatalf("the file must hold one writer's whole content: %v, %d bytes", err, len(got))
	}
	testutil.AssertPerm(t, path, 0o600)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %d entries", len(entries))
	}
}

func TestWriteFailsCleanly(t *testing.T) {
	if err := Write(filepath.Join(t.TempDir(), "missing", "f"), []byte("x"), 0o600); err == nil {
		t.Fatal("a missing folder must fail")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "sub")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("x"), 0o600); err == nil {
		t.Fatal("replacing a non-empty folder must fail")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("the temporary file must be removed: %d entries", len(entries))
	}
}

func TestWriteRetriesABusyTarget(t *testing.T) {
	fails := 2
	old := osRename
	osRename = func(from, to string) error {
		if fails > 0 {
			fails--
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: os.ErrPermission}
		}
		return old(from, to)
	}
	t.Cleanup(func() { osRename = old })
	if err := Write(filepath.Join(t.TempDir(), "f"), []byte("x"), 0o600); err != nil || fails != 0 {
		t.Fatalf("a briefly busy target must be retried: %v (fails left %d)", err, fails)
	}
}
