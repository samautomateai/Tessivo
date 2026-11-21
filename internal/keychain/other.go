//go:build !windows

package keychain

func newWindowsStore() Store { return unavailable{} }
