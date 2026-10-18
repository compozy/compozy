package update

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

func isDevVersion(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "dev") || strings.EqualFold(trimmed, "unknown") {
		return true
	}
	if strings.HasSuffix(trimmed, "-dirty") {
		return true
	}
	return isGitShortSHA(trimmed)
}

func compareVersions(currentVersion string, latestVersion string) (int, error) {
	current, err := parseVersion(currentVersion)
	if err != nil {
		return 0, fmt.Errorf("parse current version %q: %w", currentVersion, err)
	}
	latest, err := parseVersion(latestVersion)
	if err != nil {
		return 0, fmt.Errorf("parse latest version %q: %w", latestVersion, err)
	}
	return current.Compare(latest), nil
}

func parseVersion(raw string) (*semver.Version, error) {
	return semver.NewVersion(strings.TrimPrefix(trimGitDescribeSuffix(raw), "v"))
}

// trimGitDescribeSuffix collapses `vX.Y.Z-N-g<sha>` strings back to the base tag.
func trimGitDescribeSuffix(raw string) string {
	trimmed := strings.TrimSpace(raw)
	beforeCommit, commit, found := strings.CutLast(trimmed, "-g")
	if !found || commit == "" {
		return trimmed
	}
	if !isGitShortSHA(commit) {
		return trimmed
	}
	tag, count, found := strings.CutLast(beforeCommit, "-")
	if !found || count == "" {
		return trimmed
	}
	for _, char := range count {
		if char < '0' || char > '9' {
			return trimmed
		}
	}
	return tag
}

// isGitShortSHA reports whether value matches the short commit suffix from `git describe`.
func isGitShortSHA(value string) bool {
	if len(value) < 7 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}
