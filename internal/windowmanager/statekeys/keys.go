// Package statekeys owns profile partitions in the window-manager state domain.
package statekeys

// Snapshot identifies the current stored layout for a profile.
func Snapshot(profile string) string { return "snapshot:" + profile }

// Legacy identifies the exact pre-upgrade bytes preserved for a profile.
func Legacy(profile string) string { return "legacy:" + profile }
