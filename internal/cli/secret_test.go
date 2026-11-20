package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// memStore is an in-memory keychain for tests.
type memStore map[string]string

func (m memStore) Get(ref string) (string, error) {
	if v, ok := m[ref]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (m memStore) Set(ref, v string) error { m[ref] = v; return nil }
func (m memStore) Delete(ref string) error { delete(m, ref); return nil }

type failStore struct{}

func (failStore) Get(string) (string, error) { return "", errors.New("locked") }
func (failStore) Set(string, string) error   { return errors.New("locked") }
func (failStore) Delete(string) error        { return errors.New("locked") }

func TestSecretCommands(t *testing.T) {
	h := newHarness(t)
	store := memStore{}
	h.env.Keychain = store
	h.env.Secrets = secrets.Chain{secrets.Map{}, store}
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("sk-from-stdin\n"), false
	out := h.mustRun("secret", "set", "TESSIVO_LITELLM_KEY")
	if store["TESSIVO_LITELLM_KEY"] != "sk-from-stdin" || strings.Contains(out, "sk-from") {
		t.Fatalf("stored %q, output %q", store["TESSIVO_LITELLM_KEY"], out)
	}
	if out := h.mustRun("secret", "check", "TESSIVO_LITELLM_KEY"); strings.Contains(out, "sk-") {
		t.Fatal("check must never print the value")
	}
	h.mustRun("secret", "delete", "TESSIVO_LITELLM_KEY")
	if code := h.run("secret", "check", "TESSIVO_LITELLM_KEY"); code != ExitError {
		t.Fatal("want missing secret error")
	}
	for _, args := range [][]string{{"secret"}, {"secret", "set", "bad ref"}, {"secret", "zap", "X"}} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage error, got %d", args, code)
		}
	}
	h.env.Stdin = strings.NewReader("\n")
	if code := h.run("secret", "set", "X"); code != ExitUsage {
		t.Fatal("want empty value error")
	}
	h.env.Keychain = failStore{}
	h.env.Stdin = strings.NewReader("v\n")
	if h.run("secret", "set", "X") != ExitError || h.run("secret", "delete", "X") != ExitError {
		t.Fatal("want store errors")
	}
}

func TestReadSecretOnTTYPrompts(t *testing.T) {
	h := newHarness(t)
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("typed\n"), true
	v, err := readSecret(context.Background(), h.env, "Value: ")
	if err != nil || v != "typed" || !strings.Contains(h.errOut.String(), "Value: ") {
		t.Fatalf("%q %v %q", v, err, h.errOut)
	}
	noEcho(context.Background(), h.env)() // non-file stdin: no-op
}
