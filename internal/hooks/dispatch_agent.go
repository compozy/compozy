package hooks

import "context"

// DispatchAgentPreStart runs the agent.pre_start hook pipeline.
func (h *Hooks) DispatchAgentPreStart(ctx context.Context, payload AgentPreStartPayload) (AgentPreStartPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentPreStart,
		payload,
		dispatchConfig[AgentPreStartPayload, AgentStartPatch]{
			match:  matchAgentPreStart,
			apply:  applyAgentStartPatch,
			denied: agentStartPatchDenied,
			denyErr: func(_ AgentPreStartPayload, report dispatchReport) error {
				return hookDeniedError(HookAgentPreStart, report.DenyReason)
			},
		},
	)
}

// DispatchAgentSpawned runs the agent.spawned hook pipeline.
func (h *Hooks) DispatchAgentSpawned(ctx context.Context, payload AgentSpawnedPayload) (AgentSpawnedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentSpawned,
		payload,
		dispatchConfig[AgentSpawnedPayload, AgentSpawnedPatch]{
			match: matchAgentLifecycle,
			apply: applyNoop[AgentSpawnedPayload, AgentSpawnedPatch],
		},
	)
}

// DispatchAgentCrashed runs the agent.crashed hook pipeline.
func (h *Hooks) DispatchAgentCrashed(ctx context.Context, payload AgentCrashedPayload) (AgentCrashedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentCrashed,
		payload,
		dispatchConfig[AgentCrashedPayload, AgentCrashedPatch]{
			match: matchAgentLifecycle,
			apply: applyNoop[AgentCrashedPayload, AgentCrashedPatch],
		},
	)
}

// DispatchAgentStopped runs the agent.stopped hook pipeline.
func (h *Hooks) DispatchAgentStopped(ctx context.Context, payload AgentStoppedPayload) (AgentStoppedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentStopped,
		payload,
		dispatchConfig[AgentStoppedPayload, AgentStoppedPatch]{
			match: matchAgentLifecycle,
			apply: applyNoop[AgentStoppedPayload, AgentStoppedPatch],
		},
	)
}

// DispatchAgentSoulSnapshotResolved runs the agent.soul.snapshot.resolved hook dispatch.
func (h *Hooks) DispatchAgentSoulSnapshotResolved(
	ctx context.Context,
	payload AgentSoulSnapshotResolvedPayload,
) (AgentSoulSnapshotResolvedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentSoulSnapshotResolved,
		payload,
		dispatchConfig[AgentSoulSnapshotResolvedPayload, AuthoredContextObservationPatch]{
			match: matchAgentSoulSnapshotResolved,
			apply: applyNoop[AgentSoulSnapshotResolvedPayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchAgentSoulMutationAfter runs the agent.soul.mutation.after hook dispatch.
func (h *Hooks) DispatchAgentSoulMutationAfter(
	ctx context.Context,
	payload AgentSoulMutationAfterPayload,
) (AgentSoulMutationAfterPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentSoulMutationAfter,
		payload,
		dispatchConfig[AgentSoulMutationAfterPayload, AuthoredContextObservationPatch]{
			match: matchAgentSoulMutationAfter,
			apply: applyNoop[AgentSoulMutationAfterPayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchAgentHeartbeatPolicyResolved runs the agent.heartbeat.policy.resolved hook dispatch.
func (h *Hooks) DispatchAgentHeartbeatPolicyResolved(
	ctx context.Context,
	payload AgentHeartbeatPolicyResolvedPayload,
) (AgentHeartbeatPolicyResolvedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentHeartbeatPolicyResolved,
		payload,
		dispatchConfig[AgentHeartbeatPolicyResolvedPayload, AuthoredContextObservationPatch]{
			match: matchAgentHeartbeatPolicyResolved,
			apply: applyNoop[AgentHeartbeatPolicyResolvedPayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchAgentHeartbeatWakeBefore runs the agent.heartbeat.wake.before hook pipeline.
func (h *Hooks) DispatchAgentHeartbeatWakeBefore(
	ctx context.Context,
	payload AgentHeartbeatWakeBeforePayload,
) (AgentHeartbeatWakeBeforePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentHeartbeatWakeBefore,
		payload,
		dispatchConfig[AgentHeartbeatWakeBeforePayload, AuthoredContextObservationPatch]{
			match: matchAgentHeartbeatWakeBefore,
			apply: applyNoop[AgentHeartbeatWakeBeforePayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchAgentHeartbeatWakeAfter runs the agent.heartbeat.wake.after hook dispatch.
func (h *Hooks) DispatchAgentHeartbeatWakeAfter(
	ctx context.Context,
	payload AgentHeartbeatWakeAfterPayload,
) (AgentHeartbeatWakeAfterPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAgentHeartbeatWakeAfter,
		payload,
		dispatchConfig[AgentHeartbeatWakeAfterPayload, AuthoredContextObservationPatch]{
			match: matchAgentHeartbeatWakeAfter,
			apply: applyNoop[AgentHeartbeatWakeAfterPayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchSessionHealthUpdateAfter runs the session.health.update.after hook dispatch.
func (h *Hooks) DispatchSessionHealthUpdateAfter(
	ctx context.Context,
	payload SessionHealthUpdateAfterPayload,
) (SessionHealthUpdateAfterPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionHealthUpdateAfter,
		payload,
		dispatchConfig[SessionHealthUpdateAfterPayload, AuthoredContextObservationPatch]{
			match: matchSessionHealthUpdateAfter,
			apply: applyNoop[SessionHealthUpdateAfterPayload, AuthoredContextObservationPatch],
		},
	)
}
