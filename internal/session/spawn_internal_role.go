package session

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

func spawnNetworkParticipation(opts SpawnOpts) *participation.Request {
	if !IsInternalSpawnRole(opts.SpawnRole) {
		return opts.NetworkParticipation
	}
	return nil
}

func isMemoryExtractorSpawnRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), SpawnRoleMemoryExtractor)
}

// IsInternalSpawnRole reports whether a child is daemon-owned and must stay
// out of operator catalogs, metrics, and inherited collaboration channels.
func IsInternalSpawnRole(role string) bool {
	trimmed := strings.TrimSpace(role)
	return strings.EqualFold(trimmed, SpawnRoleMemoryExtractor) ||
		strings.EqualFold(trimmed, SpawnRoleAutoTitle)
}
