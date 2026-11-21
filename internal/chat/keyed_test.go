package chat

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
)

var testKey = regexp.MustCompile(`^\d{8}T\d{6}-[0-9a-f]{6}$`)

func TestKeyedStoreEnsureAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "threads")
	st, err := OpenKeyed(dir, testKey.MatchString)
	if err != nil {
		t.Fatal(err)
	}
	const key = "20261121T100000-abcdef"
	a, err := st.Ensure(key, "m", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.Ensure(key, "other", "high")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != key || b.ID != key || b.Model != "m" || b.CreatedAt != a.CreatedAt {
		t.Fatalf("ensure: %+v then %+v", a, b)
	}
	if _, err := os.Stat(filepath.Join(dir, key+".json")); err != nil {
		t.Fatalf("thread file: %v", err)
	}
	if _, err := st.Get("../x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad key: %v", err)
	}
	if _, err := st.Ensure("0123456789abcdef", "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("chat id is not a thread key: %v", err)
	}
	if _, err := st.Update(key, func(s *Session) error {
		s.Messages = append(s.Messages, Message{ID: NewMessageID(), Role: RoleUser, Content: "hi"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	release, err := st.Acquire(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Acquire(key); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
	release()
	if _, err := st.Acquire("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("acquire bad key: %v", err)
	}
}

func TestBuildSkipsNotes(t *testing.T) {
	sess := Session{Messages: []Message{
		{Role: RoleUser, Content: "q1"},
		{Role: RoleAssistant, Content: "a1"},
		{Role: RoleNote, Content: "Saved as v2"},
		{Role: RoleUser, Content: "q2"},
	}}
	for _, m := range Build(sess, nil, nil, 10000) {
		if m.Content == "Saved as v2" || m.Role == RoleNote {
			t.Fatalf("note sent to the model: %+v", m)
		}
	}
	if got := Build(sess, nil, nil, 10000); got[len(got)-1].Role != llm.RoleUser {
		t.Fatalf("last: %+v", got[len(got)-1])
	}
}
