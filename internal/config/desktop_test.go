package config

import (
	"strings"
	"testing"
)

func TestDesktopConfigValidate(t *testing.T) {
	open, closed := true, false
	for _, d := range []DesktopConfig{
		{},
		{Theme: "system"},
		{Theme: "light"},
		{Theme: "dark"},
		{Theme: "high-contrast"},
		{Zoom: 0.8},
		{Zoom: 2.0},
		{Zoom: 1.3},
		{Layout: &DesktopLayout{SideBarOpen: &open, SideBarWidth: 180, PanelOpen: &closed, PanelWidth: 480}},
		{Layout: &DesktopLayout{}},
	} {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	for _, c := range []struct {
		want string
		d    DesktopConfig
	}{
		{"desktop.theme", DesktopConfig{Theme: "purple"}},
		{"desktop.zoom", DesktopConfig{Zoom: 0.5}},
		{"desktop.zoom", DesktopConfig{Zoom: 9}},
		{"desktop.layout.side_bar_width", DesktopConfig{Layout: &DesktopLayout{SideBarWidth: 100}}},
		{"desktop.layout.panel_width", DesktopConfig{Layout: &DesktopLayout{PanelWidth: 900}}},
	} {
		want, d := c.want, c.d
		err := d.Validate()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%+v: want an error naming %s, got %v", d, want, err)
		}
	}
	f := Default()
	f.Desktop.Theme = "purple"
	if f.Validate() == nil {
		t.Fatal("File.Validate must check the desktop block")
	}
}
