package render

import (
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	data := `{"title":"PR 482 risk","risk_score":7,"tldr":"short","ok":true,"none":null,
	"impacted_areas":["ledger","batch"],"evidence":[{"claim":"double post","source":"writer.go"}],
	"mitigations":[],"meta":{"model":"m","ratio":0.25},"tags":[["a","b"]],"extra":"z"}`
	order := []string{"title", "tldr", "risk_score", "impacted_areas", "evidence", "mitigations"}
	md, err := Markdown([]byte(data), order)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# PR 482 risk\n", "## TL;DR\n\nshort", "## Risk score\n\n7\n", "- ledger\n- batch",
		"- **double post** — writer.go", "## Mitigations\n\n_None._", "- **Model**: m", "- **Ratio**: 0.25",
		"## Ok\n\nyes", "## None\n\n—", "- a, b", "## Extra",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Index(md, "## TL;DR") > strings.Index(md, "## Risk score") {
		t.Fatal("order not respected")
	}
	if _, err := Markdown([]byte(`[1]`), nil); err == nil {
		t.Fatal("want error for non-object")
	}
	if heading("") != "" || inline(false) != "no" || inline(int64(3)) != "3" {
		t.Fatal("helpers")
	}
}
