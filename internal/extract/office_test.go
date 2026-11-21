package extract

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func makeZip(t testing.TB, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range parts {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const docxBody = `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Contract renewal</w:t></w:r></w:p>
<w:p><w:r><w:t>Term:</w:t><w:tab/><w:t>12 months</w:t><w:br/><w:t>auto-renews</w:t></w:r></w:p>
<w:p></w:p><w:p></w:p><w:p></w:p>
<w:tbl><w:tr><w:tc><w:p><w:r><w:t>Vendor</w:t></w:r></w:p></w:tc>
<w:tc><w:p><w:r><w:t>Price</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>Acme</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>10k</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
</w:body></w:document>`

func TestDOCX(t *testing.T) {
	d, err := DOCX("c.docx", makeZip(t, map[string]string{"word/document.xml": docxBody}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Contract renewal", "Term:\t12 months\nauto-renews", "Vendor", "Acme", "10k"} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("missing %q in:\n%s", want, d.Text)
		}
	}
	if strings.Contains(d.Text, "\n\n\n") {
		t.Fatal("blank lines not collapsed")
	}
	if _, err := DOCX("x.docx", makeZip(t, map[string]string{"other.xml": "<a/>"})); err == nil {
		t.Fatal("want missing part error")
	}
	if _, err := DOCX("x.docx", makeZip(t, map[string]string{"word/document.xml": "<w:p><unclosed>"})); err == nil {
		t.Fatal("want xml error")
	}
	if _, err := DOCX("x.docx", []byte("not a zip")); err == nil {
		t.Fatal("want zip error")
	}
}

const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

func xlsxParts() map[string]string {
	return map[string]string{
		"xl/workbook.xml": `<workbook ` + ns + `><sheets><sheet name="Sales" sheetId="1" r:id="rId1"/>` +
			`<sheet name="Empty" sheetId="2" r:id="rId2"/>` +
			`<sheet name="Ghost" sheetId="3" r:id="rId9"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships>` +
			`<Relationship Id="rId1" Type="t" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="t" Target="/xl/worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml": `<sst ` + ns + `><si><t>region</t></si><si><t>amount</t></si>` +
			`<si><r><t>No</t></r><r><t>rth</t></r></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet ` + ns + `><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c>` +
			`<c r="C1" t="inlineStr"><is><t>ok</t></is></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>10.5</v></c><c r="C2" t="b"><v>1</v></c></row>` +
			`<row r="3"><c r="A3" t="s"><v>99</v></c><c r="C3"><v>4</v></c><c t="b"><v>0</v></c></row>` +
			`</sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet ` + ns + `><sheetData/></worksheet>`,
	}
}

func TestXLSX(t *testing.T) {
	d, err := XLSX("book.xlsx", makeZip(t, xlsxParts()))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Table book.xlsx › Sales: 2 rows", "amount: numeric, min 10.5", "North | 10.5 | TRUE",
		" |  | 4 | FALSE", "(empty table)",
	} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("missing %q in:\n%s", want, d.Text)
		}
	}
	if strings.Contains(d.Text, "Ghost") {
		t.Fatal("sheets without parts must be skipped")
	}
	noShared := xlsxParts()
	delete(noShared, "xl/sharedStrings.xml")
	if _, err := XLSX("b.xlsx", makeZip(t, noShared)); err != nil {
		t.Fatal(err)
	}
	broken := xlsxParts()
	broken["xl/worksheets/sheet1.xml"] = `<worksheet><sheetData><row><c r="A1"><v>1</c></row>`
	if _, err := XLSX("b.xlsx", makeZip(t, broken)); err == nil {
		t.Fatal("want sheet parse error")
	}
	if _, err := XLSX("b.xlsx", makeZip(t, map[string]string{"x": "y"})); err == nil {
		t.Fatal("want missing workbook error")
	}
	if columnIndex("ZZZZ1", 0) != MaxSheetCols || columnIndex("", 3) != 3 || columnIndex("AA7", 0) != 26 {
		t.Fatal("columnIndex")
	}
}

func TestPPTX(t *testing.T) {
	a := `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	parts := map[string]string{
		"ppt/slides/slide2.xml":  `<p:sld ` + a + `><a:p><a:r><a:t>Roadmap</a:t></a:r></a:p></p:sld>`,
		"ppt/slides/slide10.xml": `<p:sld ` + a + `><a:p><a:r><a:t>Thanks</a:t></a:r></a:p></p:sld>`,
		"ppt/slides/slide1.xml": `<p:sld ` + a + `><a:p><a:r><a:t>Q4</a:t></a:r><a:br/>` +
			`<a:r><a:t>review</a:t></a:r></a:p></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships><Relationship Id="r1" ` +
			`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" ` +
			`Target="../notesSlides/notesSlide7.xml"/></Relationships>`,
		"ppt/notesSlides/notesSlide7.xml": `<p:notes ` + a +
			`><a:p><a:r><a:t>Mention   churn</a:t></a:r></a:p></p:notes>`,
	}
	d, err := PPTX("deck.pptx", makeZip(t, parts))
	if err != nil {
		t.Fatal(err)
	}
	i1, i2, i3 := strings.Index(d.Text, "Q4"), strings.Index(d.Text, "Roadmap"), strings.Index(d.Text, "Thanks")
	if i1 < 0 || i1 > i2 || i2 > i3 {
		t.Fatalf("slides out of order:\n%s", d.Text)
	}
	if !strings.Contains(d.Text, "Q4\nreview") || !strings.Contains(d.Text, "Notes: Mention churn") {
		t.Fatalf("text:\n%s", d.Text)
	}
	parts["ppt/slides/slide3.xml"] = "<bad"
	if _, err := PPTX("deck.pptx", makeZip(t, parts)); err == nil {
		t.Fatal("want parse error")
	}
	if _, err := PPTX("deck.pptx", []byte("zip?")); err == nil {
		t.Fatal("want zip error")
	}
}

func TestZipBombIsRejected(t *testing.T) {
	bomb := strings.Repeat("<w:p><w:r><w:t>a</w:t></w:r></w:p>", 400_000) // ~14 MB, compresses ~1000x
	_, err := DOCX("bomb.docx", makeZip(t, map[string]string{"word/document.xml": bomb}))
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("want ErrArchiveLimit, got %v", err)
	}
}

func TestTooManyEntries(t *testing.T) {
	old := maxEntries
	maxEntries = 5
	t.Cleanup(func() { maxEntries = old })
	parts := map[string]string{}
	for i := 0; i <= maxEntries; i++ {
		parts["p/"+strconv.Itoa(i)] = ""
	}
	if _, err := openZip(makeZip(t, parts)); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("want entry limit, got %v", err)
	}
}

func TestOfficeViaFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.docx")
	if err := os.WriteFile(p, makeZip(t, map[string]string{"word/document.xml": docxBody}), 0o600); err != nil {
		t.Fatal(err)
	}
	docs, err := Path(p)
	if err != nil || docs[0].Kind != "docx" || !strings.Contains(docs[0].Text, "Acme") {
		t.Fatalf("%v %+v", err, docs)
	}
}

func FuzzOffice(f *testing.F) {
	f.Add(makeZip(f, map[string]string{"word/document.xml": docxBody}))
	f.Add(makeZip(f, xlsxParts()))
	f.Add([]byte("PK\x03\x04garbage"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = DOCX("f.docx", b)
		_, _ = XLSX("f.xlsx", b)
		_, _ = PPTX("f.pptx", b)
	})
}
