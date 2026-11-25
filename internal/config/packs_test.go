package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckPacks(t *testing.T) {
	ok := []PackSource{{ID: "team", Source: "https://example.com/p.git", Kind: "git"}}
	if err := CheckPacks(ok); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]PackSource{
		"duplicate": {ok[0], ok[0]},
		"id":        {{ID: "Bad", Source: "x", Kind: "git"}},
		"kind":      {{ID: "a", Source: "x", Kind: "ftp"}},
		"source":    {{ID: "a", Kind: "zip"}},
	}
	for want, list := range bad {
		if err := CheckPacks(list); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	many := make([]PackSource, MaxPacks+1)
	for i := range many {
		many[i] = PackSource{ID: fmt.Sprintf("p%d", i), Source: "x", Kind: "zip"}
	}
	if err := CheckPacks(many); err == nil {
		t.Fatal("too many packs")
	}
}
