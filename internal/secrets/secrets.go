// Package secrets resolves secret references (API keys, tokens) at run time.
// Tessivo never writes secrets to config, notebooks, templates or logs; config
// files only hold a reference name that a Resolver turns into the value.
package secrets

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ErrNotFound is returned when a reference has no value.
var ErrNotFound = errors.New("secret not found")

// Resolver looks up a secret by reference name.
type Resolver interface {
	Get(ref string) (string, error)
}

// Env resolves references from environment variables. A reference is used as
// the variable name, upper-cased with '-' and '.' mapped to '_'.
type Env struct {
	// Lookup defaults to os.LookupEnv; tests may replace it.
	Lookup func(string) (string, bool)
}

// Get returns the environment value for ref.
func (e Env) Get(ref string) (string, error) {
	look := e.Lookup
	if look == nil {
		look = os.LookupEnv
	}
	name := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToUpper(ref))
	if v, ok := look(name); ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w: environment variable %s", ErrNotFound, name)
}

// Map is an in-memory resolver for tests and for values entered in the UI.
type Map map[string]string

// Get returns the stored value for ref.
func (m Map) Get(ref string) (string, error) {
	if v, ok := m[ref]; ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
}

// Chain tries each resolver in order and returns the first value found.
type Chain []Resolver

// Get returns the first value any resolver has for ref.
func (c Chain) Get(ref string) (string, error) {
	for _, r := range c {
		v, err := r.Get(ref)
		if err == nil {
			return v, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return "", err
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
}

// Redact replaces every occurrence of the given secret values in s, so logs
// and error messages never leak them.
func Redact(s string, values ...string) string {
	for _, v := range values {
		if len(v) >= 4 {
			s = strings.ReplaceAll(s, v, "[REDACTED]")
		}
	}
	return s
}

// keyPatterns match common credential shapes so they are redacted even when
// the exact value is unknown (defence in depth for logs and audit entries).
var keyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}`),   // GitHub tokens
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,}`), // GitHub fine-grained tokens
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),  // AWS access key IDs
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`), // Slack tokens
	regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password)["']?\s*[:=]\s*["']?)[^\s"',;]{6,}`),
}

// RedactPatterns replaces credential-looking substrings with [REDACTED].
func RedactPatterns(s string) string {
	for _, re := range keyPatterns {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			sub := re.FindStringSubmatch(m)
			if len(sub) > 1 {
				return sub[1] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return s
}
