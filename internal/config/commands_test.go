package config

import "testing"

func TestCheckCommandRules(t *testing.T) {
	if err := CheckCommandRules([]CommandRule{{Program: "go", Args: []string{"test", "*"}}}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]CommandRule{
		{{Program: " "}},
		{{Program: "go", Args: make([]string, MaxCommandArgs+1)}},
		{{Program: "go", Args: []string{"a\x00b"}}},
		make([]CommandRule, MaxCommandRules+1),
	} {
		if CheckCommandRules(bad) == nil {
			t.Fatalf("want an error for %+v", bad[:min(len(bad), 1)])
		}
	}
}
