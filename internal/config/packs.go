package config

import (
	"fmt"
	"regexp"
)

// MaxPacks bounds the team packs added.
const MaxPacks = 20

var packID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// PackSource is a team pack added to this Tessivo: where it comes from and how.
type PackSource struct {
	ID     string `json:"id"`
	Source string `json:"source"`           // folder, .zip file or git URL
	Kind   string `json:"kind"`             // folder, zip or git
	Branch string `json:"branch,omitempty"` // git only; the remote's default when empty
}

// CheckPacks validates the pack list: at most MaxPacks, unique IDs, known kinds.
func CheckPacks(packs []PackSource) error {
	if len(packs) > MaxPacks {
		return fmt.Errorf("packs: at most %d", MaxPacks)
	}
	seen := map[string]bool{}
	for _, p := range packs {
		switch {
		case !packID.MatchString(p.ID):
			return fmt.Errorf("pack %q: id uses a-z, 0-9 and '-'", p.ID)
		case seen[p.ID]:
			return fmt.Errorf("pack %q: duplicate id", p.ID)
		case p.Kind != "folder" && p.Kind != "zip" && p.Kind != "git":
			return fmt.Errorf("pack %q: kind %q is not folder, zip or git", p.ID, p.Kind)
		case p.Source == "":
			return fmt.Errorf("pack %q: source is required", p.ID)
		}
		seen[p.ID] = true
	}
	return nil
}
