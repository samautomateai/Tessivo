// Package keychain stores secrets in the operating system's credential
// store: the macOS login keychain (via /usr/bin/security), the Secret
// Service on Linux (via secret-tool) and Windows Credential Manager (via
// advapi32). Secret values only ever travel through stdin or memory, never
// command-line arguments, so other processes cannot read them from argv.
package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"time"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// Service is the service name every Tessivo secret is filed under.
const Service = "tessivo"

// MaxSecretBytes bounds stored values.
const MaxSecretBytes = 8 << 10

// ErrUnavailable means no supported credential store was found.
var ErrUnavailable = errors.New("no OS credential store available")

// refPattern keeps references safe to pass as arguments and command words.
var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// Store reads and writes secrets by reference name.
type Store interface {
	Get(ref string) (string, error) // returns secrets.ErrNotFound when missing
	Set(ref, value string) error
	Delete(ref string) error
}

// ValidRef reports whether ref is an acceptable secret reference.
func ValidRef(ref string) error {
	if !refPattern.MatchString(ref) {
		return fmt.Errorf("secret reference %q: use letters, digits, '.', '_' or '-' (max 128)", ref)
	}
	return nil
}

func validValue(v string) error {
	if v == "" || len(v) > MaxSecretBytes {
		return fmt.Errorf("secret must be 1 to %d bytes", MaxSecretBytes)
	}
	return nil
}

// runner executes a program with stdin and returns stdout. Tests replace it.
type runner func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error)

// commandTimeout bounds each call to an OS tool.
const commandTimeout = 10 * time.Second

func execRunner(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), &toolError{err: err, stderr: secrets.RedactPatterns(errb.String())}
	}
	return out.Bytes(), nil
}

type toolError struct {
	err    error
	stderr string
}

func (e *toolError) Error() string { return fmt.Sprintf("%v: %s", e.err, e.stderr) }
func (e *toolError) Unwrap() error { return e.err }

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// New returns the store for this OS, or an unavailable store.
func New() Store {
	switch runtime.GOOS {
	case "darwin":
		return &macStore{run: execRunner, tool: "/usr/bin/security"}
	case "windows":
		return newWindowsStore()
	default:
		if path, err := exec.LookPath("secret-tool"); err == nil {
			return &secretToolStore{run: execRunner, tool: path}
		}
		return unavailable{}
	}
}

type unavailable struct{}

func (unavailable) Get(string) (string, error) {
	return "", fmt.Errorf("%w: %w", secrets.ErrNotFound, ErrUnavailable)
}
func (unavailable) Set(string, string) error { return ErrUnavailable }
func (unavailable) Delete(string) error      { return ErrUnavailable }

// Resolver adapts a Store to secrets.Resolver.
type Resolver struct{ Store Store }

// Get implements secrets.Resolver.
func (r Resolver) Get(ref string) (string, error) {
	if ValidRef(ref) != nil {
		return "", fmt.Errorf("%w: %s", secrets.ErrNotFound, ref)
	}
	return r.Store.Get(ref)
}
