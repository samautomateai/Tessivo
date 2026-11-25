package notebook

import "fmt"

// Watch modes (SRC-8): what tess serve does when the notebook's in-place
// sources change.
const (
	WatchOff    = "off"    // nothing (the default)
	WatchNotify = "notify" // publish sources.changed
	WatchRun    = "run"    // publish and start a run
)

// CheckWatch validates a watch mode; off is stored as "".
func CheckWatch(mode string) (string, error) {
	switch mode {
	case "", WatchOff:
		return "", nil
	case WatchNotify, WatchRun:
		return mode, nil
	}
	return "", fmt.Errorf("watch %q: use off, notify or run", mode)
}

// SetWatch sets the watch mode (off, notify or run).
func (n *Notebook) SetWatch(mode string) error {
	w, err := CheckWatch(mode)
	if err != nil {
		return err
	}
	return n.Update(func(m *Manifest) error {
		m.Watch = w
		return nil
	})
}
