package notebook

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{" Infra ", "infra", "Q3 review", "on-call_team"})
	if err != nil || !reflect.DeepEqual(got, []string{"Infra", "Q3 review", "on-call_team"}) {
		t.Fatalf("%v %v", got, err)
	}
	bad := map[string][]string{
		"a/b":        {"Infra", "a/b"},
		"at most 10": strings.Split("a b c d e f g h i j k", " "),
		"32":         {strings.Repeat("x", 33)},
		"empty":      {"  "},
	}
	for want, in := range bad {
		if _, err := NormalizeTags(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%v: got %v, want an error mentioning %q", in, err, want)
		}
	}
	if got, err := NormalizeTags(nil); err != nil || len(got) != 0 {
		t.Fatalf("nil: %v %v", got, err)
	}
}

func TestHasTag(t *testing.T) {
	m := Manifest{Tags: []string{"Infra"}}
	if !m.HasTag("infra") || m.HasTag("ops") {
		t.Fatal("HasTag compares case-insensitively")
	}
}
