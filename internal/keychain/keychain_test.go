package keychain

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/testutil"

	"github.com/samautomateai/tessivo/internal/secrets"
)

type call struct {
	name  string
	args  []string
	stdin string
}

// fakeTool records calls and answers from a script keyed by the first arg.
type fakeTool struct {
	calls []call
	out   map[string]string
	exit  map[string]int
}

func TestMain(m *testing.M) {
	testutil.RunHelper()
	os.Exit(m.Run())
}

// exitErr returns a real *exec.ExitError with the given status, on any OS.
func exitErr(t *testing.T, code int) error {
	t.Helper()
	name, args := testutil.HelperCommand(t, "exit", strconv.Itoa(code))
	err := exec.Command(name, args...).Run()
	if err == nil {
		t.Fatal("expected exit error")
	}
	return err
}

func (f *fakeTool) runner(t *testing.T) runner {
	return func(_ context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		f.calls = append(f.calls, call{name, args, string(stdin)})
		key := args[0]
		if code, ok := f.exit[key]; ok {
			return nil, exitErr(t, code)
		}
		return []byte(f.out[key]), nil
	}
}

func TestMacStore(t *testing.T) {
	f := &fakeTool{out: map[string]string{"find-generic-password": "sk-value\n"}, exit: map[string]int{}}
	m := &macStore{run: f.runner(t), tool: "/usr/bin/security"}
	if err := m.Set("LITELLM_KEY", "s3cret value"); err != nil {
		t.Fatal(err)
	}
	set := f.calls[0]
	if strings.Join(set.args, " ") != "-i" || strings.Contains(strings.Join(set.args, " "), "s3cret") {
		t.Fatalf("secret must not be in argv: %v", set.args)
	}
	if !strings.Contains(set.stdin, "-X "+hex.EncodeToString([]byte("s3cret value"))) {
		t.Fatalf("stdin: %q", set.stdin)
	}
	if v, err := m.Get("LITELLM_KEY"); err != nil || v != "sk-value" {
		t.Fatalf("get: %q %v", v, err)
	}
	f.exit["find-generic-password"] = macNotFound
	if _, err := m.Get("LITELLM_KEY"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	f.exit["find-generic-password"] = 1
	if _, err := m.Get("LITELLM_KEY"); err == nil || errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("want hard error, got %v", err)
	}
	f.exit["delete-generic-password"] = macNotFound
	if err := m.Delete("LITELLM_KEY"); err != nil {
		t.Fatal("deleting a missing item is fine")
	}
	f.exit["delete-generic-password"] = 2
	if err := m.Delete("LITELLM_KEY"); err == nil {
		t.Fatal("want delete error")
	}
	f.exit["-i"] = 1
	if err := m.Set("LITELLM_KEY", "v"); err == nil {
		t.Fatal("want write error")
	}
}

func TestSecretToolStore(t *testing.T) {
	f := &fakeTool{out: map[string]string{"lookup": "abc123\n"}, exit: map[string]int{}}
	s := &secretToolStore{run: f.runner(t), tool: "secret-tool"}
	if err := s.Set("TESSIVO_KEY", "abc123"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.calls[0].args, " "), "abc123") || f.calls[0].stdin != "abc123" {
		t.Fatalf("secret must go through stdin: %+v", f.calls[0])
	}
	if v, err := s.Get("TESSIVO_KEY"); err != nil || v != "abc123" {
		t.Fatalf("get: %q %v", v, err)
	}
	f.out["lookup"] = ""
	if _, err := s.Get("TESSIVO_KEY"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("empty output means not found")
	}
	f.exit["lookup"] = 1
	if _, err := s.Get("TESSIVO_KEY"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("exit 1 without output means not found")
	}
	f.exit["lookup"] = 3
	if _, err := s.Get("TESSIVO_KEY"); err == nil || errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("want hard error")
	}
	if err := s.Delete("TESSIVO_KEY"); err != nil {
		t.Fatal(err)
	}
	f.exit["clear"], f.exit["store"] = 1, 1
	if s.Delete("TESSIVO_KEY") == nil || s.Set("TESSIVO_KEY", "x") == nil {
		t.Fatal("want tool errors")
	}
}

func TestValidation(t *testing.T) {
	stores := []Store{&macStore{run: nil}, &secretToolStore{run: nil}}
	for _, s := range stores {
		for _, bad := range []string{"", "a b", "-flag", "x;rm", strings.Repeat("a", 200)} {
			if _, err := s.Get(bad); err == nil {
				t.Errorf("%T Get(%q): want error", s, bad)
			}
			if s.Set(bad, "v") == nil || s.Delete(bad) == nil {
				t.Errorf("%T %q: want error", s, bad)
			}
		}
		if s.Set("OK", "") == nil || s.Set("OK", strings.Repeat("x", MaxSecretBytes+1)) == nil {
			t.Errorf("%T: want value size errors", s)
		}
	}
	if _, err := (Resolver{Store: unavailable{}}).Get("bad ref"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("invalid refs resolve as not found")
	}
	if _, err := (Resolver{Store: unavailable{}}).Get("OK"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("unavailable store resolves as not found so the chain continues")
	}
	if (unavailable{}).Set("a", "b") == nil || (unavailable{}).Delete("a") == nil {
		t.Fatal("unavailable must refuse writes")
	}
	if New() == nil || newWindowsStore() == nil {
		t.Fatal("New must return a store")
	}
}

func TestExecRunner(t *testing.T) {
	name, args := testutil.HelperCommand(t, "cat")
	out, err := execRunner(context.Background(), name, args, []byte("piped"))
	if err != nil || string(out) != "piped" {
		t.Fatalf("%q %v", out, err)
	}
	name, args = testutil.HelperCommand(t, "fail", "token=abcdefghijk", "7")
	_, err = execRunner(context.Background(), name, args, nil)
	if exitCode(err) != 7 || strings.Contains(err.Error(), "abcdefghijk") {
		t.Fatalf("want exit 7 with redacted stderr, got %v", err)
	}
	if exitCode(errors.New("x")) != -1 {
		t.Fatal("non-exit errors")
	}
}
