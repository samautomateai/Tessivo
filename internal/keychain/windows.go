//go:build windows

package keychain

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/samautomateai/tessivo/internal/secrets"
)

// Windows Credential Manager via advapi32 (standard library syscall only).

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = syscall.Errno(1168)
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type windowsStore struct{}

func newWindowsStore() Store { return windowsStore{} }

func target(ref string) (*uint16, error) {
	p, err := syscall.UTF16PtrFromString(Service + ":" + ref)
	if err != nil {
		return nil, fmt.Errorf("credential name: %w", err)
	}
	return p, nil
}

func (windowsStore) Get(ref string) (string, error) {
	if err := ValidRef(ref); err != nil {
		return "", err
	}
	name, err := target(ref)
	if err != nil {
		return "", err
	}
	var pcred *credential
	r, _, callErr := procCredRead.Call(uintptr(unsafe.Pointer(name)), credTypeGeneric, 0,
		uintptr(unsafe.Pointer(&pcred)))
	if r == 0 {
		if errors.Is(callErr, errorNotFound) {
			return "", fmt.Errorf("%w: credential %s", secrets.ErrNotFound, ref)
		}
		return "", fmt.Errorf("credential read: %w", callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred))) //nolint:errcheck // CredFree returns no error
	size := int(pcred.CredentialBlobSize)
	if size > MaxSecretBytes {
		return "", fmt.Errorf("credential %s is larger than %d bytes", ref, MaxSecretBytes)
	}
	return string(unsafe.Slice(pcred.CredentialBlob, size)), nil
}

func (windowsStore) Set(ref, value string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	if err := validValue(value); err != nil {
		return err
	}
	name, err := target(ref)
	if err != nil {
		return err
	}
	user, _ := syscall.UTF16PtrFromString(ref)
	blob := []byte(value)
	cred := credential{
		Type: credTypeGeneric, TargetName: name, CredentialBlobSize: uint32(len(blob)), //nolint:gosec // G115: <= 8 KiB
		CredentialBlob: &blob[0], Persist: credPersistLocalMachine, UserName: user,
	}
	if r, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0); r == 0 {
		return fmt.Errorf("credential write: %w", callErr)
	}
	return nil
}

func (windowsStore) Delete(ref string) error {
	if err := ValidRef(ref); err != nil {
		return err
	}
	name, err := target(ref)
	if err != nil {
		return err
	}
	r, _, callErr := procCredDelete.Call(uintptr(unsafe.Pointer(name)), credTypeGeneric, 0)
	if r == 0 && !errors.Is(callErr, errorNotFound) {
		return fmt.Errorf("credential delete: %w", callErr)
	}
	return nil
}
