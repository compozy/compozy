// Package statekeys owns profile partitions in the window-manager state domain.
package statekeys

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const supersededPrefix = "superseded:"

// Snapshot identifies the current stored layout for a profile.
func Snapshot(profile string) string { return "snapshot:" + profile }

// Legacy identifies the exact pre-upgrade bytes preserved for a profile.
func Legacy(profile string) string { return "legacy:" + profile }

// Migration identifies the current upgrade's status for a profile.
func Migration(profile string) string { return "migration:" + profile }

// SupersededLegacy retains an earlier backup without replacing distinct bytes.
func SupersededLegacy(profile string, value []byte) string {
	digest := sha256.Sum256(value)
	return supersededPrefix + profile + ":" + hex.EncodeToString(digest[:])
}

// IsSupersededLegacy identifies backups retained independently of the latest upgrade.
func IsSupersededLegacy(key string) bool { return strings.HasPrefix(key, supersededPrefix) }

// Profile identifies every key that belongs to a window-manager profile partition.
func Profile(key string) (string, bool) {
	for _, prefix := range []string{"snapshot:", "legacy:", "migration:", supersededPrefix} {
		if profile, found := strings.CutPrefix(key, prefix); found {
			if prefix == supersededPrefix {
				profile, _, found = strings.Cut(profile, ":")
			}
			return profile, found && profile != ""
		}
	}
	return "", false
}
