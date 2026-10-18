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
