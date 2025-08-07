package domain

import (
	"github.com/Masterminds/semver/v3"
)

// Version wraps semver.Version for additional methods.
type Version struct {
	*semver.Version
}

// NewVersion creates a new Version from a string.
func NewVersion(s string) (*Version, error) {
	v, err := semver.NewVersion(s)
	if err != nil {
		return nil, err
	}
	return &Version{v}, nil
}

// BumpMajor increments the major version.
func (v *Version) BumpMajor() *Version {
	newVer := v.IncMajor()
	return &Version{&newVer}
}

// BumpMinor increments the minor version.
func (v *Version) BumpMinor() *Version {
	newVer := v.IncMinor()
	return &Version{&newVer}
}

// BumpPatch increments the patch version.
func (v *Version) BumpPatch() *Version {
	newVer := v.IncPatch()
	return &Version{&newVer}
}

// Compare compares two versions.
func (v *Version) Compare(other *Version) int {
	return v.Version.Compare(other.Version)
}

// String returns the version string with v prefix.
func (v *Version) String() string {
	return "v" + v.Version.String()
}
