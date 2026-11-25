package secrets

import (
	"errors"
	"strings"
	"testing"
)

type failing struct{}

func (failing) Get(string) (string, error) { return "", errors.New("keychain locked") }

func TestEnv(t *testing.T) {
	t.Setenv("TESSIVO_LITELLM_KEY", "sk-123")
	if v, err := (Env{}).Get("tessivo-litellm.key"); err != nil || v != "sk-123" {
		t.Fatalf("got %q %v", v, err)
	}
	e := Env{Lookup: func(string) (string, bool) { return "", false }}
	if _, err := e.Get("x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMapAndChain(t *testing.T) {
	c := Chain{Map{"a": ""}, Map{"a": "1", "b": "2"}}
	if v, err := c.Get("a"); err != nil || v != "1" {
		t.Fatalf("got %q %v", v, err)
	}
	if _, err := c.Get("zzz"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := (Chain{failing{}, Map{"a": "1"}}).Get("a"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("want hard error to stop the chain, got %v", err)
	}
}

func TestRedact(t *testing.T) {
	got := Redact("auth failed for sk-secret-value; key=abc", "sk-secret-value", "abc")
	if got != "auth failed for [REDACTED]; key=abc" {
		t.Fatalf("got %q", got)
	}
}

func TestRedactPatterns(t *testing.T) {
	cases := map[string]string{
		"Authorization: Bearer abcdefgh12345": "Authorization: Bearer [REDACTED]",
		"key sk-ABCDEFGH1234 used":            "key [REDACTED] used",
		`{"api_key": "hunter22hunter"}`:       `{"api_key": "[REDACTED]"}`,
		"password=s3cretpass;":                "password=[REDACTED];",
		"nothing to hide":                     "nothing to hide",
	}
	for in, want := range cases {
		if got := RedactPatterns(in); got != want {
			t.Errorf("RedactPatterns(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactPatternsCatchesWellKnownTokenFormats(t *testing.T) {
	for _, s := range []string{
		"ghp_" + strings.Repeat("a1B2", 9),
		"github_pat_" + strings.Repeat("x", 40),
		"AKIAIOSFODNN7EXAMPLE",
		"xoxb-1234567890-abcdefghij",
	} {
		if got := RedactPatterns("use " + s + " here"); strings.Contains(got, s) {
			t.Errorf("%s was not redacted: %q", s, got)
		}
	}
	if got := RedactPatterns("ghp_short and AKIA-not"); got != "ghp_short and AKIA-not" {
		t.Errorf("look-alikes changed: %q", got)
	}
}
