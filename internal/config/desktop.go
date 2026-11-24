package config

import (
	"errors"
	"fmt"
)

// DesktopConfig holds the desktop app's own preferences.
type DesktopConfig struct {
	// CloseToTray keeps the app running in the system tray when its window is
	// closed, so schedules and runs go on; Quit in the tray menu ends it.
	CloseToTray bool `json:"close_to_tray,omitempty"`
	// Theme is system (default when empty), light, dark or high-contrast.
	Theme string `json:"theme,omitempty"`
	// Zoom scales the whole UI, 0.8–2.0; 0 means 1.0.
	Zoom float64 `json:"zoom,omitempty"`
	// Layout remembers which panels are open and how wide they are.
	Layout *DesktopLayout `json:"layout,omitempty"`
}

// DesktopLayout is the workbench's panel state; nil and 0 mean defaults.
type DesktopLayout struct {
	SideBarOpen  *bool `json:"side_bar_open,omitempty"`
	SideBarWidth int   `json:"side_bar_width,omitempty"`
	PanelOpen    *bool `json:"panel_open,omitempty"`
	PanelWidth   int   `json:"panel_width,omitempty"`
}

// Desktop limits.
const (
	MinZoom       = 0.8
	MaxZoom       = 2.0
	MinPanelWidth = 180
	MaxPanelWidth = 480
)

var desktopThemes = map[string]bool{"": true, "system": true, "light": true, "dark": true, "high-contrast": true}

// Validate checks the theme, zoom and panel widths.
func (d DesktopConfig) Validate() error {
	var errs []error
	if !desktopThemes[d.Theme] {
		errs = append(errs, fmt.Errorf("desktop.theme %q: want system, light, dark or high-contrast", d.Theme))
	}
	if d.Zoom != 0 && (d.Zoom < MinZoom || d.Zoom > MaxZoom) {
		errs = append(errs, fmt.Errorf("desktop.zoom %g: want %g–%g", d.Zoom, MinZoom, MaxZoom))
	}
	if l := d.Layout; l != nil {
		errs = append(errs, checkWidth("desktop.layout.side_bar_width", l.SideBarWidth),
			checkWidth("desktop.layout.panel_width", l.PanelWidth))
	}
	return errors.Join(errs...)
}

func checkWidth(key string, w int) error {
	if w != 0 && (w < MinPanelWidth || w > MaxPanelWidth) {
		return fmt.Errorf("%s %d: want %d–%d", key, w, MinPanelWidth, MaxPanelWidth)
	}
	return nil
}
