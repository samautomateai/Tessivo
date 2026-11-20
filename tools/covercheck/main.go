// Command covercheck fails when total statement coverage in a Go cover
// profile is below a threshold.
//
// Usage:
//
//	covercheck -profile coverage.out -min 80
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	fl := flag.NewFlagSet("covercheck", flag.ContinueOnError)
	fl.SetOutput(out)
	profile := fl.String("profile", "coverage.out", "path to a go test -coverprofile file")
	minPct := fl.Float64("min", 80, "minimum total coverage percentage")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	f, err := os.Open(*profile)
	if err != nil {
		_, _ = fmt.Fprintln(out, "covercheck:", err)
		return 2
	}
	defer func() { _ = f.Close() }()
	pct, err := Total(f)
	if err != nil {
		_, _ = fmt.Fprintln(out, "covercheck:", err)
		return 2
	}
	_, _ = fmt.Fprintf(out, "total coverage: %.1f%% (min %.1f%%)\n", pct, *minPct)
	if pct < *minPct {
		_, _ = fmt.Fprintln(out, "covercheck: coverage below threshold")
		return 1
	}
	return 0
}

type block struct {
	stmts   int
	covered bool
}

// Total parses a cover profile and returns statement coverage as a percentage.
// Blocks listed more than once (e.g. with -coverpkg) count once, covered if any
// entry covered them.
func Total(r io.Reader) (float64, error) {
	blocks := map[string]*block{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		if err := addLine(blocks, line); err != nil {
			return 0, err
		}
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("read profile: %w", err)
	}
	total, covered := 0, 0
	for _, b := range blocks {
		total += b.stmts
		if b.covered {
			covered += b.stmts
		}
	}
	if total == 0 {
		return 100, nil
	}
	return float64(covered) * 100 / float64(total), nil
}

func addLine(blocks map[string]*block, line string) error {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return fmt.Errorf("malformed line %q", line)
	}
	stmts, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("bad statement count in %q: %w", line, err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return fmt.Errorf("bad hit count in %q: %w", line, err)
	}
	b, ok := blocks[fields[0]]
	if !ok {
		b = &block{stmts: stmts}
		blocks[fields[0]] = b
	}
	if count > 0 {
		b.covered = true
	}
	return nil
}
