// Package audit is Tessivo's tamper-evident, append-only audit log. Every run,
// model attempt, tool call and saved output is one JSON line whose hash covers
// the previous line, so editing, deleting or reordering entries is detectable
// with Verify. Entries never hold secrets or raw prompts: values are redacted
// and prompts are recorded as a SHA-256 digest and length.
package audit

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// Entry kinds.
const (
	KindRunStart     = "run.start"
	KindModelAttempt = "model.attempt"
	KindRunFinish    = "run.finish"
	KindOutputSaved  = "output.saved"
	KindToolCall     = "tool.call"
	KindToolDecision = "tool.decision"
	KindRunResume    = "run.resume"
	KindSuggestApply = "suggestion.apply"
	KindOutputSent   = "output.sent" // an output pushed or published to an integration
)

// Limits keep entries small and lines bounded.
const (
	MaxStringLen = 2048     // longer string values are truncated
	MaxLineBytes = 64 << 10 // longer entries are rejected
)

// genesis is the "previous hash" of the first entry.
var genesis = hex.EncodeToString(make([]byte, sha256.Size))

// Entry is one audit record.
type Entry struct {
	Seq   uint64         `json:"seq"`
	Time  string         `json:"time"`
	Kind  string         `json:"kind"`
	Actor string         `json:"actor,omitempty"`
	RunID string         `json:"run_id,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
	Prev  string         `json:"prev"`
	Hash  string         `json:"hash,omitempty"`
}

// digest hashes the entry with Hash cleared. encoding/json sorts map keys, so
// the encoding is canonical for the value types we store.
func (e Entry) digest() (string, error) {
	e.Hash = ""
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("encode audit entry: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Log appends entries to one file. It is safe for concurrent use.
type Log struct {
	mu     sync.Mutex
	f      *os.File
	actor  string
	seq    uint64
	last   string
	secret []string
	now    func() time.Time
}

// Open opens (or creates, 0600) the log at path and resumes its hash chain.
// actor names the writer (cli, desktop, scheduler). secretValues are redacted
// from every entry in addition to built-in key patterns.
func Open(path, actor string, secretValues ...string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("audit dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600) //nolint:gosec // G304: app-chosen path
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	l := &Log{f: f, actor: actor, last: genesis, secret: secretValues, now: time.Now}
	if err := l.resume(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return l, nil
}

// resume reads only the last line to continue the chain, so opening a large
// log stays O(1).
func (l *Log) resume() error {
	info, err := l.f.Stat()
	if err != nil {
		return fmt.Errorf("stat audit log: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}
	n := min(info.Size(), int64(MaxLineBytes)+1)
	buf := make([]byte, n)
	if _, err := l.f.ReadAt(buf, info.Size()-n); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read audit tail: %w", err)
	}
	buf = bytes.TrimRight(buf, "\n")
	if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
		buf = buf[i+1:]
	}
	var e Entry
	if err := json.Unmarshal(buf, &e); err != nil {
		return fmt.Errorf("audit log tail is corrupt: %w", err)
	}
	l.seq, l.last = e.Seq, e.Hash
	return nil
}

// Append writes one entry and fsyncs it.
func (l *Log) Append(kind, runID string, data map[string]any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := Entry{
		Seq: l.seq + 1, Time: l.now().UTC().Format(time.RFC3339Nano), Kind: kind, Actor: l.actor,
		RunID: runID, Data: sanitize(data, l.secret), Prev: l.last,
	}
	h, err := e.digest()
	if err != nil {
		return err
	}
	e.Hash = h
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}
	if len(b) > MaxLineBytes {
		return fmt.Errorf("audit entry is %d bytes (max %d)", len(b), MaxLineBytes)
	}
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	if err := l.f.Sync(); err != nil {
		return fmt.Errorf("sync audit log: %w", err)
	}
	l.seq, l.last = e.Seq, h
	return nil
}

// Close closes the file.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.f.Close(); err != nil {
		return fmt.Errorf("close audit log: %w", err)
	}
	return nil
}

// ErrTampered reports a broken hash chain.
var ErrTampered = errors.New("audit log failed verification")

// Verify checks every entry's hash and chain link. It returns the number of
// valid entries read before the first problem.
func Verify(r io.Reader) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), MaxLineBytes+1)
	prev, n, want := genesis, 0, uint64(1)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return n, fmt.Errorf("%w: line %d is not valid JSON", ErrTampered, n+1)
		}
		sum, err := e.digest()
		if err != nil {
			return n, err
		}
		switch {
		case e.Seq != want:
			return n, fmt.Errorf("%w: line %d has seq %d", ErrTampered, n+1, e.Seq)
		case e.Prev != prev:
			return n, fmt.Errorf("%w: entry %d does not link to the previous entry", ErrTampered, e.Seq)
		case e.Hash != sum:
			return n, fmt.Errorf("%w: entry %d was modified", ErrTampered, e.Seq)
		}
		prev, n, want = e.Hash, n+1, want+1
	}
	if err := sc.Err(); err != nil {
		return n, fmt.Errorf("%w: %w", ErrTampered, err)
	}
	return n, nil
}

// VerifyFile verifies the log at path.
func VerifyFile(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // G304: user-chosen log to verify
	if err != nil {
		return 0, fmt.Errorf("open audit log: %w", err)
	}
	defer func() { _ = f.Close() }()
	return Verify(f)
}

// Digest returns the SHA-256 hex digest of s, for recording prompts without
// storing their text.
func Digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sanitize(data map[string]any, values []string) map[string]any {
	if data == nil {
		return nil
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = clean(v, values)
	}
	return out
}

func clean(v any, values []string) any {
	switch x := v.(type) {
	case string:
		s := secrets.RedactPatterns(secrets.Redact(x, values...))
		if len(s) > MaxStringLen {
			s = s[:MaxStringLen] + "…"
		}
		return s
	case error:
		return clean(x.Error(), values)
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = clean(s, values)
		}
		return out
	case map[string]any:
		return sanitize(x, values)
	case bool, int, int64, float64, uint64, nil:
		return x
	default:
		return clean(fmt.Sprint(x), values)
	}
}
