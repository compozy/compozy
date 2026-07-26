package session

import (
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
)

func networkPeerCapabilities(catalog *aghconfig.CapabilityCatalog) []NetworkPeerCapability {
	if catalog == nil || len(catalog.Capabilities) == 0 {
		return []NetworkPeerCapability{}
	}

	projected := make([]NetworkPeerCapability, 0, len(catalog.Capabilities))
	for _, capability := range catalog.Capabilities {
		projected = append(projected, cloneNetworkPeerCapability(NetworkPeerCapability{
			ID:                capability.ID,
			Summary:           capability.Summary,
			Outcome:           capability.Outcome,
			Version:           capability.Version,
			Digest:            capability.Digest,
			ContextNeeded:     capability.ContextNeeded,
			ArtifactsExpected: capability.ArtifactsExpected,
			ExecutionOutline:  capability.ExecutionOutline,
			Constraints:       capability.Constraints,
			Examples:          capability.Examples,
			Requirements:      capability.Requirements,
		}))
	}
	return projected
}

func cloneNetworkPeerCapabilities(capabilities []NetworkPeerCapability) []NetworkPeerCapability {
	if len(capabilities) == 0 {
		return []NetworkPeerCapability{}
	}

	cloned := make([]NetworkPeerCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		cloned = append(cloned, cloneNetworkPeerCapability(capability))
	}
	return cloned
}

func cloneNetworkPeerCapability(capability NetworkPeerCapability) NetworkPeerCapability {
	return NetworkPeerCapability{
		ID:                strings.TrimSpace(capability.ID),
		Summary:           strings.TrimSpace(capability.Summary),
		Outcome:           strings.TrimSpace(capability.Outcome),
		Version:           strings.TrimSpace(capability.Version),
		Digest:            strings.TrimSpace(capability.Digest),
		ContextNeeded:     cloneNetworkPeerCapabilityStrings(capability.ContextNeeded),
		ArtifactsExpected: cloneNetworkPeerCapabilityStrings(capability.ArtifactsExpected),
		ExecutionOutline:  cloneNetworkPeerCapabilityStrings(capability.ExecutionOutline),
		Constraints:       cloneNetworkPeerCapabilityStrings(capability.Constraints),
		Examples:          cloneNetworkPeerCapabilityStrings(capability.Examples),
		Requirements:      cloneNetworkPeerCapabilityStrings(capability.Requirements),
	}
}

func cloneNetworkPeerCapabilityStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}

	return cloned
}

func newNetworkPeerJoin(
	sessionID string,
	peerID string,
	workspaceID string,
	displayName string,
	channel string,
	ownerKey string,
	networkParticipation participation.Spec,
	capabilities []NetworkPeerCapability,
) NetworkPeerJoin {
	return NetworkPeerJoin{
		SessionID:            strings.TrimSpace(sessionID),
		PeerID:               strings.TrimSpace(peerID),
		WorkspaceID:          strings.TrimSpace(workspaceID),
		DisplayName:          strings.TrimSpace(displayName),
		Channel:              strings.TrimSpace(channel),
		OwnerKey:             strings.TrimSpace(ownerKey),
		NetworkParticipation: networkParticipation,
		Capabilities:         cloneNetworkPeerCapabilities(capabilities),
	}
}
