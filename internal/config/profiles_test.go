package config

import (
	"testing"
)

func TestProfilesValidate(t *testing.T) {
	f := Default()
	p := ProfileOf("work", f)
	if p.Name != "work" || len(p.Providers) != len(f.Providers) || p.Defaults.Model != f.Defaults.Model {
		t.Fatalf("snapshot: %+v", p)
	}
	f.Profiles, f.Profile = []Profile{p}, "work"
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	for what, mutate := range map[string]func(*File){
		"name":      func(f *File) { f.Profiles[0].Name = "Not OK" },
		"duplicate": func(f *File) { f.Profiles = append(f.Profiles, f.Profiles[0]) },
		"active":    func(f *File) { f.Profile = "missing" },
		"provider":  func(f *File) { f.Profiles[0].Providers[0].BaseURL = "ftp://x" },
		"too many": func(f *File) {
			for i := range MaxProfiles {
				f.Profiles = append(f.Profiles, Profile{Name: "p" + string(rune('a'+i))})
			}
		},
	} {
		g := f
		g.Profiles = append([]Profile{}, f.Profiles...)
		g.Profiles[0].Providers = append([]Provider{}, f.Profiles[0].Providers...)
		mutate(&g)
		if err := g.Validate(); err == nil {
			t.Errorf("%s: want an error", what)
		}
	}
	applied := f.WithProfile(Profile{Name: "home", Providers: []Provider{}, Aliases: []Alias{}})
	if len(applied.Providers) != 0 || applied.Profile != "home" || len(applied.MCP.Servers) != len(f.MCP.Servers) {
		t.Fatalf("apply replaces only the profile's parts: %+v", applied)
	}
	if _, ok := FindProfile(f.Profiles, "work"); !ok {
		t.Fatal("names")
	}
}
