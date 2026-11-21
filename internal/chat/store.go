package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	sessionsDir = "sessions"
	memoryFile  = "memory.json"
)

// Store keeps sessions in Dir/sessions/<id>.json and memory in Dir/memory.json.
// Writes are serialized per store directory, so several Store values for the
// same directory (CLI and server in one process) stay consistent.
type Store struct {
	Dir string

	flat  bool              // sessions live directly in Dir (keyed stores)
	valid func(string) bool // key check for keyed stores; nil uses ValidID
}

// locks holds one mutex per store directory plus the sessions with a reply in
// flight, shared across Store values.
var locks = struct {
	sync.Mutex
	dirs map[string]*sync.Mutex
	busy map[string]bool
}{dirs: map[string]*sync.Mutex{}, busy: map[string]bool{}}

// Open creates the store folders (0700) if needed.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("chat store: empty directory")
	}
	if err := os.MkdirAll(filepath.Join(dir, sessionsDir), 0o700); err != nil {
		return nil, fmt.Errorf("chat store: %w", err)
	}
	return &Store{Dir: dir}, nil
}

// OpenKeyed opens a store whose sessions are named by caller-chosen keys (for
// example output IDs) and kept directly in dir as <key>.json. valid decides
// which keys are accepted; it must reject anything path-like.
func OpenKeyed(dir string, valid func(string) bool) (*Store, error) {
	if dir == "" || valid == nil {
		return nil, errors.New("chat store: empty directory or key check")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("chat store: %w", err)
	}
	return &Store{Dir: dir, flat: true, valid: valid}, nil
}

func (s *Store) validID(id string) bool {
	if s.valid != nil {
		return s.valid(id)
	}
	return ValidID(id)
}

// Ensure returns the keyed session, creating an empty one if it is missing.
func (s *Store) Ensure(key, model, effort string) (Session, error) {
	if !s.validID(key) {
		return Session{}, ErrNotFound
	}
	m := s.mu()
	m.Lock()
	defer m.Unlock()
	sess, err := s.Get(key)
	if !errors.Is(err, ErrNotFound) {
		return sess, err
	}
	t := now()
	sess = Session{ID: key, Model: model, ThinkingEffort: effort, CreatedAt: t, UpdatedAt: t, Messages: []Message{}}
	return sess, writeJSON(s.path(key), sess)
}

func (s *Store) mu() *sync.Mutex {
	locks.Lock()
	defer locks.Unlock()
	m, ok := locks.dirs[s.Dir]
	if !ok {
		m = &sync.Mutex{}
		locks.dirs[s.Dir] = m
	}
	return m
}

// Acquire marks a session as streaming; the returned func releases it.
func (s *Store) Acquire(id string) (func(), error) {
	if !s.validID(id) {
		return nil, ErrNotFound
	}
	key := filepath.Join(s.Dir, id)
	locks.Lock()
	defer locks.Unlock()
	if locks.busy[key] {
		return nil, ErrBusy
	}
	locks.busy[key] = true
	return func() {
		locks.Lock()
		delete(locks.busy, key)
		locks.Unlock()
	}, nil
}

func (s *Store) path(id string) string {
	if s.flat {
		return filepath.Join(s.Dir, id+".json")
	}
	return filepath.Join(s.Dir, sessionsDir, id+".json")
}

// Create starts an empty session.
func (s *Store) Create(title, model, effort string) (Session, error) {
	if err := CheckTitle(title); err != nil {
		return Session{}, err
	}
	t := now()
	sess := Session{
		ID: newID(), Title: strings.TrimSpace(title), Model: model, ThinkingEffort: effort,
		CreatedAt: t, UpdatedAt: t, Messages: []Message{},
	}
	m := s.mu()
	m.Lock()
	defer m.Unlock()
	return sess, writeJSON(s.path(sess.ID), sess)
}

// Get loads a session.
func (s *Store) Get(id string) (Session, error) {
	if !s.validID(id) {
		return Session{}, ErrNotFound
	}
	var sess Session
	if err := readJSON(s.path(id), MaxSessionFile, &sess); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	if sess.ID != id {
		return Session{}, fmt.Errorf("session %s: file holds id %q", id, sess.ID)
	}
	return sess, nil
}

// Update loads a session, applies edit and saves it atomically. edit returning
// an error leaves the file untouched.
func (s *Store) Update(id string, edit func(*Session) error) (Session, error) {
	m := s.mu()
	m.Lock()
	defer m.Unlock()
	sess, err := s.Get(id)
	if err != nil {
		return Session{}, err
	}
	if err := edit(&sess); err != nil {
		return Session{}, err
	}
	if len(sess.Messages) > MaxMessages {
		return Session{}, invalid("session is full (%d messages); start a new one", MaxMessages)
	}
	sess.UpdatedAt = now()
	return sess, writeJSON(s.path(id), sess)
}

// Delete removes a session.
func (s *Store) Delete(id string) error {
	if !s.validID(id) {
		return ErrNotFound
	}
	m := s.mu()
	m.Lock()
	defer m.Unlock()
	if err := os.Remove(s.path(id)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Filter narrows List.
type Filter struct {
	Query    string // case-insensitive, title or message text
	Archived bool   // true lists archived sessions, false the active ones
}

// List returns matching sessions, pinned first, then most recently updated.
// Unreadable files are skipped so one bad file cannot hide the rest.
func (s *Store) List(f Filter) ([]Summary, error) {
	if len(f.Query) > MaxQuery {
		return nil, invalid("search text is longer than %d characters", MaxQuery)
	}
	entries, err := os.ReadDir(filepath.Join(s.Dir, sessionsDir))
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	out := []Summary{}
	for i, e := range entries {
		if i >= MaxSessions {
			break
		}
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		sess, err := s.Get(id)
		if err != nil || sess.Archived != f.Archived || !sess.Matches(q) {
			continue
		}
		out = append(out, sess.Summarize())
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

// Memory returns the global chat memory.
func (s *Store) Memory() ([]string, error) {
	var mem struct {
		Facts []string `json:"facts"`
	}
	err := readJSON(filepath.Join(s.Dir, memoryFile), 1<<20, &mem)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return CleanMemory(mem.Facts)
}

// SetMemory replaces the global chat memory and returns the cleaned facts.
func (s *Store) SetMemory(facts []string) ([]string, error) {
	clean, err := CleanMemory(facts)
	if err != nil {
		return nil, err
	}
	m := s.mu()
	m.Lock()
	defer m.Unlock()
	return clean, writeJSON(filepath.Join(s.Dir, memoryFile), map[string]any{"facts": clean})
}

func readJSON(path string, limit int64, v any) error {
	f, err := os.Open(path) // #nosec G304 -- path is built from a validated ID inside the store
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if int64(len(b)) > limit {
		return fmt.Errorf("%s is larger than %d bytes", filepath.Base(path), limit)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(append(b, '\n'))
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return errors.Join(fmt.Errorf("write %s: %w", filepath.Base(path), err), os.Remove(name))
	}
	if err := os.Rename(name, path); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", filepath.Base(path), err), os.Remove(name))
	}
	return nil
}
