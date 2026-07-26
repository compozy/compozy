package hooks

import "maps"

const (
	HookNetworkThreadOpened            HookEvent = "network.thread.opened"
	HookNetworkDirectRoomOpened        HookEvent = "network.direct_room.opened"
	HookNetworkMessagePersisted        HookEvent = "network.message.persisted"
	HookNetworkWorkOpened              HookEvent = "network.work.opened"
	HookNetworkWorkTransitioned        HookEvent = "network.work.transitioned"
	HookNetworkWorkClosed              HookEvent = "network.work.closed"
	HookNetworkPeerJoined              HookEvent = "network.peer.joined"
	HookNetworkPeerLeft                HookEvent = "network.peer.left"
	HookNetworkParticipationPreResolve HookEvent = "network.participation.pre_resolve"
	HookNetworkParticipationResolved   HookEvent = "network.participation.resolved"
)

func networkHookEventSpecs() map[HookEvent]hookEventSpec {
	return map[HookEvent]hookEventSpec{
		HookNetworkThreadOpened: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkDirectRoomOpened: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkMessagePersisted: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkWorkOpened: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkWorkTransitioned: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkWorkClosed: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkPeerJoined: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkPeerLeft: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		HookNetworkParticipationPreResolve: {
			family:       HookEventFamilyNetwork,
			syncEligible: true,
		},
		HookNetworkParticipationResolved: {
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
	}
}

func networkHookEvents() []HookEvent {
	return []HookEvent{
		HookNetworkThreadOpened,
		HookNetworkDirectRoomOpened,
		HookNetworkMessagePersisted,
		HookNetworkWorkOpened,
		HookNetworkWorkTransitioned,
		HookNetworkWorkClosed,
		HookNetworkPeerJoined,
		HookNetworkPeerLeft,
		HookNetworkParticipationPreResolve,
		HookNetworkParticipationResolved,
	}
}

func mergeHookEventSpecs(
	base map[HookEvent]hookEventSpec,
	overlays ...map[HookEvent]hookEventSpec,
) map[HookEvent]hookEventSpec {
	for _, overlay := range overlays {
		maps.Copy(base, overlay)
	}
	return base
}
