package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTwoWritersKeepEachOthersChanges(t *testing.T) {
	cliCopy := newNB(t)
	appCopy, err := Open(cliCopy.Dir) // the app's core read the notebook too
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cliCopy.AddURL("https://status.example.com/", ""); err != nil {
		t.Fatal(err)
	}
	if err := appCopy.Rename("Ops review"); err != nil { // from a copy that has not seen the URL
		t.Fatal(err)
	}
	got, err := Open(cliCopy.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Manifest.Name != "Ops review" || len(got.Manifest.Context) != 1 {
		t.Fatalf("a change was lost: %+v", got.Manifest)
	}
	if appCopy.Manifest.Revision != got.Manifest.Revision || len(appCopy.Manifest.Context) != 1 {
		t.Fatal("Update must leave the copy matching what it saved")
	}

	cliCopy.Manifest.Memory = []string{"stale"}
	if err := cliCopy.Save(); !errors.Is(err, ErrChanged) {
		t.Fatalf("saving a stale copy must be refused, got %v", err)
	}
	if again, _ := Open(cliCopy.Dir); len(again.Manifest.Memory) != 0 {
		t.Fatal("the refused save must not be written")
	}
}

func TestConcurrentUpdatesAllLand(t *testing.T) {
	nb := newNB(t)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mine, err := Open(nb.Dir) // each its own copy, like separate processes
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := mine.AddText(fmt.Sprintf("note %d", i), "text"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := Open(nb.Dir)
	if err != nil || len(got.Manifest.Context) != 12 {
		t.Fatalf("want 12 items, got %d (%v)", len(got.Manifest.Context), err)
	}
	if _, err := os.Stat(filepath.Join(nb.Dir, lockFile)); !os.IsNotExist(err) {
		t.Fatal("the lock must be released")
	}
}

func TestStaleLockIsBrokenAndBusyLockTimesOut(t *testing.T) {
	nb := newNB(t)
	lock := filepath.Join(nb.Dir, lockFile)
	if err := os.WriteFile(lock, []byte("4242"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStale)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if err := nb.Rename("After a crash"); err != nil {
		t.Fatalf("a lock left by a crashed writer must be broken: %v", err)
	}

	if err := os.WriteFile(lock, []byte("4242"), 0o600); err != nil { // fresh: someone is writing
		t.Fatal(err)
	}
	wait := lockWait
	lockWait = 50 * time.Millisecond
	t.Cleanup(func() { lockWait = wait })
	if err := nb.Rename("Blocked"); err == nil {
		t.Fatal("a held lock must time out with an error")
	}
	if got, _ := Open(nb.Dir); got.Manifest.Name != "After a crash" {
		t.Fatalf("nothing may be written without the lock: %q", got.Manifest.Name)
	}
}

func TestUpdateLeavesTheCopyAloneWhenFnFails(t *testing.T) {
	nb := newNB(t)
	before := nb.Manifest
	if err := nb.SetEnabled("no-such-item", false); err == nil {
		t.Fatal("want an error")
	}
	if nb.Manifest.Revision != before.Revision {
		t.Fatal("a failed update must not change the copy")
	}
}
