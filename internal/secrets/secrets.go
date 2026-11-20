// Package secrets resolves secret references (API keys, tokens) at run time.
// Tessivo never writes secrets to config, notebooks, templates or logs; config
// files only hold a reference name that a Resolver turns into the value.
package secrets

import (
	"errors"
	"fmt"
	"os"
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
