package extract

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// KindPDF is a PDF document, read with pdfium compiled to WebAssembly: the
// parser runs sandboxed inside the core (wazero), so a hostile file cannot
// reach memory or files outside the sandbox.
const KindPDF = "pdf"

// PDF limits: file size is MaxArchiveBytes; text stops at MaxFileBytes.
const (
	maxPDFPages   = 2000
	pdfTimeout    = 60 * time.Second
	pdfPoolLimit  = 2
	pdfPageMarker = "\n\n--- page %d ---\n\n"
)

var (
	pdfOnce sync.Once
	pdfPool pdfium.Pool
	pdfErr  error
)

func pdfInstance(wait time.Duration) (pdfium.Pdfium, error) {
	pdfOnce.Do(func() {
		pdfPool, pdfErr = webassembly.Init(webassembly.Config{MinIdle: 0, MaxIdle: 1, MaxTotal: pdfPoolLimit})
	})
	if pdfErr != nil {
		return nil, fmt.Errorf("start the PDF reader: %w", pdfErr)
	}
	inst, err := pdfPool.GetInstance(wait)
	if err != nil {
		return nil, fmt.Errorf("PDF reader busy: %w", err)
	}
	return inst, nil
}

// PDF extracts the text of every page, marking page breaks, up to the limits.
func PDF(source string, b []byte) (Document, error) {
	deadline := time.NewTimer(pdfTimeout)
	defer deadline.Stop()
	inst, err := pdfInstance(pdfTimeout)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := pdfText(inst, b)
		done <- result{text, err}
	}()
	select {
	case r := <-done:
		_ = inst.Close()
		if r.err != nil {
			return Document{}, fmt.Errorf("%s: %w", source, r.err)
		}
		return Document{Source: source, Kind: KindPDF, Text: r.text}, nil
	case <-deadline.C:
		_ = inst.Kill() // a stuck parse: discard the sandbox
		return Document{}, fmt.Errorf("%s: reading the PDF took longer than %s", source, pdfTimeout)
	}
}

func pdfText(inst pdfium.Pdfium, b []byte) (string, error) {
	doc, err := inst.OpenDocument(&requests.OpenDocument{File: &b})
	if err != nil {
		return "", errors.New("not a readable PDF (damaged, or protected with a password)")
	}
	defer func() { _, _ = inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) }()
	count, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return "", fmt.Errorf("count pages: %w", err)
	}
	var sb strings.Builder
	for i := range min(count.PageCount, maxPDFPages) {
		page, err := inst.GetPageText(&requests.GetPageText{Page: requests.Page{
			ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i},
		}})
		if err != nil {
			return "", fmt.Errorf("page %d: %w", i+1, err)
		}
		if i > 0 {
			fmt.Fprintf(&sb, pdfPageMarker, i+1)
		}
		sb.WriteString(strings.TrimSpace(page.Text))
		if sb.Len() >= MaxFileBytes {
			return sb.String()[:MaxFileBytes] + "\n[… the rest of the PDF was cut]", nil
		}
	}
	if count.PageCount > maxPDFPages {
		fmt.Fprintf(&sb, "\n[… %d more pages not read]", count.PageCount-maxPDFPages)
	}
	return sb.String(), nil
}
