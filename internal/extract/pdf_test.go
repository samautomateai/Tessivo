package extract

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tinyPDF builds a valid one-object-per-page PDF with the given page texts.
func tinyPDF(pages ...string) []byte {
	var objs []string
	kids := ""
	n := len(pages)
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 4+2*i)
	}
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, n),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, text := range pages {
		stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "+
				"/Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
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
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

func TestPDFText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.pdf")
	if err := os.WriteFile(path, tinyPDF("Ship in March", "Budget approved"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := File(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindPDF || !strings.Contains(d.Text, "Ship in March") || !strings.Contains(d.Text, "Budget approved") {
		t.Fatalf("%+v", d)
	}
	if !strings.Contains(d.Text, "--- page 2 ---") {
		t.Fatalf("pages are not marked: %q", d.Text)
	}
}

func TestPDFRefusesGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pdf")
	_ = os.WriteFile(path, []byte("%PDF-1.4 this is not really a pdf"), 0o600)
	if _, err := File(path); err == nil {
		t.Fatal("a damaged PDF was read")
	}
}

func FuzzPDF(f *testing.F) {
	f.Add(tinyPDF("hello"))
	f.Add([]byte("%PDF-1.7\n1 0 obj << >> endobj"))
	f.Fuzz(func(t *testing.T, b []byte) {
		d, err := FromBytes("f.pdf", KindPDF, b)
		if err == nil && len(d.Text) > MaxFileBytes+64 {
			t.Fatalf("text over the cap: %d", len(d.Text))
		}
	})
}
