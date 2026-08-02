package hooks

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

func networkHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{
		{event: HookNetworkThreadOpened,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkDirectRoomOpened,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkMessagePersisted,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkWorkOpened,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkWorkTransitioned,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkWorkClosed,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkPeerJoined,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkPeerLeft,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
		{event: HookNetworkParticipationPreResolve,
			family:       HookEventFamilyNetwork,
			syncEligible: true,
		},
		{event: HookNetworkParticipationResolved,
			family:       HookEventFamilyNetwork,
			syncEligible: false,
		},
	}
}
