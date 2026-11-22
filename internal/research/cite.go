package research

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Citation is one valid [n] or [n:path] in an answer, resolved to the item it
// names. Path is set only for a file listed in that folder source.
type Citation struct {
	N     int    `json:"n"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
	Level string `json:"level,omitempty"`
	Path  string `json:"path,omitempty"`
}

var citeRE = regexp.MustCompile(`\[(\d{1,6})(?::([^\]\n]{1,512}))?\]`)

// ParseCitations resolves the [n] and [n:path] markers in text against the
// items the question was given. Unknown numbers are ignored; a path that is
// absolute, climbs out, or is not one of the folder's files is dropped (the
// citation keeps its source). Duplicates collapse.
func ParseCitations(text string, items []Item) []Citation {
	var out []Citation
	for _, m := range citeRE.FindAllStringSubmatch(text, -1) {
		c, ok := resolve(m[1], m[2], items)
		if ok && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

func resolve(num, rawPath string, items []Item) (Citation, bool) {
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 || n > len(items) {
		return Citation{}, false
	}
	it := items[n-1]
	c := Citation{N: n, Kind: it.Kind, ID: it.ID, Label: it.Label, Level: it.Level}
	if p := cleanPath(rawPath); p != "" {
		for _, f := range it.Files {
			if path.Clean(strings.ReplaceAll(f.Path, `\`, "/")) == p {
				c.Path = p
				break
			}
		}
	}
	return c, true
}

// cleanPath returns a relative, slash-separated path, or "" when the path is
// empty, absolute or climbs out of its folder.
func cleanPath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
		return ""
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return ""
		}
	}
	return path.Clean(p)
}

// Relabel replaces each marker that cites resolve with the item's label, so
// a later question (with new numbers) never sees stale ones: [2:a.go]
// becomes [payments-service: a.go]. Markers not in cites stay as they are.
func Relabel(text string, cites []Citation) string {
	labels := map[int]string{}
	paths := map[int][]string{}
	for _, c := range cites {
		labels[c.N] = c.Label
		if c.Path != "" {
			paths[c.N] = append(paths[c.N], c.Path)
		}
	}
	return citeRE.ReplaceAllStringFunc(text, func(m string) string {
		parts := citeRE.FindStringSubmatch(m)
		n, _ := strconv.Atoi(parts[1])
		label, ok := labels[n]
		if !ok {
			return m
		}
		if p := cleanPath(parts[2]); p != "" && slices.Contains(paths[n], p) {
			return "[" + label + ": " + p + "]"
		}
		return "[" + label + "]"
	})
}
