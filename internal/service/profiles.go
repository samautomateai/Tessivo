package service

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/samautomateai/tessivo/internal/config"
)

// ProfileSummary describes one settings profile for a list.
type ProfileSummary struct {
	Name         string `json:"name"`
	Providers    int    `json:"providers"`
	DefaultModel string `json:"defaultModel,omitempty"`
}

// ProfilesInfo is the profiles and the active one ("" when none is).
type ProfilesInfo struct {
	Active   string           `json:"active"`
	Profiles []ProfileSummary `json:"profiles"`
}

func profilesInfo(f config.File) ProfilesInfo {
	out := ProfilesInfo{Active: f.Profile, Profiles: []ProfileSummary{}}
	for _, p := range f.Profiles {
		out.Profiles = append(out.Profiles, ProfileSummary{
			Name: p.Name, Providers: len(p.Providers), DefaultModel: p.Defaults.Model,
		})
	}
	return out
}

// Profiles lists the settings profiles (SET-10).
func (s *Service) Profiles() ProfilesInfo { return profilesInfo(s.cfg()) }

// SaveProfile stores the current providers, aliases, defaults and limits as
// profile name (replacing one of that name) and makes it active.
func (s *Service) SaveProfile(name string) (ProfilesInfo, error) {
	return s.changeProfiles(func(f config.File) (config.File, error) {
		p := config.ProfileOf(name, f)
		i := slices.IndexFunc(f.Profiles, func(q config.Profile) bool { return q.Name == name })
		f.Profiles = slices.Clone(f.Profiles)
		if i >= 0 {
			f.Profiles[i] = p
		} else {
			f.Profiles = append(f.Profiles, p)
		}
		f.Profile = name
		return f, nil
	})
}

// UseProfile keeps the current settings in the active profile, then applies
// profile name. A profile that does not validate changes nothing.
func (s *Service) UseProfile(name string) (ProfilesInfo, error) {
	return s.changeProfiles(func(f config.File) (config.File, error) {
		p, ok := config.FindProfile(f.Profiles, name)
		if !ok {
			return f, fmt.Errorf("profile %q: %w", name, ErrNotFound)
		}
		f.Profiles = slices.Clone(f.Profiles)
		if i := slices.IndexFunc(f.Profiles, func(q config.Profile) bool { return q.Name == f.Profile }); i >= 0 {
			f.Profiles[i] = config.ProfileOf(f.Profile, f)
		}
		return f.WithProfile(p), nil
	})
}

// DeleteProfile removes a profile; deleting the active one keeps the settings
// and leaves no profile active.
func (s *Service) DeleteProfile(name string) (ProfilesInfo, error) {
	return s.changeProfiles(func(f config.File) (config.File, error) {
		next := slices.DeleteFunc(slices.Clone(f.Profiles), func(p config.Profile) bool { return p.Name == name })
		if len(next) == len(f.Profiles) {
			return f, fmt.Errorf("profile %q: %w", name, ErrNotFound)
		}
		f.Profiles = next
		if f.Profile == name {
			f.Profile = ""
		}
		return f, nil
	})
}

// changeProfiles applies change to the live config, validates the result and
// writes the profile-related parts to the config file; a refusal changes nothing.
func (s *Service) changeProfiles(change func(config.File) (config.File, error)) (ProfilesInfo, error) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	next, err := change(s.Config)
	if err != nil {
		return ProfilesInfo{}, err
	}
	if err := next.Validate(); err != nil {
		return ProfilesInfo{}, &InvalidError{err.Error()}
	}
	if s.ConfigPath != "" {
		f, err := config.Load(s.ConfigPath)
		if errors.Is(err, fs.ErrNotExist) {
			f, err = next, nil
		}
		if err != nil {
			return ProfilesInfo{}, fmt.Errorf("profiles: read config: %w", err)
		}
		f.Providers, f.Aliases, f.Defaults, f.Limits = next.Providers, next.Aliases, next.Defaults, next.Limits
		f.Profiles, f.Profile = next.Profiles, next.Profile
		if err := f.Save(s.ConfigPath); err != nil {
			return ProfilesInfo{}, &InvalidError{err.Error()}
		}
	}
	s.Config = next
	return profilesInfo(next), nil
}
