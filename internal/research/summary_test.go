package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestSummaryRoundTripAndCut(t *testing.T) {
	nb := newNB(t)
	id := notebook.NewID()
	long := strings.Repeat("é", 6<<10) // 12 KiB of two-byte runes
	if err := nb.SaveSummary(Summary{SourceID: id, Fingerprint: "f", Text: long, Status: StatusReady}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := nb.Summary(id)
	if err != nil || !ok || len(got.Text) > MaxSummary || !utf8.ValidString(got.Text) || got.Fingerprint != "f" {
		t.Fatalf("%v %v %d", err, ok, len(got.Text))
	}
}

func TestSummaryMissingIsPending(t *testing.T) {
	nb := newNB(t)
	if _, ok, err := nb.Summary(notebook.NewID()); err != nil || ok {
		t.Fatalf("%v %v", err, ok)
	}
	if _, _, err := nb.Summary("../x"); err == nil {
		t.Fatal("bad id accepted")
	}
}

func TestFingerprintsChangeWithContent(t *testing.T) {
	docs := []extract.Document{{Source: "a.go", Text: "package a"}, {Source: "b.go", Text: "package b"}}
	base := FolderFingerprint(docs)
	reversed := []extract.Document{docs[1], docs[0]}
	if FolderFingerprint(reversed) != base {
		t.Fatal("order changed the fingerprint")
	}
	changed := []extract.Document{{Source: "a.go", Text: "package A"}, docs[1]}
	if FolderFingerprint(changed) == base {
		t.Fatal("content change not seen")
	}
	if TextFingerprint("x") == TextFingerprint("y") {
		t.Fatal("text fingerprints collide")
	}
	path := filepath.Join(t.TempDir(), "f.txt")
	_ = os.WriteFile(path, []byte("one"), 0o600)
	a, err := FileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path, []byte("two"), 0o600)
	if b, _ := FileFingerprint(path); a == b {
		t.Fatal("file change not seen")
	}
}
