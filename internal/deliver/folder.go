package deliver

import (
	"fmt"
	"os"
)

// testFolder checks that the folder exists and Tessivo may write to it.
func (c *Client) testFolder() error {
	info, err := os.Stat(c.in.Path)
	if err != nil {
		return fmt.Errorf("folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("folder: %s is not a folder", c.in.Path)
	}
	f, err := os.CreateTemp(c.in.Path, ".tessivo-check-*")
	if err != nil {
		return fmt.Errorf("folder: cannot write to %s: %w", c.in.Path, err)
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name) //nolint:wrapcheck // the check's own temporary file
}
