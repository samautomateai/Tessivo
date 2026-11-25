package config

import (
	"errors"
	"fmt"
	"slices"
)

// MaxProfiles bounds the settings profiles in a config.
const MaxProfiles = 10

// Profile is a named set of providers, aliases, run defaults and token limits
// (SET-10). It never holds a key: providers keep only their api_key_ref.
type Profile struct {
	Name      string      `json:"name"`
	Providers []Provider  `json:"providers"`
	Aliases   []Alias     `json:"aliases"`
	Defaults  RunSettings `json:"defaults"`
	Limits    Limits      `json:"limits"`
}

// ProfileOf captures f's current providers, aliases, defaults and limits.
func ProfileOf(name string, f File) Profile {
	return Profile{
		Name: name, Providers: slices.Clone(f.Providers), Aliases: slices.Clone(f.Aliases),
		Defaults: f.Defaults, Limits: f.Limits,
	}
}

// WithProfile returns f with p's parts in place and p active; everything else stays.
func (f File) WithProfile(p Profile) File {
	f.Providers, f.Aliases = slices.Clone(p.Providers), slices.Clone(p.Aliases)
	f.Defaults, f.Limits = p.Defaults, p.Limits
	if f.Providers == nil {
		f.Providers = []Provider{}
	}
	if f.Aliases == nil {
		f.Aliases = []Alias{}
	}
	f.Profile = p.Name
	return f
}

// FindProfile returns the profile with this name.
func FindProfile(list []Profile, name string) (Profile, bool) {
	for _, p := range list {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// checkProfiles validates names, the count, the active name and each profile as settings.
func (f File) checkProfiles() error {
	if len(f.Profiles) > MaxProfiles {
		return fmt.Errorf("profiles: at most %d", MaxProfiles)
	}
	var errs []error
	seen := map[string]bool{}
	for _, p := range f.Profiles {
		if !mcpName.MatchString(p.Name) {
			errs = append(errs, fmt.Errorf("profile name %q: use 1-32 lower-case letters, digits, '-' or '_'", p.Name))
		}
		if seen[p.Name] {
			errs = append(errs, fmt.Errorf("profiles: duplicate name %q", p.Name))
		}
		seen[p.Name] = true
		g := Default().WithProfile(p)
		g.Profiles, g.Profile = nil, ""
		if err := g.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("profile %s: %w", p.Name, err))
		}
	}
	if f.Profile != "" && !seen[f.Profile] {
		errs = append(errs, fmt.Errorf("profile %q is active but not defined", f.Profile))
	}
	return errors.Join(errs...)
}
