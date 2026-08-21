package cmdpalette

import (
	"errors"
	"strings"
)

const aggregateProfileName = "All profiles"

// ProfileLens is the required profile axis for every command-palette projection.
// ID is either one stable profile ULID or the reserved aggregate identity @all.
type ProfileLens struct {
	ID   ProfileLensID `json:"profile_lens_id"`
	Name string        `json:"profile_name"`
}

// Validate rejects absent or malformed profile identity before any provider is called.
func (lens ProfileLens) Validate() error {
	if err := lens.ID.Validate(); err != nil {
		return err
	}
	if lens.ID == AggregateProfileLensID && strings.TrimSpace(lens.Name) != aggregateProfileName {
		return errors.New("cmd palette: aggregate profile lens must be labeled All profiles")
	}
	return nil
}

// IsAggregate reports whether the lens explicitly widens reads across profiles.
func (lens ProfileLens) IsAggregate() bool {
	return lens.ID == AggregateProfileLensID
}

// ScopedProfileLens constructs one real-profile lens.
func ScopedProfileLens(profileID ProfileLensID, profileName string) ProfileLens {
	return ProfileLens{ID: ProfileLensID(strings.TrimSpace(string(profileID))), Name: strings.TrimSpace(profileName)}
}

// AggregateProfileLens constructs the only valid aggregate lens.
func AggregateProfileLens() ProfileLens {
	return ProfileLens{ID: AggregateProfileLensID, Name: aggregateProfileName}
}
