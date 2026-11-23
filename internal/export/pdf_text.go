package export

import (
	"strconv"
	"strings"
)

// helvetica holds the widths (per 1000 units) of ASCII 32–126 in Helvetica.
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, // space – /
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, // 0 – ?
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, // @ – O
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556, // P – _
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, // ` – o
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584, // p – ~
}

// textWidthOf is the width in points of s in font (1 Helvetica, 2 bold, 3 Courier).
func textWidthOf(s string, font int, size float64) float64 {
	units := 0
	for _, r := range s {
		switch {
		case font == 3:
			units += 600
		case r >= 32 && r <= 126:
			units += helvetica[r-32]
		default:
			units += 556
		}
	}
	w := float64(units) * size / 1000
	if font == 2 {
		w *= 1.07 // Helvetica-Bold is slightly wider
	}
	return w
}

// cp1252 maps the Windows-1252 characters outside Latin-1 to their bytes.
var cp1252 = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88, '‰': 0x89,
	'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C, 'Ž': 0x8E, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95,
	'–': 0x96, '—': 0x97, '˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B, 'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// pdfString encodes s as a PDF literal string in WinAnsiEncoding: unknown
// characters become "?", and parentheses, backslashes and bytes outside
// printable ASCII are escaped.
func pdfString(s string) string {
	var sb strings.Builder
	sb.WriteByte('(')
	for _, r := range s {
		var b byte
		switch {
		case r >= 32 && r <= 126:
			b = byte(r)
		case r >= 0xA0 && r <= 0xFF:
			b = byte(r)
		case cp1252[r] != 0:
			b = cp1252[r]
		default:
			b = '?'
		}
		switch {
		case b == '(' || b == ')' || b == '\\':
			sb.WriteByte('\\')
			sb.WriteByte(b)
		case b < 32 || b > 126:
			sb.WriteString("\\" + strconv.FormatInt(int64(b), 8))
		default:
			sb.WriteByte(b)
		}
	}
	sb.WriteByte(')')
	return sb.String()
}
