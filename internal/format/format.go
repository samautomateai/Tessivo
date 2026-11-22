// Package format versions the files Tessivo keeps in portable folders
// (notebook.json, research.json) so a newer Tessivo can upgrade older
// folders and an older one refuses newer folders instead of damaging them.
package format

import (
	"errors"
	"fmt"
)

// ErrNewer means a file was written by a newer Tessivo.
var ErrNewer = errors.New("made by a newer version of Tessivo; update Tessivo to open it")

// Upgrade brings m from *version to current. A missing version (0) is 1,
// the format before versions were recorded. steps[i] upgrades version i+1
// to i+2; *version advances only past steps that succeed.
func Upgrade[M any](what string, version *int, current int, steps []func(*M) error, m *M) error {
	if *version == 0 {
		*version = 1
	}
	if *version > current {
		return fmt.Errorf("%s uses format %d, this Tessivo reads up to %d: %w", what, *version, current, ErrNewer)
	}
	for *version < current {
		if *version-1 >= len(steps) {
			return fmt.Errorf("%s: no upgrade from format %d", what, *version)
		}
		if err := steps[*version-1](m); err != nil {
			return fmt.Errorf("%s: upgrade from format %d: %w", what, *version, err)
		}
		*version++
	}
	return nil
}
