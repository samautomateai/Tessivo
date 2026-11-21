package notebook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// Suggestion is a proposed change to the notebook (pinned memory or the
// prompt) that came out of an output's thread. It changes nothing until the
// user applies it.
type Suggestion struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // SuggestMemory or SuggestPrompt
	Text      string `json:"text"`
	Reason    string `json:"reason,omitempty"`
	MessageID string `json:"messageId,omitempty"` // thread message that produced it
	Base      string `json:"base,omitempty"`      // sha256 hex of the target prompt when made (prompt only)
	Status    string `json:"status"`              // SuggestOpen, SuggestApplied or SuggestDismissed
	CreatedAt string `json:"createdAt"`
	DecidedAt string `json:"decidedAt,omitempty"`
	// CreatedPrompt names the saved prompt applying this suggestion created.
	CreatedPrompt string `json:"createdPrompt,omitempty"`
}

// Suggestion kinds and statuses.
const (
	SuggestMemory    = "memory"
	SuggestPrompt    = "prompt"
	SuggestOpen      = "open"
	SuggestApplied   = "applied"
	SuggestDismissed = "dismissed"
)

// Suggestion limits.
const (
	MaxOpenSuggestions = 20
	MaxPromptText      = 16 << 10
	maxReason          = 1000
	maxSuggestionsFile = 1 << 20
)

// Check validates the kind and text. maxMemoryItem bounds a memory suggestion.
func (s Suggestion) Check(maxMemoryItem int) error {
	limit := 0
	switch s.Kind {
	case SuggestMemory:
		limit = maxMemoryItem
	case SuggestPrompt:
		limit = MaxPromptText
	default:
		return fmt.Errorf("suggestion kind %q: want memory or prompt", s.Kind)
	}
	if strings.TrimSpace(s.Text) == "" {
		return errors.New("suggestion text is empty")
	}
	if len(s.Text) > limit || !utf8.ValidString(s.Text) {
		return fmt.Errorf("suggestion text is limited to %d bytes of valid text", limit)
	}
	if len(s.Reason) > maxReason || !utf8.ValidString(s.Reason) {
		return fmt.Errorf("suggestion reason is limited to %d bytes of valid text", maxReason)
	}
	return nil
}

// suggestionLocks serializes suggestion updates per notebook folder.
var suggestionLocks = struct {
	sync.Mutex
	dirs map[string]*sync.Mutex
}{dirs: map[string]*sync.Mutex{}}

func (n *Notebook) suggestionLock() *sync.Mutex {
	suggestionLocks.Lock()
	defer suggestionLocks.Unlock()
	m, ok := suggestionLocks.dirs[n.Dir]
	if !ok {
		m = &sync.Mutex{}
		suggestionLocks.dirs[n.Dir] = m
	}
	return m
}

func (n *Notebook) suggestionsPath(outputID string) (string, error) {
	if !ValidID(outputID) {
		return "", fmt.Errorf("output id %q is not valid", outputID)
	}
	return filepath.Join(n.Dir, SuggestionsDir, outputID+".json"), nil
}

// Suggestions returns an output's suggestions, oldest first (empty if none).
func (n *Notebook) Suggestions(outputID string) ([]Suggestion, error) {
	path, err := n.suggestionsPath(outputID)
	if err != nil {
		return nil, err
	}
	return readSuggestions(path)
}

func readSuggestions(path string) ([]Suggestion, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is built from a validated output ID
	if errors.Is(err, fs.ErrNotExist) {
		return []Suggestion{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open suggestions: %w", err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxSuggestionsFile+1))
	if err != nil {
		return nil, fmt.Errorf("read suggestions: %w", err)
	}
	if len(b) > maxSuggestionsFile {
		return nil, fmt.Errorf("suggestions file is larger than %d bytes", maxSuggestionsFile)
	}
	var list []Suggestion
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("parse suggestions: %w", err)
	}
	return list, nil
}

// UpdateSuggestions loads an output's suggestions, applies edit and saves the
// result atomically. Updates for one notebook are serialized. A result with
// more than MaxOpenSuggestions open items is rejected and nothing is written.
func (n *Notebook) UpdateSuggestions(outputID string, edit func([]Suggestion) ([]Suggestion, error)) error {
	path, err := n.suggestionsPath(outputID)
	if err != nil {
		return err
	}
	m := n.suggestionLock()
	m.Lock()
	defer m.Unlock()
	list, err := readSuggestions(path)
	if err != nil {
		return err
	}
	if list, err = edit(list); err != nil {
		return err
	}
	open := 0
	for _, s := range list {
		if s.Status == SuggestOpen {
			open++
		}
	}
	if open > MaxOpenSuggestions {
		return fmt.Errorf("%w: at most %d open suggestions per output", ErrTooManySuggestions, MaxOpenSuggestions)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create suggestions folder: %w", err)
	}
	return writeJSON(path, list)
}

// ErrTooManySuggestions is returned when an output already has the maximum
// number of open suggestions.
var ErrTooManySuggestions = errors.New("too many open suggestions")
