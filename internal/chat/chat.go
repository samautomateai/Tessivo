// Package chat stores chat sessions and the global chat memory as JSON files
// (0600, atomic writes) and assembles the messages sent to the model. It knows
// nothing about providers or transports; the service layer drives it.
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/llm"
)

// Roles.
const (
	RoleUser      = llm.RoleUser
	RoleAssistant = llm.RoleAssistant
	RoleNote      = "note" // stored for the reader only; never sent to the model
)

// Attachment kinds.
const (
	AttachPath     = "path"     // absolute file or folder path
	AttachNotebook = "notebook" // a notebook's enabled context
	AttachOutput   = "output"   // a saved output, ref "notebookID/outputID"
)

// Limits. Everything read from disk or the API is bounded.
const (
	MaxTitle          = 200
	MaxMessageBytes   = 64 << 10  // one user message
	MaxReplyBytes     = 512 << 10 // one assistant reply (longer replies are cut)
	MaxMessages       = 2000      // per session
	MaxAttachments    = 20        // per message
	MaxSessions       = 5000      // listed per store
	MaxSessionFile    = 32 << 20  // bytes read for one session
	MaxMemory         = 50
	MaxMemoryItem     = 1000
	MaxQuery          = 200
	previewRunes      = 120
	autoTitleRunes    = 60
	idBytes           = 8
	attachmentRefMax  = 4096
	attachmentLabelMx = 200
)

// Errors callers map to HTTP statuses.
var (
	ErrNotFound = errors.New("chat session not found")
	ErrBusy     = errors.New("a reply is already streaming in this session")
	ErrInvalid  = errors.New("invalid chat request")
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Attachment is context attached to a user message.
type Attachment struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Label  string `json:"label,omitempty"`
	Tokens int    `json:"tokens,omitempty"`
}

// Message is one turn.
type Message struct {
	ID          string       `json:"id"`
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	CreatedAt   string       `json:"createdAt"`
	Model       string       `json:"model,omitempty"`   // label of the model that answered
	Stopped     bool         `json:"stopped,omitempty"` // reply cut short by the user or an error
	Error       string       `json:"error,omitempty"`
	Tokens      int          `json:"tokens,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Session is a conversation with its settings.
type Session struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Pinned         bool      `json:"pinned,omitempty"`
	Archived       bool      `json:"archived,omitempty"`
	Model          string    `json:"model,omitempty"`          // alias or provider/model; "" uses the default
	ThinkingEffort string    `json:"thinkingEffort,omitempty"` // off, low, medium, high; "" uses the default
	CreatedAt      string    `json:"createdAt"`
	UpdatedAt      string    `json:"updatedAt"`
	Messages       []Message `json:"messages"`
}

// Summary is a session without its messages, for lists.
type Summary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Pinned       bool   `json:"pinned,omitempty"`
	Archived     bool   `json:"archived,omitempty"`
	Model        string `json:"model,omitempty"`
	UpdatedAt    string `json:"updatedAt"`
	MessageCount int    `json:"messageCount"`
	Preview      string `json:"preview"`
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

func newID() string {
	b := make([]byte, idBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewMessageID returns a random message ID.
func NewMessageID() string { return newID() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ValidID reports whether id is a session ID this package issues.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Summarize returns the list view of s.
func (s Session) Summarize() Summary {
	sum := Summary{
		ID: s.ID, Title: s.Title, Pinned: s.Pinned, Archived: s.Archived, Model: s.Model,
		UpdatedAt: s.UpdatedAt, MessageCount: len(s.Messages),
	}
	if n := len(s.Messages); n > 0 {
		sum.Preview = truncateRunes(oneLine(s.Messages[n-1].Content), previewRunes)
	}
	return sum
}

// Matches reports whether q (lowercase) appears in the title or any message.
func (s Session) Matches(q string) bool {
	if q == "" || strings.Contains(strings.ToLower(s.Title), q) {
		return true
	}
	for _, m := range s.Messages {
		if strings.Contains(strings.ToLower(m.Content), q) {
			return true
		}
	}
	return false
}

// AutoTitle names an untitled session after its first user message.
func (s *Session) AutoTitle() {
	if s.Title != "" {
		return
	}
	for _, m := range s.Messages {
		if m.Role == RoleUser {
			s.Title = truncateRunes(oneLine(m.Content), autoTitleRunes)
			return
		}
	}
}

// CheckTitle validates a session title.
func CheckTitle(t string) error {
	if utf8.RuneCountInString(t) > MaxTitle || strings.ContainsAny(t, "\n\r") {
		return invalid("titles are one line of at most %d characters", MaxTitle)
	}
	return nil
}

// CheckMessage validates a user message and its attachments.
func CheckMessage(text string, atts []Attachment) error {
	if strings.TrimSpace(text) == "" {
		return invalid("message is empty")
	}
	if len(text) > MaxMessageBytes {
		return invalid("message is longer than %d KiB", MaxMessageBytes>>10)
	}
	if !utf8.ValidString(text) {
		return invalid("message is not valid UTF-8")
	}
	if len(atts) > MaxAttachments {
		return invalid("at most %d attachments per message", MaxAttachments)
	}
	for _, a := range atts {
		if err := a.check(); err != nil {
			return err
		}
	}
	return nil
}

func (a Attachment) check() error {
	switch a.Kind {
	case AttachPath, AttachNotebook, AttachOutput:
	default:
		return invalid("attachment kind %q: want path, notebook or output", a.Kind)
	}
	if a.Ref == "" || len(a.Ref) > attachmentRefMax || strings.ContainsRune(a.Ref, 0) {
		return invalid("attachment ref is empty or too long")
	}
	if len(a.Label) > attachmentLabelMx {
		return invalid("attachment label is too long")
	}
	return nil
}

// CleanMemory trims facts, drops empty ones and enforces the limits.
func CleanMemory(facts []string) ([]string, error) {
	if len(facts) > MaxMemory {
		return nil, invalid("at most %d memory items", MaxMemory)
	}
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if len(f) > MaxMemoryItem || !utf8.ValidString(f) {
			return nil, invalid("memory items are limited to %d characters of valid text", MaxMemoryItem)
		}
		out = append(out, f)
	}
	return out, nil
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
