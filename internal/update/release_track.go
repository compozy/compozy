package update

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type releaseTrack string

const (
	releaseTrackStable releaseTrack = "stable"
	releaseTrackBeta   releaseTrack = "beta"
)

func releaseTrackForVersion(raw string) (releaseTrack, *semver.Version, error) {
	parsed, err := parseVersion(raw)
	if err != nil {
		return "", nil, err
	}
	prerelease := parsed.Prerelease()
	if prerelease == "" {
		return releaseTrackStable, parsed, nil
	}
	if strings.Split(prerelease, ".")[0] == string(releaseTrackBeta) {
		return releaseTrackBeta, parsed, nil
	}
	return "", nil, fmt.Errorf("unsupported prerelease track %q", prerelease)
}

func isBetaVersionForLine(candidate *semver.Version, current *semver.Version) bool {
	if candidate == nil || current == nil {
		return false
	}
	prerelease := candidate.Prerelease()
	return prerelease != "" &&
		strings.Split(prerelease, ".")[0] == string(releaseTrackBeta) &&
		candidate.Major() == current.Major() &&
		candidate.Minor() == current.Minor()
}
