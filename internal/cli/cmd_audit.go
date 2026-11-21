package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

// AuditFile is the audit log name inside a notebook (and the config dir).
const AuditFile = service.AuditFile

// auditPath is the notebook's log, or the user-level log for notebook-less runs.
func auditPath(nb *notebook.Notebook, env Env) (string, error) {
	if nb != nil {
		return filepath.Join(nb.Dir, AuditFile), nil
	}
	p, err := configPath(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), AuditFile), nil
}

func resolveAuditTarget(arg string) string {
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		return filepath.Join(arg, AuditFile)
	}
	return arg
}

func cmdAudit(args []string, env Env) error {
	if len(args) < 1 {
		return usagef("audit: want verify|show <notebook-or-file>")
	}
	sub := args[0]
	fs := newFlags("audit "+sub, env)
	last := fs.Int("last", 20, "show the last N entries")
	pos, err := parse(fs, args[1:])
	if err != nil {
		return err
	}
	target := ""
	if len(pos) == 1 {
		target = resolveAuditTarget(pos[0])
	} else if target, err = auditPath(nil, env); err != nil {
		return err
	}
	switch sub {
	case "verify":
		n, err := audit.VerifyFile(target)
		if err != nil {
			return fmt.Errorf("%s: %w (after %d valid entries)", target, err, n)
		}
		_, err = fmt.Fprintf(env.Stdout, "ok: %d entries verified in %s\n", n, target)
		return wrapWrite(err)
	case subShow:
		return showAudit(target, *last, env)
	default:
		return usagef("audit: unknown subcommand %q", sub)
	}
}

func showAudit(path string, last int, env Env) error {
	f, err := os.Open(path) //nolint:gosec // G304: user-chosen log
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), audit.MaxLineBytes+1)
	ring := make([]audit.Entry, 0, max(last, 1))
	for sc.Scan() {
		var e audit.Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if len(ring) == cap(ring) {
			ring = ring[1:]
		}
		ring = append(ring, e)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read audit log: %w", err)
	}
	for _, e := range ring {
		keys := make([]string, 0, len(e.Data))
		for k, v := range e.Data {
			keys = append(keys, fmt.Sprintf("%s=%v", k, v))
		}
		sortStrings(keys)
		line := fmt.Sprintf("%d %s %-14s %s %s\n", e.Seq, e.Time, e.Kind, e.RunID, strings.Join(keys, " "))
		if _, err := fmt.Fprint(env.Stdout, line); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}
