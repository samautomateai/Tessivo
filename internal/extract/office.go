package extract

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DOCX extracts paragraphs and tables (cells joined with " | ") from Word.
func DOCX(source string, b []byte) (Document, error) {
	z, err := openZip(b)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	data, err := z.read("word/document.xml")
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	text, err := xmlText(data, wordRules)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	return Document{Source: source, Kind: KindDOCX, Text: text}, nil
}

// PPTX extracts slide text and speaker notes, in slide order.
func PPTX(source string, b []byte) (Document, error) {
	z, err := openZip(b)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	var sb strings.Builder
	for i, slide := range numberedParts(z, slidePattern) {
		data, err := z.read(slide)
		if err != nil {
			return Document{}, fmt.Errorf("%s: %w", source, err)
		}
		text, err := xmlText(data, drawingRules)
		if err != nil {
			return Document{}, fmt.Errorf("%s: %w", source, err)
		}
		fmt.Fprintf(&sb, "## Slide %d\n%s\n", i+1, strings.TrimSpace(text))
		if notes := slideNotes(z, slide); notes != "" {
			fmt.Fprintf(&sb, "Notes: %s\n", notes)
		}
		sb.WriteString("\n")
	}
	return Document{Source: source, Kind: KindPPTX, Text: sb.String()}, nil
}

var slidePattern = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// numberedParts returns parts matching re, sorted by their captured number.
func numberedParts(z *safeZip, re *regexp.Regexp) []string {
	type part struct {
		n    int
		name string
	}
	var parts []part
	for name := range z.files {
		if m := re.FindStringSubmatch(name); m != nil {
			n, _ := strconv.Atoi(m[1])
			parts = append(parts, part{n, name})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].n < parts[j].n })
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.name
	}
	return out
}

// slideNotes follows the slide's relationships to its notes part, if any.
func slideNotes(z *safeZip, slide string) string {
	rels := path.Join(path.Dir(slide), "_rels", path.Base(slide)+".rels")
	for _, rel := range relationships(z, rels) {
		if !strings.HasSuffix(rel.Type, "/notesSlide") {
			continue
		}
		target := path.Clean(path.Join(path.Dir(slide), rel.Target))
		data, err := z.read(target)
		if err != nil {
			return ""
		}
		text, err := xmlText(data, drawingRules)
		if err != nil {
			return ""
		}
		return strings.Join(strings.Fields(text), " ")
	}
	return ""
}

type relationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

func relationships(z *safeZip, name string) []relationship {
	if !z.has(name) {
		return nil
	}
	data, err := z.read(name)
	if err != nil {
		return nil
	}
	var doc struct {
		Rels []relationship `xml:"Relationship"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return doc.Rels
}

// textRules tells xmlText which elements carry text and which end a line.
type textRules struct {
	text      string            // local name of text runs (w:t, a:t)
	breaks    map[string]string // end-element local name -> separator
	tabs      map[string]bool   // empty elements that mean a tab
	lineBreak map[string]bool   // empty elements that mean a newline
}

var wordRules = textRules{
	text:      "t",
	breaks:    map[string]string{"p": "\n", "tc": " | ", "tr": "\n"},
	tabs:      map[string]bool{"tab": true},
	lineBreak: map[string]bool{"br": true, "cr": true},
}

var drawingRules = textRules{
	text:      "t",
	breaks:    map[string]string{"p": "\n"},
	lineBreak: map[string]bool{"br": true},
}

// MaxXMLTokens bounds work on a single XML part.
const MaxXMLTokens = 5_000_000

// xmlText streams an XML part and collects text per the rules. Go's decoder
// does not expand external entities, so XXE and billion-laughs do not apply.
func xmlText(data []byte, r textRules) (string, error) {
	c := collector{rules: r}
	if err := walkXML(data, c.token); err != nil {
		return "", err
	}
	return tidy(c.sb.String()), nil
}

// walkXML streams tokens to fn with a token budget.
func walkXML(data []byte, fn func(xml.Token)) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for tokens := 0; tokens <= MaxXMLTokens; tokens++ {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("parse xml: %w", err)
		}
		fn(tok)
	}
	return fmt.Errorf("%w: too many XML tokens", ErrArchiveLimit)
}

type collector struct {
	rules  textRules
	sb     strings.Builder
	inText bool
}

func (c *collector) token(tok xml.Token) {
	switch t := tok.(type) {
	case xml.StartElement:
		c.inText = t.Name.Local == c.rules.text
		if c.rules.tabs[t.Name.Local] {
			c.sb.WriteByte('\t')
		}
		if c.rules.lineBreak[t.Name.Local] {
			c.sb.WriteByte('\n')
		}
	case xml.EndElement:
		c.inText = false
		if sep, ok := c.rules.breaks[t.Name.Local]; ok {
			c.sb.WriteString(sep)
		}
	case xml.CharData:
		if c.inText {
			c.sb.Write(t)
		}
	}
}

// tidy collapses runs of blank lines.
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(l, " "), " |"), " ")
		if l == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
