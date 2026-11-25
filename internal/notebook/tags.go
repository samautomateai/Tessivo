package notebook

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tag limits (NB-5).
const (
	MaxTags   = 10
	MaxTagLen = 32
)

// NormalizeTags trims tags, drops case-insensitive duplicates (the first
// spelling wins) and checks each is 1–MaxTagLen letters, digits, spaces, '-'
// or '_', with at most MaxTags in all.
func NormalizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		t := strings.TrimSpace(raw)
		if err := checkTag(t); err != nil {
			return nil, err
		}
		if k := strings.ToLower(t); !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	if len(out) > MaxTags {
		return nil, fmt.Errorf("at most %d tags", MaxTags)
	}
	return out, nil
}

func checkTag(t string) error {
	switch {
	case t == "":
		return errors.New("a tag is empty")
	case utf8.RuneCountInString(t) > MaxTagLen:
		return fmt.Errorf("tag %q is longer than %d characters", t, MaxTagLen)
	}
	for _, r := range t {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' && r != '_' {
			return fmt.Errorf("tag %q: use letters, digits, spaces, '-' and '_'", t)
		}
	}
	return nil
}

// HasTag reports whether the notebook has tag, ignoring case.
func (m Manifest) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if strings.EqualFold(t, strings.TrimSpace(tag)) {
			return true
		}
	}
	return false
}

// SetTags replaces the notebook's tags (see NormalizeTags).
func (n *Notebook) SetTags(tags []string) error {
	clean, err := NormalizeTags(tags)
	if err != nil {
		return err
	}
	return n.Update(func(m *Manifest) error {
		m.Tags = clean
		return nil
	})
}

// SetPinned pins or unpins the notebook (pinned notebooks are listed first).
func (n *Notebook) SetPinned(pinned bool) error {
	return n.Update(func(m *Manifest) error {
		m.Pinned = pinned
		return nil
	})
}
