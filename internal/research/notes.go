package research

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// Note is text kept in the notebook, written by hand or saved from an answer.
// Include puts it into conversations.
type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Origin    string `json:"origin"`
	Include   bool   `json:"include"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

const maxTitle = 200

// Notes lists the notes, oldest first.
func (n *Notebook) Notes() ([]Note, error) {
	entries, err := os.ReadDir(filepath.Join(n.Dir, notesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return []Note{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	out := []Note{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !itemID.MatchString(id) {
			continue
		}
		var note Note
		if err := readJSON(filepath.Join(n.Dir, notesDir, e.Name()), MaxNote*2, &note); err == nil {
			out = append(out, note)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// SaveNote creates a note (empty ID) or replaces one.
func (n *Notebook) SaveNote(note Note) (Note, error) {
	if len(note.Text) > MaxNote || !utf8.ValidString(note.Text) || len(note.Title) > maxTitle ||
		!utf8.ValidString(note.Title) {
		return Note{}, fmt.Errorf("a note is at most %d KiB of text with a title of at most %d characters",
			MaxNote>>10, maxTitle)
	}
	if note.Origin != OriginAI {
		note.Origin = OriginHuman
	}
	stamp := now()
	if note.ID == "" {
		count, err := n.noteCount()
		if err != nil {
			return Note{}, err
		}
		if count >= MaxNotes {
			return Note{}, fmt.Errorf("%w: at most %d notes", ErrLimit, MaxNotes)
		}
		note.ID, note.CreatedAt = notebook.NewID(), stamp
	} else if !itemID.MatchString(note.ID) {
		return Note{}, fmt.Errorf("note %q: %w", note.ID, ErrNotFound)
	}
	note.UpdatedAt = stamp
	if err := os.MkdirAll(filepath.Join(n.Dir, notesDir), 0o700); err != nil {
		return Note{}, fmt.Errorf("notes folder: %w", err)
	}
	return note, writeJSON(filepath.Join(n.Dir, notesDir, note.ID+".json"), note)
}

// noteCount counts note files without reading them.
func (n *Notebook) noteCount() (int, error) {
	entries, err := os.ReadDir(filepath.Join(n.Dir, notesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("list notes: %w", err)
	}
	count := 0
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), ".json"); ok && itemID.MatchString(id) {
			count++
		}
	}
	return count, nil
}

// DeleteNote removes a note.
func (n *Notebook) DeleteNote(id string) error {
	if !itemID.MatchString(id) {
		return fmt.Errorf("note %q: %w", id, ErrNotFound)
	}
	err := os.Remove(filepath.Join(n.Dir, notesDir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("note %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}
