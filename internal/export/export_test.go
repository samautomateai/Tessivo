package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

const report = "# Risk report\n\nScore **7 of 10**, high.\n\n## Mitigations\n\n" +
	"- Add `idempotency` keys\n- Retry <safely> & log\n\n1. First\n2. Second\n\n" +
	"| a | b |\n|---|---|\n| 1 | 2 |\n"

func documentXML(t *testing.T, b []byte) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	var doc string
	for _, f := range r.File {
		names[f.Name] = true
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			raw, _ := io.ReadAll(rc)
			_ = rc.Close()
			doc = string(raw)
		}
	}
	for _, want := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		if !names[want] {
			t.Fatalf("missing %s", want)
		}
	}
	return doc
}

func wellFormed(t *testing.T, doc string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("document.xml is not well-formed: %v", err)
		}
	}
}

func TestDOCX(t *testing.T) {
	b, err := DOCX("Risk report", report)
	if err != nil {
		t.Fatal(err)
	}
	doc := documentXML(t, b)
	wellFormed(t, doc)
	for _, want := range []string{
		"Risk report", "7 of 10", "Mitigations", "idempotency", "Retry &lt;safely&gt; &amp; log",
		"First", "<w:b/>", "Consolas", "a | b",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml lacks %q", want)
		}
	}
	if strings.Contains(doc, "**") || strings.Contains(doc, "|---|") {
		t.Errorf("markdown syntax left in: %s", doc)
	}
	if _, err := DOCX("x", strings.Repeat("a", MaxMarkdown+1)); err == nil {
		t.Fatal("oversized report accepted")
	}
}

func TestFileName(t *testing.T) {
	if got := FileName(`Q3: risk / plan?`, "docx"); got != "Q3 risk plan.docx" {
		t.Fatalf("got %q", got)
	}
	if got := FileName("", "md"); got != "output.md" {
		t.Fatalf("got %q", got)
	}
}

func FuzzDOCX(f *testing.F) {
	for _, s := range []string{report, "**", "`", "# ", "- \x00", "<w:p>", "1. **a `b` c**"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		b, err := DOCX("t", md)
		if err != nil {
			return
		}
		wellFormed(t, documentXML(t, b))
	})
}
