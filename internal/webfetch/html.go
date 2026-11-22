package webfetch

import (
	"html"
	"strings"
)

// blockTags end a line in the text.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "ul": true, "ol": true, "table": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "title": true, "section": true,
	"article": true, "header": true, "footer": true, "blockquote": true, "pre": true, "hr": true,
}

// HTMLText reduces an HTML page to its text: scripts, styles and comments
// are dropped, block elements end lines, entities are decoded and blank
// lines collapse. It never parses into a tree, so hostile markup costs one pass.
func HTMLText(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '<' {
			j := strings.IndexByte(s[i:], '<')
			if j < 0 {
				j = len(s) - i
			}
			sb.WriteString(s[i : i+j])
			i += j
			continue
		}
		next, block, ok := skipMarkup(s, i)
		if !ok {
			if next < 0 { // an unclosed tag: keep the rest as text
				sb.WriteString(s[i:])
			}
			break
		}
		if block {
			sb.WriteByte('\n')
		}
		i = next
	}
	return tidy(html.UnescapeString(sb.String()))
}

// skipMarkup steps over the comment, tag or script/style element at s[i]
// ('<'). It returns where text resumes and whether the tag ends a line; ok
// is false when the rest of s is unterminated markup (next < 0: an unclosed
// tag, kept as text; otherwise dropped).
func skipMarkup(s string, i int) (next int, block, ok bool) {
	if strings.HasPrefix(s[i:], "<!--") {
		end := strings.Index(s[i+4:], "-->")
		if end < 0 {
			return 0, false, false
		}
		return i + 4 + end + 3, false, true
	}
	end := strings.IndexByte(s[i:], '>')
	if end < 0 {
		return -1, false, false
	}
	body := s[i+1 : i+end]
	name := tagName(body)
	next = i + end + 1
	if (name == "script" || name == "style") && !strings.HasPrefix(strings.TrimSpace(body), "/") {
		skip := strings.Index(strings.ToLower(s[next:]), "</"+name)
		if skip < 0 {
			return 0, false, false
		}
		return next + skip, false, true
	}
	return next, blockTags[name], true
}

// tagName is the lower-case element name of a tag body such as "/P class=x".
func tagName(body string) string {
	body = strings.TrimPrefix(strings.TrimSpace(body), "/")
	end := strings.IndexAny(body, " \t\r\n/")
	if end >= 0 {
		body = body[:end]
	}
	return strings.ToLower(body)
}

// tidy collapses runs of spaces in each line and keeps at most one blank line.
func tidy(s string) string {
	var out []string
	blank := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
