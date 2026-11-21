package notebook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/testutil"
)

func newNB(t *testing.T) *Notebook {
	t.Helper()
	nb, err := Create(filepath.Join(t.TempDir(), "nb"), "nb", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	return nb
}

func TestCheckpointRoundTrip(t *testing.T) {
	nb := newNB(t)
	type cp struct{ Step int }
	if err := nb.SaveCheckpoint("r1", cp{Step: 3}); err != nil {
		t.Fatal(err)
	}
	testutil.AssertPerm(t, filepath.Join(nb.Dir, RunsDir, "r1.checkpoint.json"), 0o600)
	var got cp
	if err := nb.LoadCheckpoint("r1", &got); err != nil || got.Step != 3 {
		t.Fatalf("load: %+v %v", got, err)
	}
	ids, err := nb.ListCheckpoints()
	if err != nil || len(ids) != 1 || ids[0] != "r1" {
		t.Fatalf("list: %v %v", ids, err)
	}
	if err := nb.RemoveCheckpoint("r1"); err != nil {
		t.Fatal(err)
	}
	if err := nb.LoadCheckpoint("r1", &got); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("after remove: want not exist, got %v", err)
	}
	if err := nb.RemoveCheckpoint("r1"); err != nil {
		t.Fatalf("removing twice must be fine: %v", err)
	}
}

func TestCheckpointTooLarge(t *testing.T) {
	nb := newNB(t)
	old := MaxCheckpoint
	MaxCheckpoint = 1 << 10
	t.Cleanup(func() { MaxCheckpoint = old })
	err := nb.SaveCheckpoint("big", map[string]string{"x": strings.Repeat("a", 2<<10)})
	if !errors.Is(err, ErrCheckpointTooLarge) {
		t.Fatalf("want ErrCheckpointTooLarge, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(nb.Dir, RunsDir, "big.checkpoint.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("no file may be left behind")
	}
}

func TestCheckpointRejectsBadID(t *testing.T) {
	nb := newNB(t)
	if err := nb.SaveCheckpoint("../x", 1); err == nil {
		t.Fatal("a run id with a path must be refused")
	}
	if _, err := nb.LockRun("a/b"); err == nil {
		t.Fatal("a run id with a path must be refused")
	}
}

func TestLockRunExclusive(t *testing.T) {
	nb := newNB(t)
	unlock, err := nb.LockRun("r1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nb.LockRun("r1"); !errors.Is(err, ErrRunLocked) {
		t.Fatalf("second lock: want ErrRunLocked, got %v", err)
	}
	unlock()
	unlock2, err := nb.LockRun("r1")
	if err != nil {
		t.Fatalf("after unlock: %v", err)
	}
	unlock2()
}

func TestLockRunStale(t *testing.T) {
	nb := newNB(t)
	oldStale, oldRefresh := LockStale, LockRefresh
	LockStale, LockRefresh = 50*time.Millisecond, time.Hour
	t.Cleanup(func() { LockStale, LockRefresh = oldStale, oldRefresh })
	lock := filepath.Join(nb.Dir, RunsDir, "r1.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, past, past); err != nil {
		t.Fatal(err)
	}
	unlock, err := nb.LockRun("r1")
	if err != nil {
		t.Fatalf("a stale lock must be taken over: %v", err)
	}
	unlock()
}

func TestLockRunRefreshes(t *testing.T) {
	nb := newNB(t)
	oldStale, oldRefresh := LockStale, LockRefresh
	LockStale, LockRefresh = 300*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { LockStale, LockRefresh = oldStale, oldRefresh })
	unlock, err := nb.LockRun("r1")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	time.Sleep(450 * time.Millisecond) //nolint:forbidigo // the refresher must outlive LockStale
	if _, err := nb.LockRun("r1"); !errors.Is(err, ErrRunLocked) {
		t.Fatalf("a refreshed lock must not go stale: %v", err)
	}
}

func TestRunRecordCompat(t *testing.T) {
	nb := newNB(t)
	old := `{"id":"r0","startedAt":"2026-11-01T00:00:00Z","durationMs":5,"workType":"summarize","prompt":"p",` +
		`"context":[],"settings":{"sources":{}},"modelUsed":"m","attempts":[],"strategy":"full","totalTokens":1,` +
		`"valid":true}`
	if err := os.WriteFile(filepath.Join(nb.Dir, RunsDir, "r0.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := nb.LoadRun("r0")
	if err != nil || r.Status != "" || r.ID != "r0" || r.Mode != "" {
		t.Fatalf("old record: %+v %v", r, err)
	}
	r.Status, r.Mode, r.Steps = StatusFinished, "agent", 2
	r.ToolCalls = []ToolCallRecord{{ID: "c1", Tool: "grep", Args: TruncateRecord(strings.Repeat("a", 9000))}}
	if err := nb.SaveRun(r); err != nil {
		t.Fatal(err)
	}
	back, err := nb.LoadRun("r0")
	if err != nil || back.Status != StatusFinished || len(back.ToolCalls) != 1 ||
		len(back.ToolCalls[0].Args) > 8<<10+16 {
		t.Fatalf("round trip: %+v %v", back, err)
	}
}
