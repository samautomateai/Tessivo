package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/extract"
)

func TestPDFExportReadsBack(t *testing.T) {
	long := strings.Repeat("Every line of this paragraph wraps onto the next one when it is too wide. ", 120)
	b, err := PDF("Risk report", report+"\n"+long+"\n\nCafé € ünïcode 漢字\n")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-1.")) || !bytes.HasSuffix(bytes.TrimSpace(b), []byte("%%EOF")) {
		t.Fatalf("not a PDF: %q…", b[:min(40, len(b))])
	}
	d, err := extract.FromBytes("r.pdf", extract.KindPDF, b)
	if err != nil {
		t.Fatalf("our own PDF did not open: %v", err)
	}
	for _, want := range []string{
		"Risk report", "7 of 10", "Mitigations", "idempotency", "Retry <safely> & log",
		"a | b", "Café €", "--- page 2 ---",
	} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("exported PDF lacks %q", want)
		}
	}
	if strings.Contains(d.Text, "**") {
		t.Error("markdown markers left in")
	}
	if _, err := PDF("x", strings.Repeat("a", MaxMarkdown+1)); err == nil {
		t.Fatal("oversized report accepted")
	}
}

func FuzzPDFExport(f *testing.F) {
	for _, s := range []string{report, "(", ")", "\\", "# ", "**`", "\x00\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		b, err := PDF("t", md)
		if err != nil {
			return
		}
		if !bytes.HasPrefix(b, []byte("%PDF-1.")) || !bytes.Contains(b, []byte("%%EOF")) {
			t.Fatal("not a PDF")
		}
	})
}
