package schema

import (
	"strings"
	"testing"
)

const report = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["title", "score", "areas"],
  "properties": {
    "title": {"type": "string"},
    "score": {"type": "integer", "minimum": 0, "maximum": 10},
    "level": {"type": "string", "enum": ["low", "medium", "high"]},
    "ratio": {"type": "number"},
    "ok": {"type": "boolean"},
    "areas": {"type": "array", "minItems": 1, "items": {"type": "string"}}
  }
}`

func mustParse(t *testing.T) *Schema {
	t.Helper()
	s, err := Parse([]byte(report))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestValidOutput(t *testing.T) {
	s := mustParse(t)
	good := "```json\n" + `{"title":"x","score":7,"level":"high","ratio":0.5,"ok":true,"areas":["a"]}` + "\n```"
	if err := s.ValidateJSON([]byte(good)); err != nil {
		t.Fatal(err)
	}
	if got := StripFences(good); !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Fatalf("StripFences: %q", got)
	}
}

func TestInvalidOutputListsAllProblems(t *testing.T) {
	s := mustParse(t)
	bad := `{"title":1,"score":11.5,"level":"extreme","ratio":"x","ok":"yes","areas":[],"extra":true}`
	err := s.ValidateJSON([]byte(bad))
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{
		"$.title: want string", "$.score: want integer", "above maximum", "not one of", "$.ratio: want number",
		"$.ok: want boolean", "at least 1 items", `unexpected property "extra"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestTypeMismatches(t *testing.T) {
	s := mustParse(t)
	for _, in := range []string{`[]`, `"s"`, `null`, `{"title":"t","score":-1,"areas":"no"}`, `{"areas":[1]}`} {
		if s.ValidateJSON([]byte(in)) == nil {
			t.Errorf("want error for %s", in)
		}
	}
	if s.ValidateJSON([]byte(`not json`)) == nil {
		t.Fatal("want JSON error")
	}
	if _, err := Parse([]byte(`{`)); err == nil {
		t.Fatal("want parse error")
	}
	if kind(int64(1)) != "int64" {
		t.Fatal("kind fallback")
	}
	if StripFences("```") != "" {
		t.Fatal("bare fence")
	}
}
