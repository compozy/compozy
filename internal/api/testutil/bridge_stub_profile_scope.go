package testutil

import (
	"strings"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
)

func normalizeBridgeInstanceProfile(instance bridgepkg.BridgeInstance) bridgepkg.BridgeInstance {
	instance.ProfileID = strings.TrimSpace(instance.ProfileID)
	if instance.ProfileID == "" {
		instance.ProfileID = store.DefaultProfileID
	}
	return instance
}

func bridgeInstanceVisible(instance *bridgepkg.BridgeInstance, readScope store.ReadScope) bool {
	if instance == nil {
		return false
	}
	return readScope.Matches(normalizeBridgeInstanceProfile(*instance).ProfileID)
}

func filterBridgeInstancesByScope(
	instances []bridgepkg.BridgeInstance,
	readScope store.ReadScope,
) []bridgepkg.BridgeInstance {
	filtered := make([]bridgepkg.BridgeInstance, 0, len(instances))
	for _, instance := range instances {
		normalized := normalizeBridgeInstanceProfile(instance)
		if readScope.Matches(normalized.ProfileID) {
			filtered = append(filtered, normalized)
		}
	}
	return filtered
}
