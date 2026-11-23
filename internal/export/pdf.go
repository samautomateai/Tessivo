package export

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
)

// A4 page in points, with 2 cm margins.
const (
	pageW, pageH = 595.0, 842.0
	margin       = 56.0
	textWidth    = pageW - 2*margin
)

// style is how a paragraph is set: font size, space before, indent and prefix.
type style struct {
	size, before, indent float64
	heading              bool
	prefix               string
}

var (
	styleBody   = style{size: 11, before: 5}
	styleBullet = style{size: 11, before: 2, indent: 14, prefix: "• "}
	styleH      = [...]style{
		{},
		{size: 20, before: 14, heading: true},
		{size: 15, before: 12, heading: true},
		{size: 13, before: 10, heading: true},
	}
)

// word is one word with its font: 1 Helvetica, 2 Helvetica-Bold, 3 Courier.
type word struct {
	text string
	font int
}

// PDF converts Markdown to a PDF using the built-in Helvetica and Courier
// fonts. Text outside Windows-1252 (e.g. CJK) prints as "?".
func PDF(title, md string) ([]byte, error) {
	if len(md) > MaxMarkdown {
		return nil, fmt.Errorf("output is larger than %d bytes", MaxMarkdown)
	}
	l := &layout{y: pageH - margin}
	if !strings.HasPrefix(strings.TrimSpace(md), "# ") && title != "" {
		l.paragraph(styleH[1], title)
	}
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case skipLine(trimmed):
		case strings.HasPrefix(trimmed, "#"):
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			l.paragraph(styleH[min(level, 3)], strings.TrimSpace(trimmed[level:]))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			l.paragraph(styleBullet, trimmed[2:])
		case strings.HasPrefix(trimmed, "|"):
			cells := strings.Split(strings.Trim(trimmed, "|"), "|")
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			l.paragraph(styleBody, strings.Join(cells, " | "))
		default:
			l.paragraph(styleBody, trimmed)
		}
	}
	return l.finish(title)
}

// layout fills pages top to bottom with wrapped lines.
type layout struct {
	pages []*bytes.Buffer
	page  *bytes.Buffer
	y     float64
}

func (l *layout) paragraph(st style, text string) {
	words := words(text, st.heading)
	if st.prefix != "" {
		words = append([]word{{st.prefix, 1}}, words...)
	}
	l.y -= st.before
	lineH := st.size * 1.3
	x0 := margin + st.indent
	words = splitLong(words, st.size, textWidth-st.indent)
	for len(words) > 0 {
		n := fitWords(words, st.size, textWidth-st.indent)
		l.line(x0, lineH, st.size, words[:n])
		words = words[n:]
	}
}

// line places one line, starting a new page when it would not fit.
func (l *layout) line(x, lineH, size float64, ws []word) {
	if l.page == nil || l.y-lineH < margin {
		l.page = &bytes.Buffer{}
		l.pages = append(l.pages, l.page)
		l.y = pageH - margin
	}
	l.y -= lineH
	fmt.Fprintf(l.page, "BT %.2f %.2f Td ", x, l.y)
	font := 0
	for i, w := range ws {
		if w.font != font {
			fmt.Fprintf(l.page, "/F%d %.1f Tf ", w.font, size)
			font = w.font
		}
		text := w.text
		if i < len(ws)-1 {
			text += " "
		}
		l.page.WriteString(pdfString(text) + " Tj ")
	}
	l.page.WriteString("ET\n")
}

// words splits inline Markdown into words carrying their font.
func words(text string, bold bool) []word {
	var out []word
	b, code := bold, false
	for _, part := range splitInline(text) {
		switch part {
		case "**":
			b = !b || bold
		case "`":
			code = !code
		default:
			font := 1
			switch {
			case code:
				font = 3
			case b:
				font = 2
			}
			for _, f := range strings.Fields(part) {
				out = append(out, word{f, font})
			}
		}
	}
	return out
}

// fitWords is how many words fit on one line (at least one; splitLong has
// already cut words wider than the line).
func fitWords(ws []word, size, width float64) int {
	used := 0.0
	for i, w := range ws {
		wd := textWidthOf(w.text, w.font, size)
		if i > 0 {
			wd += textWidthOf(" ", w.font, size)
		}
		if used+wd > width && i > 0 {
			return i
		}
		used += wd
	}
	return len(ws)
}

// splitLong cuts words wider than a line into pieces that fit.
func splitLong(ws []word, size, width float64) []word {
	out := make([]word, 0, len(ws))
	for _, w := range ws {
		r := []rune(w.text)
		for len(r) > 0 {
			n := len(r)
			for n > 1 && textWidthOf(string(r[:n]), w.font, size) > width {
				n--
			}
			out = append(out, word{string(r[:n]), w.font})
			r = r[n:]
		}
	}
	return out
}

func (l *layout) finish(title string) ([]byte, error) {
	if len(l.pages) == 0 {
		l.line(margin, 14, 11, []word{{" ", 1}})
	}
	objs := []string{
		"", // catalog, filled below
		"", // pages
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>",
		"<< /Title " + pdfString(title) + " /Producer (Tessivo) >>",
	}
	kids := make([]string, 0, len(l.pages))
	for _, p := range l.pages {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		if _, err := zw.Write(p.Bytes()); err != nil {
			return nil, fmt.Errorf("pdf: %w", err)
		}
		if err := zw.Close(); err != nil {
			return nil, fmt.Errorf("pdf: %w", err)
		}
		pageObj := len(objs) + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObj))
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>", pageW, pageH,
				pageObj+1),
			fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", z.Len(), z.String()))
	}
	objs[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids))
	return writeObjects(objs), nil
}

func writeObjects(objs []string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Info 6 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}
