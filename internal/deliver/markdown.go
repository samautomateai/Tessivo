package deliver

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

// mdBlock is one block of the Markdown Tessivo renders: headings, paragraphs,
// list items, fenced code and rules. Anything else is kept as a paragraph.
type mdBlock struct {
	kind string // h, p, ul, ol, code, hr

	level int // heading level
	text  string
}

var (
	numberedItem = regexp.MustCompile(`^\d{1,4}[.)] `)
	tableRule    = regexp.MustCompile(`^\|?[\s:|-]+\|?$`)
	headingLine  = regexp.MustCompile(`^(#{1,6}) (.*)$`)
)

// parseMarkdown splits md into blocks.
func parseMarkdown(md string) []mdBlock {
	var out []mdBlock
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, mdBlock{kind: "p", text: strings.Join(para, " ")})
			para = nil
		}
	}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, mdBlock{kind: kindCode, text: strings.Join(code, "\n")})
		case trimmed == "":
			flush()
		case headingLine.MatchString(trimmed):
			flush()
			m := headingLine.FindStringSubmatch(trimmed)
			out = append(out, mdBlock{kind: "h", level: len(m[1]), text: m[2]})
		case trimmed == "---" || trimmed == "***":
			flush()
			out = append(out, mdBlock{kind: "hr"})
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			flush()
			out = append(out, mdBlock{kind: "ul", text: trimmed[2:]})
		case numberedItem.MatchString(trimmed):
			flush()
			out = append(out, mdBlock{kind: "ol", text: numberedItem.ReplaceAllString(trimmed, "")})
		case strings.HasPrefix(trimmed, "|"):
			flush()
			if !tableRule.MatchString(trimmed) {
				out = append(out, mdBlock{kind: "p", text: trimmed})
			}
		default:
			para = append(para, trimmed)
		}
	}
	flush()
	return out
}

// span is a run of inline text: plain, **bold** or `code`.
type span struct {
	text       string
	bold, code bool
}

// inline splits text into spans; markers without a closing pair stay as text.
func inline(text string) []span {
	var out []span
	for text != "" {
		i := strings.IndexAny(text, "*`")
		if i < 0 {
			return append(out, span{text: text})
		}
		marker, bold := "`", false
		if strings.HasPrefix(text[i:], "**") {
			marker, bold = "**", true
		} else if text[i] == '*' {
			out = append(out, span{text: text[:i+1]})
			text = text[i+1:]
			continue
		}
		end := strings.Index(text[i+len(marker):], marker)
		if end < 0 {
			return append(out, span{text: text})
		}
		if i > 0 {
			out = append(out, span{text: text[:i]})
		}
		out = append(out, span{text: text[i+len(marker) : i+len(marker)+end], bold: bold, code: !bold})
		text = text[i+len(marker)+end+len(marker):]
	}
	return out
}

// HTML converts Markdown to Confluence storage XHTML. Every piece of text is
// escaped, so markup in an output never becomes live.
func HTML(md string) string {
	var sb strings.Builder
	open := ""
	closeList := func() {
		if open != "" {
			sb.WriteString("</" + open + ">")
			open = ""
		}
	}
	for _, b := range parseMarkdown(md) {
		if b.kind != "ul" && b.kind != "ol" || open != b.kind {
			closeList()
		}
		switch b.kind {
		case "h":
			tag := "h" + strconv.Itoa(b.level)
			sb.WriteString("<" + tag + ">" + inlineHTML(b.text) + "</" + tag + ">")
		case "ul", "ol":
			if open == "" {
				sb.WriteString("<" + b.kind + ">")
				open = b.kind
			}
			sb.WriteString("<li>" + inlineHTML(b.text) + "</li>")
		case kindCode:
			sb.WriteString("<pre><code>" + html.EscapeString(b.text) + "</code></pre>")
		case "hr":
			sb.WriteString("<hr/>")
		default:
			sb.WriteString("<p>" + inlineHTML(b.text) + "</p>")
		}
	}
	closeList()
	return sb.String()
}

func inlineHTML(text string) string {
	var sb strings.Builder
	for _, s := range inline(text) {
		t := html.EscapeString(s.text)
		switch {
		case s.bold:
			sb.WriteString("<strong>" + t + "</strong>")
		case s.code:
			sb.WriteString("<code>" + t + "</code>")
		default:
			sb.WriteString(t)
		}
	}
	return sb.String()
}

// maxRichText is Notion's limit on one rich text item.
const maxRichText = 2000

// NotionBlocks converts Markdown to Notion blocks (headings 1 to 3, paragraphs,
// list items, code and dividers), cutting rich text at Notion's 2,000 characters.
func NotionBlocks(md string) []map[string]any {
	var out []map[string]any
	for _, b := range parseMarkdown(md) {
		var kind string
		var body map[string]any
		switch b.kind {
		case "h":
			kind = "heading_" + strconv.Itoa(min(b.level, 3))
		case "ul":
			kind = "bulleted_list_item"
		case "ol":
			kind = "numbered_list_item"
		case kindCode:
			code := richText([]span{{text: b.text}})
			kind, body = kindCode, map[string]any{"rich_text": code, "language": "plain text"}
		case "hr":
			kind, body = "divider", map[string]any{}
		default:
			kind = "paragraph"
		}
		if body == nil {
			body = map[string]any{"rich_text": richText(inline(b.text))}
		}
		out = append(out, map[string]any{"object": "block", keyType: kind, kind: body})
	}
	return out
}

func richText(spans []span) []map[string]any {
	out := []map[string]any{}
	for _, s := range spans {
		r := []rune(s.text)
		for len(r) > 0 {
			n := min(len(r), maxRichText)
			out = append(out, map[string]any{
				keyType: "text", "text": map[string]any{"content": string(r[:n])},
				"annotations": map[string]any{"bold": s.bold, kindCode: s.code},
			})
			r = r[n:]
		}
	}
	return out
}

// Repeated JSON keys and block kinds.
const (
	kindCode = "code"
	keyType  = "type"
	keyTitle = "title"
)
