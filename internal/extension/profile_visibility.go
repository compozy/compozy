package extensionpkg

import "strings"

// ProfileLens identifies the profile whose extension contributions are being projected.
type ProfileLens struct {
	ID   string
	Name string
}

func (lens ProfileLens) normalize() ProfileLens {
	return ProfileLens{
		ID:   strings.TrimSpace(lens.ID),
		Name: strings.TrimSpace(lens.Name),
	}
}

func (lens ProfileLens) valid() bool {
	normalized := lens.normalize()
	return normalized.ID != "" && normalized.Name != ""
}

func manifestPlacementVisible(placement, profileName string) bool {
	placement = strings.TrimSpace(placement)
	return placement == "" || placement == strings.TrimSpace(profileName)
}
