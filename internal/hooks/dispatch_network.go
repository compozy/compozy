package hooks

import "context"

// DispatchNetworkPeerJoined runs the network.peer.joined hook dispatch.
func (h *Hooks) DispatchNetworkPeerJoined(
	ctx context.Context,
	payload NetworkPeerJoinedPayload,
) (NetworkPeerJoinedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkPeerJoined,
		payload,
		dispatchConfig[NetworkPeerJoinedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkPeerJoinedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkPeerLeft runs the network.peer.left hook dispatch.
func (h *Hooks) DispatchNetworkPeerLeft(
	ctx context.Context,
	payload NetworkPeerLeftPayload,
) (NetworkPeerLeftPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkPeerLeft,
		payload,
		dispatchConfig[NetworkPeerLeftPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkPeerLeftPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkThreadOpened runs the network.thread.opened hook dispatch.
func (h *Hooks) DispatchNetworkThreadOpened(
	ctx context.Context,
	payload NetworkThreadOpenedPayload,
) (NetworkThreadOpenedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkThreadOpened,
		payload,
		dispatchConfig[NetworkThreadOpenedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkThreadOpenedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkDirectRoomOpened runs the network.direct_room.opened hook dispatch.
func (h *Hooks) DispatchNetworkDirectRoomOpened(
	ctx context.Context,
	payload NetworkDirectRoomOpenedPayload,
) (NetworkDirectRoomOpenedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkDirectRoomOpened,
		payload,
		dispatchConfig[NetworkDirectRoomOpenedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkDirectRoomOpenedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkMessagePersisted runs the network.message.persisted hook dispatch.
func (h *Hooks) DispatchNetworkMessagePersisted(
	ctx context.Context,
	payload NetworkMessagePersistedPayload,
) (NetworkMessagePersistedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkMessagePersisted,
		payload,
		dispatchConfig[NetworkMessagePersistedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkMessagePersistedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkWorkOpened runs the network.work.opened hook dispatch.
func (h *Hooks) DispatchNetworkWorkOpened(
	ctx context.Context,
	payload NetworkWorkOpenedPayload,
) (NetworkWorkOpenedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkWorkOpened,
		payload,
		dispatchConfig[NetworkWorkOpenedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkWorkOpenedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkWorkTransitioned runs the network.work.transitioned hook dispatch.
func (h *Hooks) DispatchNetworkWorkTransitioned(
	ctx context.Context,
	payload NetworkWorkTransitionedPayload,
) (NetworkWorkTransitionedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkWorkTransitioned,
		payload,
		dispatchConfig[NetworkWorkTransitionedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkWorkTransitionedPayload, NetworkObservationPatch],
		},
	)
}

// DispatchNetworkWorkClosed runs the network.work.closed hook dispatch.
func (h *Hooks) DispatchNetworkWorkClosed(
	ctx context.Context,
	payload NetworkWorkClosedPayload,
) (NetworkWorkClosedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkWorkClosed,
		payload,
		dispatchConfig[NetworkWorkClosedPayload, NetworkObservationPatch]{
			match: matchNetwork,
			apply: applyNoop[NetworkWorkClosedPayload, NetworkObservationPatch],
		},
	)
}
