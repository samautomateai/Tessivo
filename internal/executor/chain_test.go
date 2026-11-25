package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
)

func TestPerSourceChainCondensesEachSourceFirst(t *testing.T) {
	spec := summarizeSpec(t)
	spec.Settings.Chain = config.ChainPerSource
	spec.Documents = []extract.Document{
		{Source: "a.md", Kind: "markdown", Text: "alpha"},
		{Source: "b.md", Kind: "markdown", Text: "beta"},
	}
	m := &scripted{replies: []string{"notes on a", "notes on b", goodSummary}}
	res, err := (&Pipeline{Model: m}).Run(context.Background(), spec, nil)
	if err != nil || res.Strategy != StrategyPerSource || !res.Valid {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(m.reqs) != 3 {
		t.Fatalf("want one call per source and one to write, got %d", len(m.reqs))
	}
	final := m.reqs[2].Messages[1].Content
	if !strings.Contains(final, "notes on a") || !strings.Contains(final, "notes on b") ||
		strings.Contains(final, "alpha") {
		t.Fatal("the output is written from the notes, not the sources")
	}
}

func TestChainSettingIsValidatedAndLayered(t *testing.T) {
	if err := (config.RunSettings{Chain: "loop"}).Validate(); err == nil {
		t.Fatal("unknown chain accepted")
	}
	e := config.Resolve(
		config.Layer{Name: "app", Settings: config.RunSettings{}},
		config.Layer{Name: "prompt", Settings: config.RunSettings{Chain: config.ChainPerSource}},
	)
	if e.Chain != config.ChainPerSource || e.Sources["chain"] != "prompt" {
		t.Fatalf("%+v", e)
	}
}
