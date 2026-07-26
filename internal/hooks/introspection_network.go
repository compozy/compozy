package hooks

import "maps"

const introspectionNetworkObservationPatchValue = "NetworkObservationPatch"

func networkHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookNetworkPeerJoined: {
			Event:         HookNetworkPeerJoined,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkPeerJoinedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkPeerLeft: {
			Event:         HookNetworkPeerLeft,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkPeerLeftPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkThreadOpened: {
			Event:         HookNetworkThreadOpened,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkThreadOpenedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkDirectRoomOpened: {
			Event:         HookNetworkDirectRoomOpened,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkDirectRoomOpenedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkMessagePersisted: {
			Event:         HookNetworkMessagePersisted,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkMessagePersistedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkWorkOpened: {
			Event:         HookNetworkWorkOpened,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkWorkOpenedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkWorkTransitioned: {
			Event:         HookNetworkWorkTransitioned,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkWorkTransitionedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkWorkClosed: {
			Event:         HookNetworkWorkClosed,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkWorkClosedPayload",
			PatchSchema:   introspectionNetworkObservationPatchValue,
		},
		HookNetworkParticipationPreResolve: {
			Event:         HookNetworkParticipationPreResolve,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  true,
			PayloadSchema: "NetworkParticipationPreResolvePayload",
			PatchSchema:   "NetworkParticipationPreResolvePatch",
		},
		HookNetworkParticipationResolved: {
			Event:         HookNetworkParticipationResolved,
			Family:        HookEventFamilyNetwork,
			SyncEligible:  false,
			PayloadSchema: "NetworkParticipationResolvedPayload",
			PatchSchema:   "NetworkParticipationResolvedPatch",
		},
	}
}

func mergeHookEventDescriptors(
	base map[HookEvent]EventDescriptor,
	overlays ...map[HookEvent]EventDescriptor,
) map[HookEvent]EventDescriptor {
	for _, overlay := range overlays {
		maps.Copy(base, overlay)
	}
	return base
}
