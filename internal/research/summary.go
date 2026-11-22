package research

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/extract"
)

// MaxSummary bounds a stored summary (bytes).
const MaxSummary = 8 << 10

// Summary statuses on disk; a source without a summary file is pending.
const (
	StatusReady  = "ready"
	StatusFailed = "failed"
)

// Summary is a source's stored summary and the content it was made from.
type Summary struct {
	SourceID    string `json:"sourceId"`
	Fingerprint string `json:"fingerprint"`
	Model       string `json:"model,omitempty"`
	CreatedAt   string `json:"createdAt"`
	Text        string `json:"text,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func (n *Notebook) summaryPath(sourceID string) (string, error) {
	if !itemID.MatchString(sourceID) {
		return "", fmt.Errorf("source %q: %w", sourceID, ErrNotFound)
	}
	return filepath.Join(n.Dir, summariesDir, sourceID+".json"), nil
}

// Summary returns a source's summary; ok is false when there is none yet.
func (n *Notebook) Summary(sourceID string) (s Summary, ok bool, err error) {
	path, err := n.summaryPath(sourceID)
	if err != nil {
		return Summary{}, false, err
	}
	err = readJSON(path, 4*MaxSummary, &s)
	if errors.Is(err, fs.ErrNotExist) {
		return Summary{}, false, nil
	}
	if err != nil {
		return Summary{}, false, err
	}
	return s, true, nil
}

// SaveSummary stores a summary, its text cut to MaxSummary at a rune boundary.
func (n *Notebook) SaveSummary(s Summary) error {
	path, err := n.summaryPath(s.SourceID)
	if err != nil {
		return err
	}
	if len(s.Text) > MaxSummary {
		cut := MaxSummary
		for cut > 0 && !utf8.RuneStart(s.Text[cut]) {
			cut--
		}
		s.Text = s.Text[:cut]
	}
	if s.CreatedAt == "" {
		s.CreatedAt = now()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("summaries folder: %w", err)
	}
	return writeJSON(path, s)
}

// FileFingerprint is the sha256 of a file's bytes.
func FileFingerprint(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: a source the user added
	if err != nil {
		return "", fmt.Errorf("fingerprint %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxCopy)); err != nil {
		return "", fmt.Errorf("fingerprint %s: %w", filepath.Base(path), err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FolderFingerprint covers exactly what extraction read from a folder: each
// document's source, size and content, in source order.
func FolderFingerprint(docs []extract.Document) string {
	lines := make([]string, 0, len(docs))
	for _, d := range docs {
		lines = append(lines, d.Source+"\x00"+strconv.Itoa(len(d.Text))+"\x00"+TextFingerprint(d.Text))
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, l := range lines {
		_, _ = io.WriteString(h, l+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TextFingerprint is the sha256 of text (MCP resources).
func TextFingerprint(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
