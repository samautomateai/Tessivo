// Package render turns validated structured outputs into readable Markdown.
package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Markdown renders an output object. order lists the keys in display order
// (normally the schema's required list); keys not listed are appended.
// A "title" key becomes the H1 heading.
func Markdown(data []byte, order []string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	var sb strings.Builder
	if t, ok := obj["title"].(string); ok && t != "" {
		fmt.Fprintf(&sb, "# %s\n\n", t)
	}
	for _, k := range keys(obj, order) {
		if k == "title" {
			continue
		}
		fmt.Fprintf(&sb, "## %s\n\n", heading(k))
		writeValue(&sb, obj[k])
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n") + "\n", nil
}

func keys(obj map[string]any, order []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range order {
		if _, ok := obj[k]; ok && !seen[k] {
			out, seen[k] = append(out, k), true
		}
	}
	var rest []string
	for k := range obj {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func heading(k string) string {
	s := strings.ReplaceAll(k, "_", " ")
	if s == "" {
		return s
	}
	if strings.EqualFold(s, "tldr") {
		return "TL;DR"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func writeValue(sb *strings.Builder, v any) {
	switch x := v.(type) {
	case []any:
		if len(x) == 0 {
			sb.WriteString("_None._\n")
			return
		}
		for _, item := range x {
			fmt.Fprintf(sb, "- %s\n", inline(item))
		}
	case map[string]any:
		for _, k := range keys(x, nil) {
			fmt.Fprintf(sb, "- **%s**: %s\n", heading(k), inline(x[k]))
		}
	default:
		fmt.Fprintf(sb, "%s\n", inline(v))
	}
}

// inline renders a value on one line; objects become "a — b — c" with the
// first field in bold.
func inline(v any) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", x), "0"), ".")
	case bool:
		if x {
			return "yes"
		}
		return "no"
	case map[string]any:
		ks := keys(x, nil)
		parts := make([]string, 0, len(ks))
		for i, k := range ks {
			s := inline(x[k])
			if i == 0 {
				s = "**" + s + "**"
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, " — ")
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			parts = append(parts, inline(item))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(x)
	}
}
