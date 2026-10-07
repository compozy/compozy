package hooks

import "context"

// DispatchLoopStarted runs the loop.started hook dispatch.
func (h *Hooks) DispatchLoopStarted(
	ctx context.Context,
	payload LoopStartedPayload,
) (LoopStartedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopStarted,
		payload,
		dispatchConfig[LoopStartedPayload, LoopObservationPatch]{
			match: matchLoopLifecycle,
			apply: applyNoop[LoopStartedPayload, LoopObservationPatch],
		},
	)
}

// DispatchLoopGenerationPre runs the loop.generation.pre hook pipeline.
func (h *Hooks) DispatchLoopGenerationPre(
	ctx context.Context,
	payload LoopGenerationPrePayload,
) (LoopGenerationPrePayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopGenerationPre,
		payload,
		dispatchConfig[LoopGenerationPrePayload, LoopGenerationPrePatch]{
			match:  matchLoopGeneration,
			apply:  applyLoopGenerationPrePatch,
			denied: loopControlPatchDenied,
			denyErr: func(_ LoopGenerationPrePayload, report dispatchReport) error {
				return hookDeniedError(HookLoopGenerationPre, report.DenyReason)
			},
		},
	)
}

// DispatchLoopGenerationPost runs the loop.generation.post hook dispatch.
func (h *Hooks) DispatchLoopGenerationPost(
	ctx context.Context,
	payload LoopGenerationPostPayload,
) (LoopGenerationPostPayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopGenerationPost,
		payload,
		dispatchConfig[LoopGenerationPostPayload, LoopObservationPatch]{
			match: matchLoopGeneration,
			apply: applyNoop[LoopGenerationPostPayload, LoopObservationPatch],
		},
	)
}

// DispatchLoopGatePre runs the loop.gate.pre hook pipeline.
func (h *Hooks) DispatchLoopGatePre(
	ctx context.Context,
	payload LoopGatePrePayload,
) (LoopGatePrePayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopGatePre,
		payload,
		dispatchConfig[LoopGatePrePayload, LoopGatePrePatch]{
			match:  matchLoopGate,
			apply:  applyLoopGatePrePatch,
			denied: loopControlPatchDenied,
			denyErr: func(_ LoopGatePrePayload, report dispatchReport) error {
				return hookDeniedError(HookLoopGatePre, report.DenyReason)
			},
		},
	)
}

// DispatchLoopGatePost runs the loop.gate.post hook dispatch.
func (h *Hooks) DispatchLoopGatePost(
	ctx context.Context,
	payload LoopGatePostPayload,
) (LoopGatePostPayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopGatePost,
		payload,
		dispatchConfig[LoopGatePostPayload, LoopObservationPatch]{
			match: matchLoopGate,
			apply: applyNoop[LoopGatePostPayload, LoopObservationPatch],
		},
	)
}

// DispatchLoopNodeTerminal runs the loop.node.terminal hook dispatch.
func (h *Hooks) DispatchLoopNodeTerminal(
	ctx context.Context,
	payload LoopNodeTerminalPayload,
) (LoopNodeTerminalPayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopNodeTerminal,
		payload,
		dispatchConfig[LoopNodeTerminalPayload, LoopObservationPatch]{
			match: matchLoopNodeTerminal,
			apply: applyNoop[LoopNodeTerminalPayload, LoopObservationPatch],
		},
	)
}

// DispatchLoopTerminal runs the loop.terminal hook dispatch.
func (h *Hooks) DispatchLoopTerminal(
	ctx context.Context,
	payload LoopTerminalPayload,
) (LoopTerminalPayload, error) {
	return h.executeDispatch(
		ctx,
		HookLoopTerminal,
		payload,
		dispatchConfig[LoopTerminalPayload, LoopObservationPatch]{
			match: matchLoopLifecycle,
			apply: applyNoop[LoopTerminalPayload, LoopObservationPatch],
		},
	)
}

// DispatchSpawnPreCreate runs the spawn.pre_create hook pipeline.
func (h *Hooks) DispatchSpawnPreCreate(
	ctx context.Context,
	payload SpawnPreCreatePayload,
) (SpawnPreCreatePayload, error) {
	return h.executeDispatch(
		ctx,
		HookSpawnPreCreate,
		payload,
		dispatchConfig[SpawnPreCreatePayload, SpawnCreatePatch]{
			match:  matchSpawnPreCreate,
			apply:  applySpawnCreatePatch,
			denied: spawnCreatePatchDenied,
			denyErr: func(_ SpawnPreCreatePayload, report dispatchReport) error {
				return hookDeniedError(HookSpawnPreCreate, report.DenyReason)
			},
			guard: guardSpawnCreatePatch,
		},
	)
}

// DispatchSpawnCreated runs the spawn.created hook dispatch.
func (h *Hooks) DispatchSpawnCreated(
	ctx context.Context,
	payload SpawnCreatedPayload,
) (SpawnCreatedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookSpawnCreated,
		payload,
		dispatchConfig[SpawnCreatedPayload, SpawnObservationPatch]{
			match: matchSpawnLifecycle,
			apply: applyNoop[SpawnCreatedPayload, SpawnObservationPatch],
		},
	)
}

// DispatchSpawnParentStopped runs the spawn.parent_stopped hook dispatch.
func (h *Hooks) DispatchSpawnParentStopped(
	ctx context.Context,
	payload SpawnParentStoppedPayload,
) (SpawnParentStoppedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookSpawnParentStopped,
		payload,
		dispatchConfig[SpawnParentStoppedPayload, SpawnObservationPatch]{
			match: matchSpawnLifecycle,
			apply: applyNoop[SpawnParentStoppedPayload, SpawnObservationPatch],
		},
	)
}

// DispatchSpawnTTLExpired runs the spawn.ttl_expired hook dispatch.
func (h *Hooks) DispatchSpawnTTLExpired(
	ctx context.Context,
	payload SpawnTTLExpiredPayload,
) (SpawnTTLExpiredPayload, error) {
	return h.executeDispatch(
		ctx,
		HookSpawnTTLExpired,
		payload,
		dispatchConfig[SpawnTTLExpiredPayload, SpawnObservationPatch]{
			match: matchSpawnLifecycle,
			apply: applyNoop[SpawnTTLExpiredPayload, SpawnObservationPatch],
		},
	)
}

// DispatchSpawnReaped runs the spawn.reaped hook dispatch.
func (h *Hooks) DispatchSpawnReaped(
	ctx context.Context,
	payload SpawnReapedPayload,
) (SpawnReapedPayload, error) {
	return h.executeDispatch(
		ctx,
		HookSpawnReaped,
		payload,
		dispatchConfig[SpawnReapedPayload, SpawnObservationPatch]{
			match: matchSpawnLifecycle,
			apply: applyNoop[SpawnReapedPayload, SpawnObservationPatch],
		},
	)
}
