package desktoprelease

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func ValidateDesktopChannel(channel string) error {
	switch channel {
	case ChannelBeta:
		return nil
	case ChannelStable:
		return fmt.Errorf("desktop release: stable channel is reserved and cannot publish")
	default:
		return fmt.Errorf("desktop release: unsupported channel %q", channel)
	}
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

func AssertDefaultComparator(source string) error {
	for _, forbidden := range []string{"allowDowngrades", "version_comparator"} {
		if strings.Contains(source, forbidden) {
			return fmt.Errorf("desktop release: custom updater comparator %q is forbidden", forbidden)
		}
	}
	return nil
}
