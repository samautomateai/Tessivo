package service

import (
	"context"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/research"
)

// A source whose content changes on every read (live MCP data, a folder that
// holds the notebook itself) must not be summarized in a loop, and opening a
// notebook after every event must not extract every source again.
const (
	recheckAfter   = 30 * time.Second // a source is read for its fingerprint at most this often
	resummaryAfter = 5 * time.Minute  // a changed source is summarized again at most this often
)

type sourceCheck struct {
	at          time.Time
	fingerprint string
	err         error
}

// checkTable remembers recent fingerprint checks and summaries in this process.
type checkTable struct {
	mu         sync.Mutex
	checks     map[string]sourceCheck
	summarized map[string]time.Time
}

func (s *Service) checkTable() *checkTable {
	s.checkOnce.Do(func() {
		s.checks = &checkTable{checks: map[string]sourceCheck{}, summarized: map[string]time.Time{}}
	})
	return s.checks
}

func checkKey(notebookID, sourceID string) string { return notebookID + "/" + sourceID }

// checkSource returns a source's fingerprint, reading it at most once per recheckAfter.
func (s *Service) checkSource(ctx context.Context, nb *research.Notebook, src research.Source) (string, error) {
	t, key := s.checkTable(), checkKey(nb.Manifest.ID, src.ID)
	t.mu.Lock()
	c, ok := t.checks[key]
	t.mu.Unlock()
	if ok && s.now().Sub(c.at) < recheckAfter {
		return c.fingerprint, c.err
	}
	read, err := s.readResearchSource(ctx, nb, src)
	t.mu.Lock()
	t.checks[key] = sourceCheck{at: s.now(), fingerprint: read.fingerprint, err: err}
	t.mu.Unlock()
	return read.fingerprint, err
}

// markSummarized starts a source's re-summary cooldown.
func (s *Service) markSummarized(notebookID, sourceID string) {
	t := s.checkTable()
	t.mu.Lock()
	t.summarized[checkKey(notebookID, sourceID)] = s.now()
	t.mu.Unlock()
}

func (s *Service) mayResummarize(notebookID, sourceID string) bool {
	t := s.checkTable()
	t.mu.Lock()
	last, ok := t.summarized[checkKey(notebookID, sourceID)]
	t.mu.Unlock()
	return !ok || s.now().Sub(last) >= resummaryAfter
}

// recheck compares a source that has a ready summary with its content now:
// unreadable, updating (a changed source, queued outside its cooldown) or ready.
func (s *Service) recheck(ctx context.Context, nb *research.Notebook, src research.Source, sum research.Summary,
) (state, msg string) {
	fp, err := s.checkSource(ctx, nb, src)
	switch {
	case err != nil:
		return SummaryUnreadable, s.redactor()(err.Error())
	case fp != sum.Fingerprint && s.mayResummarize(nb.Manifest.ID, src.ID):
		s.enqueueSummary(nb.Manifest.ID, src.ID)
		return SummaryUpdating, ""
	}
	return SummaryReady, ""
}
