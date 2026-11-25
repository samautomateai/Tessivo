package service

import (
	"maps"
	"slices"

	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// The {{previous}} prompt variable (MEM-3).
const (
	previousVar      = "previous"
	maxPreviousChars = 32000
	noPrevious       = "(there is no earlier output yet)"
)

// withPrevious fills {{previous}} with the notebook's latest output when the
// prompt uses it and the run did not set it. vars is copied, never changed.
func withPrevious(nb *notebook.Notebook, prompt string, vars map[string]string) map[string]string {
	if nb == nil || !slices.Contains(executor.Variables(prompt), previousVar) {
		return vars
	}
	if _, set := vars[previousVar]; set {
		return vars
	}
	out := maps.Clone(vars)
	if out == nil {
		out = map[string]string{}
	}
	out[previousVar] = latestOutput(nb)
	return out
}

// latestOutput is the newest output's Markdown, cut at maxPreviousChars.
func latestOutput(nb *notebook.Notebook) string {
	outs, err := nb.ListOutputs()
	if err != nil || len(outs) == 0 {
		return noPrevious
	}
	newest := slices.MaxFunc(outs, func(a, b notebook.Output) int {
		switch {
		case a.CreatedAt < b.CreatedAt:
			return -1
		case a.CreatedAt > b.CreatedAt:
			return 1
		}
		return 0
	})
	_, md, err := nb.GetOutput(newest.ID)
	if err != nil || md == "" {
		return noPrevious
	}
	if len(md) > maxPreviousChars {
		md = md[:maxPreviousChars] + "\n\n[… the rest of the previous output was cut]"
	}
	return md
}
