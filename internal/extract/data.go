package extract

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SampleRows is how many data rows a CSV summary includes.
const SampleRows = 20

// FullRowsLimit is the largest table included in full; bigger ones are sampled.
const FullRowsLimit = 200

// CSV summarises tabular data: columns, row count, per-column statistics
// computed by Tessivo (so the model never has to do arithmetic), and either
// all rows (small tables) or a sample.
func CSV(source string, b []byte) (Document, error) {
	r := csv.NewReader(bytes.NewReader(b))
	if strings.HasSuffix(strings.ToLower(source), ".tsv") {
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return Document{}, fmt.Errorf("%s: parse csv: %w", source, err)
	}
	if len(rows) == 0 {
		return Document{Source: source, Kind: "csv", Text: "(empty table)"}, nil
	}
	header, data := rows[0], rows[1:]
	var sb strings.Builder
	fmt.Fprintf(&sb, "Table %s: %d rows, %d columns\n\nColumns:\n", source, len(data), len(header))
	for i, h := range header {
		fmt.Fprintf(&sb, "- %s: %s\n", h, columnStats(data, i))
	}
	shown := data
	label := "All rows"
	if len(data) > FullRowsLimit {
		shown, label = data[:SampleRows], fmt.Sprintf("First %d rows (sample)", SampleRows)
	}
	fmt.Fprintf(&sb, "\n%s:\n%s\n", label, strings.Join(header, " | "))
	for _, row := range shown {
		sb.WriteString(strings.Join(row, " | "))
		sb.WriteByte('\n')
	}
	return Document{Source: source, Kind: "csv", Text: sb.String()}, nil
}

func columnStats(data [][]string, col int) string {
	var nums []float64
	distinct := map[string]int{}
	empty := 0
	for _, row := range data {
		if col >= len(row) || strings.TrimSpace(row[col]) == "" {
			empty++
			continue
		}
		v := strings.TrimSpace(row[col])
		distinct[v]++
		if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", ""), 64); err == nil {
			nums = append(nums, f)
		}
	}
	filled := len(data) - empty
	if filled > 0 && len(nums) == filled {
		minV, maxV, sum := math.Inf(1), math.Inf(-1), 0.0
		for _, n := range nums {
			minV, maxV, sum = math.Min(minV, n), math.Max(maxV, n), sum+n
		}
		return fmt.Sprintf("numeric, min %s, max %s, mean %s, sum %s, empty %d",
			num(minV), num(maxV), num(sum/float64(len(nums))), num(sum), empty)
	}
	return fmt.Sprintf("text, %d distinct values, empty %d%s", len(distinct), empty, topValues(distinct))
}

func topValues(counts map[string]int) string {
	if len(counts) == 0 || len(counts) > 12 {
		return ""
	}
	parts := make([]string, 0, len(counts))
	for v, c := range counts {
		parts = append(parts, fmt.Sprintf("%s (%d)", v, c))
	}
	sortStrings(parts)
	return ", values: " + strings.Join(parts, ", ")
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// MaxJSONChars caps how much JSON text is included.
const MaxJSONChars = 200_000

// JSON validates and pretty-prints JSON, truncating very large documents.
func JSON(source string, b []byte) (Document, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return Document{}, fmt.Errorf("%s: invalid json: %w", source, err)
	}
	text := buf.String()
	if len(text) > MaxJSONChars {
		text = text[:MaxJSONChars] + "\n… (truncated)"
	}
	return Document{Source: source, Kind: "json", Text: text}, nil
}
