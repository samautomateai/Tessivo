package agenttools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/samautomateai/tessivo/internal/llm"
)

type readFile struct{ r *Roots }

func (readFile) Def() llm.ToolDef {
	return def("read_file", "Read a text file inside the run's folders, with line numbers. "+
		"Use start_line and end_line to read part of a large file.",
		`{"type":"object","properties":{"path":{"type":"string"},`+
			`"start_line":{"type":"integer","minimum":1},"end_line":{"type":"integer","minimum":1}},`+
			`"required":["path"]}`)
}

func (t readFile) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	p, err := t.r.Resolve(a.Path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(p) // #nosec G304 -- p is confined to the run's roots by Resolve
	if err != nil {
		return "", fmt.Errorf("read %s: %w", a.Path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", a.Path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder; use list_directory", a.Path)
	}
	return readLines(ctx, f, a.Path, max(a.StartLine, 1), a.EndLine)
}

var errBinary = errors.New("binary file, not shown")

// readLines returns lines start..end (end 0 = to the end) prefixed with their
// numbers, stopping at maxRead bytes of output.
func readLines(ctx context.Context, f io.Reader, name string, start, end int) (string, error) {
	br := bufio.NewReaderSize(f, 64<<10)
	if head, _ := br.Peek(8 << 10); bytes.IndexByte(head, 0) >= 0 {
		return "", fmt.Errorf("%s: %w", name, errBinary)
	}
	sc := bufio.NewScanner(br)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	var sb strings.Builder
	for n := 1; sc.Scan(); n++ {
		if n%1000 == 0 && ctx.Err() != nil {
			return "", fmt.Errorf("read %s: %w", name, ctx.Err())
		}
		if n < start {
			continue
		}
		if end > 0 && n > end {
			break
		}
		line := fmt.Sprintf("%d\t%s\n", n, sc.Text())
		if sb.Len()+len(line) > maxRead {
			fmt.Fprintf(&sb, "[truncated at 256 KiB before line %d; use start_line to read further]\n", n)
			return sb.String(), nil
		}
		sb.WriteString(line)
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return sb.String(), nil
}
