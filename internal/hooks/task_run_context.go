package hooks

import "github.com/compozy/agh/internal/network/participation"

// NetworkSpecSnapshot returns the immutable participation value carried by one task-run hook.
func (c TaskRunContext) NetworkSpecSnapshot() participation.Spec {
	if c.ResolvedNetworkParticipation == nil {
		return participation.LocalSpec()
	}
	return *c.ResolvedNetworkParticipation
}

// NetworkSpecSnapshot returns the immutable participation value carried by one task-level hook.
func (c TaskContext) NetworkSpecSnapshot() participation.Spec {
	if c.ResolvedNetworkParticipation == nil {
		return participation.LocalSpec()
	}
	return *c.ResolvedNetworkParticipation
}
