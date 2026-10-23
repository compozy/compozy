package desktoprelease

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func ValidateDesktopChannel(channel string) error {
	switch channel {
	case ChannelStable, ChannelBeta:
		return nil
	default:
		return fmt.Errorf("desktop release: unsupported channel %q", channel)
	}
}

// CompatibilityChannels returns the channels whose live apps a release on channel must keep serving.
// Stable releases also serve the frozen beta channel, whose final bridge release reads the stable feed.
func CompatibilityChannels(channel string) ([]string, error) {
	switch channel {
	case ChannelStable:
		return []string{ChannelStable, ChannelBeta}, nil
	case ChannelBeta:
		return []string{ChannelBeta}, nil
	default:
		return nil, fmt.Errorf("desktop release: unsupported channel %q", channel)
	}
}

// MinAppVersion returns the lowest live app version across the compatibility channels, which the new
// runtime must keep serving. Without a live channel the release version is its own minimum.
func MinAppVersion(releaseVersion string, liveVersions []string) (string, error) {
	if err := ValidateVersion(releaseVersion); err != nil {
		return "", err
	}
	lowest := ""
	for _, live := range liveVersions {
		if live == "" {
			continue
		}
		if err := ValidateVersion(live); err != nil {
			return "", fmt.Errorf("desktop release: live feed version: %w", err)
		}
		if lowest == "" || semver.Compare("v"+live, "v"+lowest) < 0 {
			lowest = live
		}
	}
	if lowest == "" {
		return releaseVersion, nil
	}
	return lowest, nil
}

func ValidateVersion(version string) error {
	if version == "" || strings.HasPrefix(version, "v") || !semver.IsValid("v"+version) {
		return fmt.Errorf("desktop release: version %q must be strict unprefixed SemVer", version)
	}
	return nil
}

func AssertStrictlyGreater(candidate, live string) error {
	if err := ValidateVersion(candidate); err != nil {
		return err
	}
	if live == "" {
		return nil
	}
	if err := ValidateVersion(live); err != nil {
		return fmt.Errorf("desktop release: live feed version: %w", err)
	}
	if semver.Compare("v"+candidate, "v"+live) <= 0 {
		return fmt.Errorf("desktop release: candidate %s must be strictly greater than live %s", candidate, live)
	}
	return nil
}

func AssertCompatibleWithPrevious(minAppVersion, previousAppVersion string) error {
	if err := ValidateVersion(minAppVersion); err != nil {
		return fmt.Errorf("desktop release: min_app_version: %w", err)
	}
	if previousAppVersion == "" {
		return nil
	}
	if err := ValidateVersion(previousAppVersion); err != nil {
		return fmt.Errorf("desktop release: previous app version: %w", err)
	}
	if semver.Compare("v"+minAppVersion, "v"+previousAppVersion) > 0 {
		return fmt.Errorf(
			"desktop release: min_app_version %s exceeds previous channel app version %s",
			minAppVersion,
			previousAppVersion,
		)
	}
	return nil
}
