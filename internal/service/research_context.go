package service

import (
	"context"

	"github.com/samautomateai/tessivo/internal/research"
)

// researchItems numbers what a question would send: sources at full (their
// text) or summary (their summary), then included notes. A source that
// cannot be read is skipped and named in skipped.
func (s *Service) researchItems(ctx context.Context, nb *research.Notebook) (items []research.Item, skipped []string) {
	for _, src := range nb.Manifest.Sources {
		if src.Level == research.LevelOff {
			continue
		}
		it := research.Item{
			N: len(items) + 1, Kind: research.ItemSource, ID: src.ID, Label: src.Label, Level: src.Level,
		}
		if sum, ok, _ := nb.Summary(src.ID); src.Level == research.LevelSummary && ok &&
			sum.Status == research.StatusReady {
			it.Text = sum.Text
			items = append(items, it)
			continue
		}
		read, err := s.readResearchSource(ctx, nb, src)
		if err != nil {
			skipped = append(skipped, src.Label)
			continue
		}
		it.Level, it.Text, it.Files = research.LevelFull, read.text, read.files
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

// ResearchContext reports what a question would send now: counts, the size
// before fitting and the sources that would be shortened.
func (s *Service) ResearchContext(ctx context.Context, id string) (ContextTotals, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return ContextTotals{}, err
	}
	items, _ := s.researchItems(ctx, nb)
	built := research.Build(nb.Manifest.Name, nb.Manifest.Description, items, nil, "", budget(s.researchSettings(nb)))
	tot := ContextTotals{Tokens: built.Tokens, Cut: built.Cut}
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
