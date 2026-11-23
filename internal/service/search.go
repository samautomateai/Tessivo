package service

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

// Search result kinds.
const (
	SearchNotebook = "notebook"
	SearchOutput   = "output"
	SearchPrompt   = "prompt"
	SearchResearch = "research"
	SearchNote     = "note"
)

// Search bounds.
const (
	maxSearchResults = 100
	maxSearchRead    = 1 << 20 // bytes read from one output
	snippetSide      = 70      // characters of context on each side of a hit
)

// SearchResult is one match: what it is, where it lives and a snippet.
type SearchResult struct {
	Kind       string `json:"kind"`
	NotebookID string `json:"notebookId"`
	ID         string `json:"id,omitempty"` // output, prompt or note
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
}

// Search finds notebooks, outputs, prompts, research notebooks and notes
// whose text holds every word of query (case-insensitive), at most limit.
func (s *Service) Search(query string, limit int) ([]SearchResult, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, invalidf("give words to search for")
	}
	if limit <= 0 || limit > maxSearchResults {
		limit = maxSearchResults
	}
	out := []SearchResult{}
	add := func(r SearchResult, text string) bool {
		lower := strings.ToLower(text)
		for _, w := range words {
			if !strings.Contains(lower, w) {
				return len(out) < limit
			}
		}
		r.Snippet = snippet(text, words[0])
		out = append(out, r)
		return len(out) < limit
	}
	nbs, err := s.ListNotebooks()
	if err != nil {
		return nil, err
	}
	for _, info := range nbs {
		if !searchNotebook(info, add) {
			return out, nil
		}
	}
	s.searchResearch(add)
	return out, nil
}

type matcher func(r SearchResult, text string) bool

func searchNotebook(info NotebookInfo, add matcher) bool {
	if !add(SearchResult{Kind: SearchNotebook, NotebookID: info.ID, Title: info.Name}, info.Name) {
		return false
	}
	nb := info.nb
	if outs, err := nb.ListOutputs(); err == nil {
		for i := len(outs) - 1; i >= 0; i-- { // newest first
			o := outs[i]
			text := o.Title + "\n" + readCapped(filepath.Join(nb.Dir, notebook.OutputsDir, o.ID+".md"))
			if !add(SearchResult{Kind: SearchOutput, NotebookID: info.ID, ID: o.ID, Title: o.Title}, text) {
				return false
			}
		}
	}
	names, _ := nb.ListPrompts()
	for _, n := range names {
		p, err := nb.LoadPrompt(n)
		if err == nil && !add(SearchResult{Kind: SearchPrompt, NotebookID: info.ID, ID: n, Title: n}, n+"\n"+p.Text) {
			return false
		}
	}
	return true
}

func (s *Service) searchResearch(add matcher) {
	root, err := s.researchRoot()
	if err != nil {
		return
	}
	rnbs, err := research.List(root)
	if err != nil {
		return
	}
	for _, r := range rnbs {
		m := r.Manifest
		if !add(SearchResult{Kind: SearchResearch, NotebookID: m.ID, Title: m.Name}, m.Name+"\n"+m.Description) {
			return
		}
		notes, _ := r.Notes()
		for _, n := range notes {
			if !add(SearchResult{Kind: SearchNote, NotebookID: m.ID, ID: n.ID, Title: n.Title}, n.Title+"\n"+n.Text) {
				return
			}
		}
	}
}

func readCapped(path string) string {
	f, err := os.Open(path) //nolint:gosec // G304: a notebook's own output file
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	b, _ := io.ReadAll(io.LimitReader(f, maxSearchRead))
	return string(b)
}

// snippet is the text around the first hit of word, on one line.
func snippet(text, word string) string {
	i := strings.Index(strings.ToLower(text), word)
	if i < 0 {
		i = 0
	}
	start, end := max(0, i-snippetSide), min(len(text), i+len(word)+snippetSide)
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	s := strings.Join(strings.Fields(text[start:end]), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < len(text) {
		s += "…"
	}
	return s
}
