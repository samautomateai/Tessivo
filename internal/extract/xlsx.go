package extract

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// Spreadsheet limits.
const (
	MaxSheets      = 50
	MaxSheetRows   = 100_000
	MaxSheetCols   = 1_000
	MaxSharedItems = 1_000_000
)

// XLSX extracts every sheet as a table summary (same format as CSV: columns,
// computed statistics and rows or a sample).
func XLSX(source string, b []byte) (Document, error) {
	z, err := openZip(b)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	shared, err := sharedStrings(z)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	sheets, err := sheetList(z)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", source, err)
	}
	var sb strings.Builder
	for _, sh := range sheets {
		data, err := z.read(sh.part)
		if err != nil {
			return Document{}, fmt.Errorf("%s: %w", source, err)
		}
		rows, truncated, err := sheetRows(data, shared)
		if err != nil {
			return Document{}, fmt.Errorf("%s sheet %q: %w", source, sh.name, err)
		}
		sb.WriteString(tableText(source+" › "+sh.name, rows))
		if truncated {
			fmt.Fprintf(&sb, "(sheet truncated at %d rows)\n", MaxSheetRows)
		}
		sb.WriteString("\n")
	}
	return Document{Source: source, Kind: KindXLSX, Text: strings.TrimSpace(sb.String())}, nil
}

type sheetRef struct{ name, part string }

func sheetList(z *safeZip) ([]sheetRef, error) {
	data, err := z.read("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(data, &wb); err != nil {
		return nil, fmt.Errorf("parse workbook: %w", err)
	}
	targets := map[string]string{}
	for _, r := range relationships(z, "xl/_rels/workbook.xml.rels") {
		targets[r.ID] = r.Target
	}
	var out []sheetRef
	for i, s := range wb.Sheets {
		if len(out) >= MaxSheets {
			break
		}
		part := "xl/worksheets/sheet" + strconv.Itoa(i+1) + ".xml"
		if t, ok := targets[s.RID]; ok {
			part = path.Clean(path.Join("xl", strings.TrimPrefix(t, "/xl/")))
		}
		if z.has(part) {
			out = append(out, sheetRef{name: s.Name, part: part})
		}
	}
	return out, nil
}

func sharedStrings(z *safeZip) ([]string, error) {
	if !z.has("xl/sharedStrings.xml") {
		return nil, nil
	}
	data, err := z.read("xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}
	ss := sharedCollector{}
	if err := walkXML(data, ss.token); err != nil {
		return nil, fmt.Errorf("shared strings: %w", err)
	}
	if ss.overflow {
		return nil, fmt.Errorf("%w: too many shared strings", ErrArchiveLimit)
	}
	return ss.out, nil
}

type sharedCollector struct {
	out      []string
	cur      strings.Builder
	inT      bool
	overflow bool
}

func (c *sharedCollector) token(tok xml.Token) {
	switch t := tok.(type) {
	case xml.StartElement:
		c.inT = t.Name.Local == "t"
		if t.Name.Local == "si" {
			c.cur.Reset()
		}
	case xml.EndElement:
		c.inT = false
		if t.Name.Local == "si" {
			if len(c.out) >= MaxSharedItems {
				c.overflow = true
				return
			}
			c.out = append(c.out, c.cur.String())
		}
	case xml.CharData:
		if c.inT {
			c.cur.Write(t)
		}
	}
}

// cell is the part of <c> we decode.
type cell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Value  string `xml:"v"`
	Inline string `xml:"is>t"`
}

func sheetRows(data []byte, shared []string) ([][]string, bool, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var rows [][]string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return rows, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("parse sheet: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		if len(rows) >= MaxSheetRows {
			return rows, true, nil
		}
		row, err := readRow(dec, se, shared)
		if err != nil {
			return nil, false, err
		}
		rows = append(rows, row)
	}
}

func readRow(dec *xml.Decoder, start xml.StartElement, shared []string) ([]string, error) {
	var row []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parse row: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "c" {
				continue
			}
			var c cell
			if err := dec.DecodeElement(&c, &t); err != nil {
				return nil, fmt.Errorf("parse cell: %w", err)
			}
			col := columnIndex(c.Ref, len(row))
			if col >= MaxSheetCols {
				continue
			}
			for len(row) <= col {
				row = append(row, "")
			}
			row[col] = cellValue(c, shared)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return row, nil
			}
		}
	}
}

func cellValue(c cell, shared []string) string {
	switch c.Type {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err != nil || i < 0 || i >= len(shared) {
			return ""
		}
		return shared[i]
	case "inlineStr":
		return c.Inline
	case "b":
		if c.Value == "1" {
			return "TRUE"
		}
		return "FALSE"
	default:
		return c.Value
	}
}

// columnIndex converts the letters of a cell reference ("BC12") to a 0-based
// column; without a reference the cell follows the previous one.
func columnIndex(ref string, next int) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
		if n > MaxSheetCols {
			return MaxSheetCols
		}
	}
	if n == 0 {
		return next
	}
	return n - 1
}
