package hooks

import "context"

// DispatchContextPreCompact runs the context.pre_compact hook pipeline.
func (h *Hooks) DispatchContextPreCompact(
	ctx context.Context,
	payload ContextPreCompactPayload,
) (ContextPreCompactPayload, error) {
	return h.executeDispatch(
		ctx,
		HookContextPreCompact,
		payload,
		dispatchConfig[ContextPreCompactPayload, ContextPreCompactPatch]{
			match:  matchContextCompact,
			apply:  applyContextCompactionPatch,
			denied: contextCompactionPatchDenied,
		},
	)
}

// DispatchContextPostCompact runs the context.post_compact hook pipeline.
func (h *Hooks) DispatchContextPostCompact(
	ctx context.Context,
	payload ContextPostCompactPayload,
) (ContextPostCompactPayload, error) {
	return h.executeDispatch(
		ctx,
		HookContextPostCompact,
		payload,
		dispatchConfig[ContextPostCompactPayload, ContextPostCompactPatch]{
			match:  matchContextCompact,
			apply:  applyContextCompactionPatch,
			denied: contextCompactionPatchDenied,
		},
	)
}

// DispatchCoordinatorPreSpawn runs the coordinator.pre_spawn hook pipeline.
func (h *Hooks) DispatchCoordinatorPreSpawn(
	ctx context.Context,
	payload CoordinatorPreSpawnPayload,
) (CoordinatorPreSpawnPayload, error) {
	return h.executeDispatch(
		ctx,
		HookCoordinatorPreSpawn,
		payload,
		dispatchConfig[CoordinatorPreSpawnPayload, CoordinatorSpawnPatch]{
			match:  matchCoordinatorPreSpawn,
			apply:  applyCoordinatorSpawnPatch,
			denied: coordinatorSpawnPatchDenied,
			denyErr: func(_ CoordinatorPreSpawnPayload, report dispatchReport) error {
				return hookDeniedError(HookCoordinatorPreSpawn, report.DenyReason)
			},
		},
	)
}

// DispatchCoordinatorSpawned runs the coordinator.spawned hook dispatch.
func (h *Hooks) DispatchCoordinatorSpawned(
	ctx context.Context,
	payload CoordinatorSpawnedPayload,
) (CoordinatorSpawnedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookCoordinatorSpawned,
		payload,
		dispatchConfig[CoordinatorSpawnedPayload, CoordinatorObservationPatch]{
			match: matchCoordinatorLifecycle,
			apply: applyNoop[CoordinatorSpawnedPayload, CoordinatorObservationPatch],
		},
	)
}

// DispatchCoordinatorDecision runs the coordinator.decision hook dispatch.
func (h *Hooks) DispatchCoordinatorDecision(
	ctx context.Context,
	payload CoordinatorDecisionPayload,
) (CoordinatorDecisionPayload, error) {
	return h.executeDispatch(
		ctx,
		HookCoordinatorDecision,
		payload,
		dispatchConfig[CoordinatorDecisionPayload, CoordinatorObservationPatch]{
			match: matchCoordinatorLifecycle,
			apply: applyNoop[CoordinatorDecisionPayload, CoordinatorObservationPatch],
		},
	)
}

// DispatchCoordinatorStopped runs the coordinator.stopped hook dispatch.
func (h *Hooks) DispatchCoordinatorStopped(
	ctx context.Context,
	payload CoordinatorStoppedPayload,
) (CoordinatorStoppedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookCoordinatorStopped,
		payload,
		dispatchConfig[CoordinatorStoppedPayload, CoordinatorObservationPatch]{
			match: matchCoordinatorLifecycle,
			apply: applyNoop[CoordinatorStoppedPayload, CoordinatorObservationPatch],
		},
	)
}

// DispatchCoordinatorFailed runs the coordinator.failed hook dispatch.
func (h *Hooks) DispatchCoordinatorFailed(
	ctx context.Context,
	payload CoordinatorFailedPayload,
) (CoordinatorFailedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookCoordinatorFailed,
		payload,
		dispatchConfig[CoordinatorFailedPayload, CoordinatorObservationPatch]{
			match: matchCoordinatorLifecycle,
			apply: applyNoop[CoordinatorFailedPayload, CoordinatorObservationPatch],
		},
	)
}
