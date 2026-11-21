package keychain

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// macStore drives /usr/bin/security in interactive mode (-i): commands are
// written to stdin, and the secret is passed hex-encoded (-X), so it never
// appears in argv and needs no quoting.
type macStore struct {
	run  runner
	tool string
}

// macNotFound is security's exit status for a missing item.
const macNotFound = 44

func (m *macStore) Get(ref string) (string, error) {
	if err := ValidRef(ref); err != nil {
		return "", err
	}
	args := []string{"find-generic-password", "-s", Service, "-a", ref, "-w"}
	out, err := m.run(context.Background(), m.tool, args, nil)
	if err != nil {
		if exitCode(err) == macNotFound {
			return "", fmt.Errorf("%w: keychain item %s", secrets.ErrNotFound, ref)
		}
		return "", fmt.Errorf("keychain read: %w", err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (m *macStore) Set(ref, value string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	if err := validValue(value); err != nil {
		return err
	}
	cmd := fmt.Sprintf("add-generic-password -U -s %s -a %s -l %s -X %s\n",
		Service, ref, "Tessivo:"+ref, hex.EncodeToString([]byte(value)))
	if _, err := m.run(context.Background(), m.tool, []string{"-i"}, []byte(cmd)); err != nil {
		return fmt.Errorf("keychain write: %w", err)
	}
	return nil
}

func (m *macStore) Delete(ref string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	_, err := m.run(context.Background(), m.tool, []string{"delete-generic-password", "-s", Service, "-a", ref}, nil)
	if err != nil && exitCode(err) != macNotFound {
		return fmt.Errorf("keychain delete: %w", err)
	}
	return nil
}

// secretToolStore uses libsecret's secret-tool (GNOME Keyring, KWallet via
// the Secret Service API). store reads the secret from stdin.
type secretToolStore struct {
	run  runner
	tool string
}

func (s *secretToolStore) attrs(ref string) []string {
	return []string{"service", Service, "account", ref}
}

func (s *secretToolStore) Get(ref string) (string, error) {
	if err := ValidRef(ref); err != nil {
		return "", err
	}
	out, err := s.run(context.Background(), s.tool, append([]string{"lookup"}, s.attrs(ref)...), nil)
	if err != nil {
		if exitCode(err) == 1 && len(out) == 0 {
			return "", fmt.Errorf("%w: secret %s", secrets.ErrNotFound, ref)
		}
		return "", fmt.Errorf("secret service read: %w", err)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("%w: secret %s", secrets.ErrNotFound, ref)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (s *secretToolStore) Set(ref, value string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	if err := validValue(value); err != nil {
		return err
	}
	args := append([]string{"store", "--label=Tessivo: " + ref}, s.attrs(ref)...)
	if _, err := s.run(context.Background(), s.tool, args, []byte(value)); err != nil {
		return fmt.Errorf("secret service write: %w", err)
	}
	return nil
}

func (s *secretToolStore) Delete(ref string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	if _, err := s.run(context.Background(), s.tool, append([]string{"clear"}, s.attrs(ref)...), nil); err != nil {
		return fmt.Errorf("secret service delete: %w", err)
	}
	return nil
}
