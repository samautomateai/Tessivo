package cli

import (
	"bytes"
	"fmt"
	"os"
)

// cmdDiagnostics writes a bundle to attach to a bug report.
func cmdDiagnostics(args []string, env Env) error {
	fs := newFlags("diagnostics", env)
	out := fs.String("out", "tessivo-diagnostics.zip", "file to write")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	svc.SecretStore = env.Keychain
	var buf bytes.Buffer
	if err := svc.Diagnostics(&buf, Version, logPath(svc.ConfigPath)); err != nil {
		return err
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("diagnostics: %w", err)
	}
	_, err = fmt.Fprintf(env.Stdout, "wrote %s (no keys, notebook names or content)\n", *out)
	return wrapWrite(err)
}
