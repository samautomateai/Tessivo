package notebook

import (
	"strings"
	"sync"
	"testing"
)

func TestSuggestionCheck(t *testing.T) {
	cases := []struct {
		name string
		s    Suggestion
		ok   bool
	}{
		{"memory", Suggestion{Kind: SuggestMemory, Text: "cite file paths"}, true},
		{"prompt", Suggestion{Kind: SuggestPrompt, Text: "Assess {{pr}}"}, true},
		{"empty", Suggestion{Kind: SuggestMemory, Text: "  "}, false},
		{"unknown kind", Suggestion{Kind: "shell", Text: "x"}, false},
		{"memory too long", Suggestion{Kind: SuggestMemory, Text: strings.Repeat("a", 1001)}, false},
		{"prompt too long", Suggestion{Kind: SuggestPrompt, Text: strings.Repeat("a", MaxPromptText+1)}, false},
		{"bad utf8", Suggestion{Kind: SuggestMemory, Text: "a\xffb"}, false},
	}
	for _, c := range cases {
		if err := c.s.Check(1000); (err == nil) != c.ok {
			t.Errorf("%s: err=%v", c.name, err)
		}
	}
}

func open(n int) []Suggestion {
	out := make([]Suggestion, n)
	for i := range out {
		out[i] = Suggestion{ID: NewID(), Kind: SuggestMemory, Text: "x", Status: SuggestOpen}
	}
	return out
}

func TestUpdateSuggestionsCapsOpen(t *testing.T) {
	nb := newNB(t)
	out := NewID()
	if err := nb.UpdateSuggestions(out, func(s []Suggestion) ([]Suggestion, error) {
		return append(s, open(MaxOpenSuggestions)...), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := nb.UpdateSuggestions(out, func(s []Suggestion) ([]Suggestion, error) {
		return append(s, open(1)...), nil
	}); err == nil {
		t.Fatal("want the open cap to reject a 21st suggestion")
	}
	got, err := nb.Suggestions(out)
	if err != nil || len(got) != MaxOpenSuggestions {
		t.Fatalf("after reject: %d %v", len(got), err)
	}
	// Resolved suggestions do not count toward the cap.
	if err := nb.UpdateSuggestions(out, func(s []Suggestion) ([]Suggestion, error) {
		s[0].Status = SuggestDismissed
		return append(s, open(1)...), nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSuggestionsConcurrent(t *testing.T) {
	nb := newNB(t)
	out := NewID()
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := nb.UpdateSuggestions(out, func(s []Suggestion) ([]Suggestion, error) {
				return append(s, open(1)...), nil
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := nb.Suggestions(out)
	if err != nil || len(got) != 10 {
		t.Fatalf("got %d, %v", len(got), err)
	}
}

func TestSuggestionsBadOutputID(t *testing.T) {
	nb := newNB(t)
	if _, err := nb.Suggestions("../x"); err == nil {
		t.Fatal("want error")
	}
	if err := nb.UpdateSuggestions("../x", func(s []Suggestion) ([]Suggestion, error) { return s, nil }); err == nil {
		t.Fatal("want error")
	}
}

func TestSuggestionsMissingIsEmpty(t *testing.T) {
	nb := newNB(t)
	got, err := nb.Suggestions(NewID())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
}
