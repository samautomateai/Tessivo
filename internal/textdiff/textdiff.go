// Package textdiff compares two texts line by line: the shared start and end
// are trimmed, then the middle is aligned by longest common subsequence when
// it is small enough, else shown as removed then added. Time and memory are
// bounded by maxCells.
package textdiff

import "strings"

// Ops.
const (
	Same   = ' '
	Add    = '+'
	Remove = '-'
)

// maxCells bounds the LCS table (lines of a × lines of b).
const maxCells = 4_000_000

// Line is one line of a diff.
type Line struct {
	Op   byte   `json:"op"`
	Text string `json:"text"`
}

// Counts are the lines added and removed.
type Counts struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// Summary counts a diff's added and removed lines.
func Summary(lines []Line) Counts {
	var c Counts
	for _, l := range lines {
		switch l.Op {
		case Add:
			c.Added++
		case Remove:
			c.Removed++
		}
	}
	return c
}

func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Lines is the diff that turns a into b.
func Lines(a, b string) []Line {
	x, y := split(a), split(b)
	pre := 0
	for pre < len(x) && pre < len(y) && x[pre] == y[pre] {
		pre++
	}
	suf := 0
	for suf < len(x)-pre && suf < len(y)-pre && x[len(x)-1-suf] == y[len(y)-1-suf] {
		suf++
	}
	out := make([]Line, 0, len(x)+len(y))
	for _, l := range x[:pre] {
		out = append(out, Line{Same, l})
	}
	out = append(out, middle(x[pre:len(x)-suf], y[pre:len(y)-suf])...)
	for _, l := range x[len(x)-suf:] {
		out = append(out, Line{Same, l})
	}
	return out
}

func middle(x, y []string) []Line {
	if len(x)*len(y) > maxCells || len(x) == 0 || len(y) == 0 {
		return replaced(x, y)
	}
	// lcs[i][j] is the LCS length of x[i:] and y[j:].
	w := len(y) + 1
	lcs := make([]int32, (len(x)+1)*w)
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i*w+j] = lcs[(i+1)*w+j+1] + 1
			} else {
				lcs[i*w+j] = max(lcs[(i+1)*w+j], lcs[i*w+j+1])
			}
		}
	}
	out := make([]Line, 0, len(x)+len(y))
	i, j := 0, 0
	for i < len(x) && j < len(y) {
		switch {
		case x[i] == y[j]:
			out = append(out, Line{Same, x[i]})
			i, j = i+1, j+1
		case lcs[(i+1)*w+j] >= lcs[i*w+j+1]:
			out = append(out, Line{Remove, x[i]})
			i++
		default:
			out = append(out, Line{Add, y[j]})
			j++
		}
	}
	return append(out, replaced(x[i:], y[j:])...)
}

func replaced(x, y []string) []Line {
	out := make([]Line, 0, len(x)+len(y))
	for _, l := range x {
		out = append(out, Line{Remove, l})
	}
	for _, l := range y {
		out = append(out, Line{Add, l})
	}
	return out
}
