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
	if prereleaseTrack(prerelease) == releaseTrackBeta {
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
		prereleaseTrack(prerelease) == releaseTrackBeta &&
		candidate.Major() == current.Major() &&
		candidate.Minor() == current.Minor()
}

// isBetaTrackCandidate accepts newer betas of the current line and any newer stable release,
// so beta installs move onto the stable track through their normal update.
func isBetaTrackCandidate(prerelease bool, candidate *semver.Version, current *semver.Version) bool {
	if prerelease {
		return isBetaVersionForLine(candidate, current)
	}
	return candidate != nil && current != nil && candidate.Prerelease() == "" && candidate.GreaterThan(current)
}

func prereleaseTrack(prerelease string) releaseTrack {
	track, _, _ := strings.Cut(prerelease, ".")
	return releaseTrack(track)
}
