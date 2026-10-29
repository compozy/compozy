package session

import (
	"strings"
)

// IsInternalSpawnRole reports whether a child is daemon-owned and must stay
// out of operator catalogs and metrics.
func IsInternalSpawnRole(role string) bool {
	trimmed := strings.TrimSpace(role)
	return strings.EqualFold(trimmed, SpawnRoleAutoTitle)
}

// spawnRuntimeMode starts daemon-owned input-only children without provider
// tools or hosted MCP: they only transform the prompt they are given.
func spawnRuntimeMode(role string) string {
	if IsInternalSpawnRole(role) {
		return RuntimeModeVerdictOnly
	}
	return ""
}
