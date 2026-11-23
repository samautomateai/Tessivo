package service

import (
	"github.com/samautomateai/tessivo/internal/export"
)

// Export formats.
const (
	FormatMarkdown = "md"
	FormatDOCX     = "docx"
	FormatPDF      = "pdf"
)

// ExportFile is an output rendered as a file to save or share.
type ExportFile struct {
	Name        string
	ContentType string
	Data        []byte
}

// ExportOutput renders a saved output as Markdown or Word.
func (s *Service) ExportOutput(notebookID, outputID, format string) (ExportFile, error) {
	o, err := s.GetOutput(notebookID, outputID)
	if err != nil {
		return ExportFile{}, err
	}
	return RenderExport(o.Title, o.Markdown, format)
}

// RenderExport renders Markdown in a format (shared with the CLI).
func RenderExport(title, markdown, format string) (ExportFile, error) {
	switch format {
	case FormatMarkdown, "":
		return ExportFile{
			Name: export.FileName(title, FormatMarkdown), ContentType: "text/markdown; charset=utf-8",
			Data: []byte(markdown),
		}, nil
	case FormatDOCX:
		b, err := export.DOCX(title, markdown)
		if err != nil {
			return ExportFile{}, &InvalidError{err.Error()}
		}
		return ExportFile{
			Name:        export.FileName(title, FormatDOCX),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			Data:        b,
		}, nil
	case FormatPDF:
		b, err := export.PDF(title, markdown)
		if err != nil {
			return ExportFile{}, &InvalidError{err.Error()}
		}
		return ExportFile{Name: export.FileName(title, FormatPDF), ContentType: "application/pdf", Data: b}, nil
	default:
		return ExportFile{}, invalidf("format %q: want md, docx or pdf", format)
	}
}
