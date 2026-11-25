package worktype

import (
	"strings"
	"testing"
)

func TestBuiltins(t *testing.T) {
	all := All()
	if len(all) != 4 || all[0].ID != Analyze || all[3].ID != Summarize {
		t.Fatalf("all: %v", all)
	}
	for _, w := range all {
		s, err := w.ParsedSchema()
		if err != nil {
			t.Fatalf("%s: %v", w.ID, err)
		}
		if s.Type != "object" || len(s.Required) == 0 || w.SystemPrompt == "" || w.StarterPrompt == "" {
			t.Fatalf("%s incomplete", w.ID)
		}
		// Strict structured output needs every property required.
		if len(s.Required) != len(s.Properties) {
			t.Fatalf("%s: %d required vs %d properties", w.ID, len(s.Required), len(s.Properties))
		}
	}
}

func TestGet(t *testing.T) {
	w, err := Get(Analyze)
	if err != nil || w.Name != "Analyze" {
		t.Fatalf("%v %+v", err, w)
	}
	s, _ := w.ParsedSchema()
	good := `{"title":"t","summary":"s","risk_score":7,"risk_level":"high","findings":[],"impacted_areas":["a"],` +
		`"evidence":[{"claim":"c","source":"f.go"}],"mitigations":[],"open_questions":[]}`
	if err := s.ValidateJSON([]byte(good)); err != nil {
		t.Fatal(err)
	}
	if _, err := Get("poem"); err == nil || !strings.Contains(err.Error(), "unknown work type") {
		t.Fatalf("want unknown error, got %v", err)
	}
	bad := WorkType{ID: "x", Schema: []byte("{")}
	if _, err := bad.ParsedSchema(); err == nil {
		t.Fatal("want parse error")
	}
}

func TestMustSchemaPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	mustSchema("missing")
}
