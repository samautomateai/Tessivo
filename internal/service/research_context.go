package service

import (
	"context"

	"github.com/samautomateai/tessivo/internal/research"
)

// researchItems numbers what a question would send: sources at full (their
// text) or summary (their summary), then included notes. A source that
// cannot be read is skipped and named in skipped; a summarized source is
// checked against its content and queued again when it changed.
//
// With estimate set (context totals) nothing is read that has a cached
// size: full sources count their cached tokens and summaries are not
// checked, so changing a level never re-reads a source.
func (s *Service) researchItems(ctx context.Context, nb *research.Notebook, estimate bool,
) (items []research.Item, skipped []string) {
	for _, src := range nb.Manifest.Sources {
		if src.Level == research.LevelOff {
			continue
		}
		it, ok := s.sourceItem(ctx, nb, src, len(items)+1, estimate)
		if !ok {
			skipped = append(skipped, src.Label)
			continue
		}
		items = append(items, it)
	}
	notes, _ := nb.Notes()
	for _, n := range notes {
		if n.Include {
			items = append(items, research.Item{
				N: len(items) + 1, Kind: research.ItemNote, ID: n.ID, Label: n.Title, Text: n.Text,
			})
		}
	}
	return items, skipped
}

// sourceItem is source n as it would be sent; false when it cannot be read.
func (s *Service) sourceItem(ctx context.Context, nb *research.Notebook, src research.Source, n int, estimate bool,
) (research.Item, bool) {
	it := research.Item{N: n, Kind: research.ItemSource, ID: src.ID, Label: src.Label, Level: src.Level}
	if sum, ok, _ := nb.Summary(src.ID); src.Level == research.LevelSummary && ok &&
		sum.Status == research.StatusReady {
		if !estimate {
			if state, _ := s.recheck(ctx, nb, src, sum); state == SummaryUnreadable {
				return it, false
			}
		}
		it.Text = sum.Text
		return it, true
	}
	it.Level = research.LevelFull
	if estimate && src.TokensFor != "" && src.Tokens > 0 {
		it.Size = src.Tokens
		return it, true
	}
	read, err := s.readResearchSource(ctx, nb, src)
	if err != nil {
		return it, false
	}
	it.Text, it.Files = read.text, read.files
	return it, true
}

// ResearchContext reports what a question would send now: counts, the size
// before fitting and the sources that would be shortened.
func (s *Service) ResearchContext(ctx context.Context, id string) (ContextTotals, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return ContextTotals{}, err
	}
	items, _ := s.researchItems(ctx, nb, true)
	tokens, cut := research.Estimate(items, budget(s.researchSettings(nb)))
	tot := ContextTotals{Tokens: tokens, Cut: cut}
	if tot.Cut == nil {
		tot.Cut = []string{}
	}
	for _, it := range items {
		switch {
		case it.Kind == research.ItemNote:
			tot.Notes++
		case it.Level == research.LevelSummary:
			tot.Sources++
			tot.Summaries++
		default:
			tot.Sources++
		}
	}
	return tot, nil
}
