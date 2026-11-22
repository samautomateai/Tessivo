// Package export turns a saved output's Markdown into files people share:
// Markdown as is, and Word (.docx) built with the standard library (zip and
// WordprocessingML). Headings, lists, bold and inline code are kept; tables
// become plain " | "-separated lines.
package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// MaxMarkdown bounds the Markdown converted in one call.
const MaxMarkdown = 4 << 20

var (
	numbered   = regexp.MustCompile(`^\d{1,4}[.)] `)
	tableRule  = regexp.MustCompile(`^\|?[\s:|-]+\|?$`)
	unsafeName = regexp.MustCompile(`[^\p{L}\p{N} ._-]+`)
)

// FileName is a safe file name for title with the extension ext.
func FileName(title, ext string) string {
	name := strings.Join(strings.Fields(unsafeName.ReplaceAllString(title, " ")), " ")
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	if name == "" {
		name = "output"
	}
	return name + "." + ext
}

// DOCX converts Markdown to a Word document.
func DOCX(title, md string) ([]byte, error) {
	if len(md) > MaxMarkdown {
		return nil, fmt.Errorf("output is larger than %d bytes", MaxMarkdown)
	}
	var body strings.Builder
	if !strings.HasPrefix(strings.TrimSpace(md), "# ") && title != "" {
		paragraph(&body, 1, "", title)
	}
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t")
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case skipLine(trimmed):
			continue
		case strings.HasPrefix(trimmed, "#"):
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			paragraph(&body, min(level, 3), "", strings.TrimSpace(trimmed[level:]))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			paragraph(&body, 0, "• ", trimmed[2:])
		case numbered.MatchString(trimmed):
			paragraph(&body, 0, "", trimmed)
		case strings.HasPrefix(trimmed, "|"):
			cells := strings.Split(strings.Trim(trimmed, "|"), "|")
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			paragraph(&body, 0, "", strings.Join(cells, " | "))
		default:
			paragraph(&body, 0, "", trimmed)
		}
	}
	return pack(body.String())
}

var headingSize = [...]string{"", "36", "30", "26"}

// paragraph writes one w:p; level 1–3 is a heading, prefix starts a list item.
func paragraph(sb *strings.Builder, level int, prefix, text string) {
	sb.WriteString("<w:p>")
	if prefix != "" {
		sb.WriteString(`<w:pPr><w:ind w:left="360" w:hanging="240"/></w:pPr>`)
		run(sb, prefix, false, false, "")
	}
	size := ""
	if level > 0 {
		size = headingSize[level]
	}
	bold, code := level > 0, false
	for _, part := range splitInline(text) {
		switch part {
		case "**":
			bold = !bold || level > 0
		case "`":
			code = !code
		default:
			run(sb, part, bold, code, size)
		}
	}
	sb.WriteString("</w:p>")
}

// splitInline splits text into pieces and the ** and ` markers between them.
func splitInline(text string) []string {
	var out []string
	for text != "" {
		i := strings.IndexAny(text, "*`")
		switch {
		case i < 0:
			return append(out, text)
		case text[i] == '`':
			out = append(out, text[:i], "`")
			text = text[i+1:]
		case strings.HasPrefix(text[i:], "**"):
			out = append(out, text[:i], "**")
			text = text[i+2:]
		default:
			out = append(out, text[:i+1])
			text = text[i+1:]
		}
	}
	return out
}

func run(sb *strings.Builder, text string, bold, code bool, size string) {
	if text == "" {
		return
	}
	sb.WriteString("<w:r>")
	if bold || code || size != "" {
		sb.WriteString("<w:rPr>")
		if code {
			sb.WriteString(`<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/>`)
		}
		if bold {
			sb.WriteString("<w:b/>")
		}
		if size != "" {
			sb.WriteString(`<w:sz w:val="` + size + `"/>`)
		}
		sb.WriteString("</w:rPr>")
	}
	sb.WriteString(`<w:t xml:space="preserve">`)
	_ = xml.EscapeText(sb, []byte(text)) // writes to a strings.Builder: never fails; invalid characters become U+FFFD
	sb.WriteString("</w:t></w:r>")
}

const (
	contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ` +
		`ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
	rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" ` +
		`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" ` +
		`Target="word/document.xml"/></Relationships>`
	docStart = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`
	docEnd = `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr></w:body></w:document>`
)

func pack(body string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, part := range []struct{ name, data string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"word/document.xml", docStart + body + docEnd},
	} {
		w, err := zw.Create(part.name)
		if err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
		if _, err := w.Write([]byte(part.data)); err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("docx: %w", err)
	}
	return buf.Bytes(), nil
}

// skipLine is true for blank lines, code fences and table rules (|---|).
func skipLine(s string) bool {
	return s == "" || strings.HasPrefix(s, "```") || (tableRule.MatchString(s) && strings.Contains(s, "-"))
}
