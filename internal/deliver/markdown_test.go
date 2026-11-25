package deliver

import (
	"encoding/json"
	"strings"
	"testing"
)

const sample = "# Title\n\nSome **bold** and `code` <script>alert(1)</script> & more.\n\n## Steps\n\n" +
	"- one\n- two\n\n1. first\n2. second\n\n```go\nx := \"]]>\"\n```\n\n---\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"

func TestHTMLEscapesEverything(t *testing.T) {
	got := HTML(sample)
	for _, want := range []string{
		"<h1>Title</h1>", "<strong>bold</strong>", "<code>code</code>",
		"&lt;script&gt;alert(1)&lt;/script&gt; &amp; more.",
		"<ul><li>one</li><li>two</li></ul>", "<ol><li>first</li><li>second</li></ol>",
		"<pre><code>x := &#34;]]&gt;&#34;</code></pre>", "<hr/>", "<p>| a | b |</p>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") || strings.Contains(got, "]]>") {
		t.Fatalf("live markup leaked:\n%s", got)
	}
}

func TestNotionBlocks(t *testing.T) {
	blocks := NotionBlocks(sample)
	kinds := []string{}
	for _, b := range blocks {
		kinds = append(kinds, b["type"].(string))
	}
	want := "heading_1 paragraph heading_2 bulleted_list_item bulleted_list_item numbered_list_item " +
		"numbered_list_item code divider paragraph paragraph"
	if strings.Join(kinds, " ") != want {
		t.Fatalf("kinds: %s", strings.Join(kinds, " "))
	}
	raw, _ := json.Marshal(blocks[1])
	if !strings.Contains(string(raw), `"bold":true`) || !strings.Contains(string(raw), `"code":true`) {
		t.Fatalf("annotations: %s", raw)
	}
	long := NotionBlocks(strings.Repeat("x", 4500))
	rich := long[0]["paragraph"].(map[string]any)["rich_text"].([]map[string]any)
	if len(rich) != 3 {
		t.Fatalf("rich text is cut at 2000 characters: %d parts", len(rich))
	}
}
