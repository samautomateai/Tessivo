package research

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newNB(t *testing.T) *Notebook {
	t.Helper()
	nb, err := Create(filepath.Join(t.TempDir(), "nb"), "nb", "Board", "Board papers")
	if err != nil {
		t.Fatal(err)
	}
	return nb
}

func TestCreateOpenList(t *testing.T) {
	root := t.TempDir()
	a, err := Create(filepath.Join(root, "a"), "a", "A", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"sources", "summaries", "notes", "chats"} {
		if fi, err := os.Stat(filepath.Join(a.Dir, sub)); err != nil || !fi.IsDir() {
			t.Fatalf("%s: %v", sub, err)
		}
	}
	time.Sleep(1100 * time.Millisecond) // CreatedAt has second resolution
	if _, err := Create(filepath.Join(root, "b"), "b", "B", ""); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "junk"), 0o700)
	list, err := List(root)
	if err != nil || len(list) != 2 || list[0].Manifest.ID != "b" {
		t.Fatalf("list: %v %d", err, len(list))
	}
	if _, err := Create(a.Dir, "a", "again", ""); err == nil {
		t.Fatal("want an error for an existing notebook")
	}
	got, err := Open(a.Dir)
	if err != nil || got.Manifest.Name != "A" {
		t.Fatalf("open: %v %+v", err, got)
	}
}

func TestAddPathFileAndFolder(t *testing.T) {
	nb := newNB(t)
	file := filepath.Join(t.TempDir(), "minutes.md")
	_ = os.WriteFile(file, []byte("# Minutes"), 0o600)
	s, err := nb.AddPath(file)
	if err != nil || s.Kind != KindFile || s.Level != LevelFull || s.LevelSet || s.InPlace {
		t.Fatalf("file: %v %+v", err, s)
	}
	if b, err := os.ReadFile(nb.SourcePath(s)); err != nil || string(b) != "# Minutes" ||
		!strings.HasPrefix(nb.SourcePath(s), filepath.Join(nb.Dir, "sources")) {
		t.Fatalf("copy: %v %q", err, nb.SourcePath(s))
	}
	dir := t.TempDir()
	f, err := nb.AddPath(dir)
	if err != nil || f.Kind != KindFolder || !f.InPlace || nb.SourcePath(f) != dir {
		t.Fatalf("folder: %v %+v", err, f)
	}
	again, _ := Open(nb.Dir)
	if len(again.Manifest.Sources) != 2 {
		t.Fatalf("not saved: %+v", again.Manifest.Sources)
	}
}

func TestAddMCPValidates(t *testing.T) {
	nb := newNB(t)
	if _, err := nb.AddMCP("fake", ""); err == nil {
		t.Fatal("empty uri accepted")
	}
	if _, err := nb.AddMCP("fake", strings.Repeat("u", 2049)); err == nil {
		t.Fatal("long uri accepted")
	}
	s, err := nb.AddMCP("fake", "file:///n.md")
	if err != nil || s.Kind != KindMCP || nb.SourcePath(s) != "" || s.Label != "file:///n.md" {
		t.Fatalf("%v %+v", err, s)
	}
}

func TestSetLevel(t *testing.T) {
	nb := newNB(t)
	s, _ := nb.AddMCP("fake", "file:///n.md")
	if err := nb.SetLevel(s.ID, LevelSummary, false); !errors.Is(err, ErrNoSummary) {
		t.Fatalf("summary without one: %v", err)
	}
	if err := nb.SetLevel(s.ID, LevelOff, false); err != nil {
		t.Fatal(err)
	}
	if got := nb.Manifest.Sources[0]; got.Level != LevelOff || !got.LevelSet {
		t.Fatalf("%+v", got)
	}
	if err := nb.SetLevel(s.ID, "loud", true); err == nil {
		t.Fatal("unknown level accepted")
	}
	if err := nb.SetLevel("nope", LevelOff, true); err == nil {
		t.Fatal("unknown source accepted")
	}
}

func TestRemoveSourceDeletesFiles(t *testing.T) {
	nb := newNB(t)
	file := filepath.Join(t.TempDir(), "a.txt")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	s, _ := nb.AddPath(file)
	summary := filepath.Join(nb.Dir, "summaries", s.ID+".json")
	_ = os.WriteFile(summary, []byte("{}"), 0o600)
	copied := nb.SourcePath(s)
	if err := nb.RemoveSource(s.ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{copied, summary} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s left behind", p)
		}
	}
	if len(nb.Manifest.Sources) != 0 {
		t.Fatal("still listed")
	}
}

func TestNotesCRUDAndCaps(t *testing.T) {
	nb := newNB(t)
	n, err := nb.SaveNote(Note{Title: "T", Text: "body", Origin: OriginHuman, Include: true})
	if err != nil || n.ID == "" || n.CreatedAt == "" {
		t.Fatalf("%v %+v", err, n)
	}
	n.Text = "edited"
	if _, err := nb.SaveNote(n); err != nil {
		t.Fatal(err)
	}
	list, _ := nb.Notes()
	if len(list) != 1 || list[0].Text != "edited" {
		t.Fatalf("%+v", list)
	}
	if _, err := nb.SaveNote(Note{Text: strings.Repeat("x", MaxNote+1)}); err == nil {
		t.Fatal("oversized note accepted")
	}
	if err := nb.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if err := nb.DeleteNote("../x"); err == nil {
		t.Fatal("bad id accepted")
	}
}

func TestNoteLimit(t *testing.T) {
	nb := newNB(t)
	for range MaxNotes {
		if _, err := nb.SaveNote(Note{Text: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := nb.SaveNote(Note{Text: "x"}); !errors.Is(err, ErrLimit) {
		t.Fatalf("501st note: %v", err)
	}
}

func TestSourceLimit(t *testing.T) {
	nb := newNB(t)
	for i := range MaxSources {
		if _, err := nb.AddMCP("fake", "file:///"+string(rune('a'+i%26))+strings.Repeat("x", i/26)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := nb.AddMCP("fake", "file:///last"); !errors.Is(err, ErrLimit) {
		t.Fatalf("201st source: %v", err)
	}
}

func TestOldManifestLoads(t *testing.T) {
	nb := newNB(t)
	m := map[string]any{"id": "nb", "name": "Old", "createdAt": "", "sources": []map[string]any{
		{"id": "20261122T100000-abcdef", "label": "x", "kind": "file", "path": "sources/x", "level": "weird"},
	}}
	b, _ := json.Marshal(m)
	_ = os.WriteFile(filepath.Join(nb.Dir, ManifestFile), b, 0o600)
	got, err := Open(nb.Dir)
	if err != nil || got.Manifest.Sources[0].Level != LevelFull || got.Manifest.Sources[0].LevelSet {
		t.Fatalf("%v %+v", err, got.Manifest.Sources)
	}
}
