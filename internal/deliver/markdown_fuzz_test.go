package deliver

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzHTML checks that no input makes live markup or breaks out of CDATA.
func FuzzHTML(f *testing.F) {
	for _, s := range []string{sample, "**", "`a", "# <img src=x onerror=1>", "```\n]]>\n", "- *x*\n1) y"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		out := HTML(md)
		if strings.Contains(out, "<script") || strings.Contains(out, "]]>") || strings.Contains(out, "<img") {
			t.Fatalf("live markup from %q: %s", md, out)
		}
		for _, b := range NotionBlocks(md) {
			if body, ok := b[b[keyType].(string)].(map[string]any); ok {
				if rt, ok := body["rich_text"].([]map[string]any); ok {
					for _, r := range rt {
						s := r["text"].(map[string]any)["content"].(string)
						if utf8.RuneCountInString(s) > maxRichText {
							t.Fatalf("rich text over %d runes", maxRichText)
						}
					}
				}
			}
		}
	})
}
