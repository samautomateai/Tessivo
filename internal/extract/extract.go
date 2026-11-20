// Package extract turns context items (files, folders, code bases, pasted
// text) into plain-text documents a model can read. Each format has a small
// extractor; everything uses the standard library.
package extract

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Document is extracted text from one source.
type Document struct {
	Source string `json:"source"` // file path or label
	Kind   string `json:"kind"`   // text, markdown, code, csv, json
	Text   string `json:"text"`
}

// Tokens estimates the token count (≈ 4 characters per token).
func (d Document) Tokens() int { return EstimateTokens(d.Text) }

// EstimateTokens is a fast, provider-neutral token estimate.
func EstimateTokens(s string) int { return (utf8.RuneCountInString(s) + 3) / 4 }

// Office document kinds.
const (
	KindDOCX = "docx"
	KindXLSX = "xlsx"
	KindPPTX = "pptx"
)

// ErrUnsupported is returned for formats with no extractor yet.
var ErrUnsupported = errors.New("unsupported format")

// MaxFileBytes caps the size of a single file read for context.
const MaxFileBytes = 2 << 20 // 2 MiB

var codeExt = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".java": true, ".kt": true,
	".rs": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
	".swift": true, ".dart": true, ".scala": true, ".sh": true, ".sql": true, ".proto": true, ".yaml": true,
	".yml": true, ".toml": true, ".tf": true, ".html": true, ".css": true, ".scss": true, ".vue": true,
	".xml": true, ".gradle": true, ".mod": true, ".ini": true, ".cfg": true,
}

var textExt = map[string]string{
	".txt": "text", ".log": "text", ".md": "markdown", ".markdown": "markdown", ".rst": "text",
}

// KindOf reports the extractor kind for a file name, or "" if unsupported.
func KindOf(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case textExt[ext] != "":
		return textExt[ext]
	case ext == ".csv" || ext == ".tsv":
		return "csv"
	case ext == ".json":
		return "json"
	case ext == ".docx" || ext == ".xlsx" || ext == ".pptx":
		return ext[1:]
	case codeExt[ext]:
		return "code"
	case ext == "" && isWellKnown(filepath.Base(name)):
		return "code"
	default:
		return ""
	}
}

func isWellKnown(base string) bool {
	switch base {
	case "Makefile", "Dockerfile", "Taskfile", "LICENSE", "README", "Jenkinsfile":
		return true
	}
	return false
}

// Path extracts a file, or walks a folder as a code base.
func Path(path string) ([]Document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("context %s: %w", path, err)
	}
	if info.IsDir() {
		return Tree(path, TreeOptions{})
	}
	d, err := File(path)
	if err != nil {
		return nil, err
	}
	return []Document{d}, nil
}

// File extracts one file by its extension.
func File(path string) (Document, error) {
	kind := KindOf(path)
	if kind == "" {
		return Document{}, fmt.Errorf("%s: %w (%s)", path, ErrUnsupported, filepath.Ext(path))
	}
	limit := int64(MaxFileBytes)
	if isOffice(kind) {
		limit = MaxArchiveBytes
	}
	b, err := readLimited(path, limit)
	if err != nil {
		return Document{}, err
	}
	if !isOffice(kind) && isBinary(b) {
		return Document{}, fmt.Errorf("%s: %w (binary content)", path, ErrUnsupported)
	}
	return FromBytes(path, kind, b)
}

// FromBytes extracts already-loaded content, e.g. piped stdin.
func FromBytes(source, kind string, b []byte) (Document, error) {
	switch kind {
	case "csv":
		return CSV(source, b)
	case "json":
		return JSON(source, b)
	case KindDOCX:
		return DOCX(source, b)
	case KindXLSX:
		return XLSX(source, b)
	case KindPPTX:
		return PPTX(source, b)
	default:
		return Document{Source: source, Kind: kind, Text: string(b)}, nil
	}
}

func isOffice(kind string) bool { return kind == KindDOCX || kind == KindXLSX || kind == KindPPTX }

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the user chose this context file
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, limit)
	}
	buf := make([]byte, min(info.Size(), limit)+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(n) > limit {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, limit)
	}
	return buf[:n], nil
}

func isBinary(b []byte) bool {
	head := b
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}
