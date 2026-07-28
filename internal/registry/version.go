package registry

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

// VersionIsNewer reports whether latest is a valid semantic version newer than
// current. Invalid version strings never produce an update signal.
func VersionIsNewer(current string, latest string) bool {
	normalizedCurrent := normalizeVersion(current)
	normalizedLatest := normalizeVersion(latest)
	if normalizedLatest == "" {
		return false
	}

	latestVersion, latestOK := parseSemanticVersion(normalizedLatest)
	if !latestOK {
		return false
	}
	if normalizedCurrent == "" {
		return true
	}

	currentVersion, currentOK := parseSemanticVersion(normalizedCurrent)
	if !currentOK {
		return false
	}

	return compareSemanticVersions(currentVersion, latestVersion) < 0
}

func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return trimmed
}

type semanticVersion struct {
	version *semver.Version
}

func parseSemanticVersion(version string) (semanticVersion, bool) {
	parsed, err := semver.StrictNewVersion(strings.TrimSpace(version))
	if err != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{version: parsed}, true
}

func compareSemanticVersions(current semanticVersion, latest semanticVersion) int {
	return current.version.Compare(latest.version)
}
