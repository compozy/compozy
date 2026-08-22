package hooks

import "context"

func (h *Hooks) DispatchSessionRuntimeRecoveryStarted(
	ctx context.Context,
	payload SessionRuntimeRecoveryStartedPayload,
) (SessionRuntimeRecoveryStartedPayload, error) {
	return h.dispatchSessionRuntimeRecovery(ctx, HookSessionRuntimeRecoveryStarted, payload)
}

func (h *Hooks) DispatchSessionRuntimeRecoverySucceeded(
	ctx context.Context,
	payload SessionRuntimeRecoverySucceededPayload,
) (SessionRuntimeRecoverySucceededPayload, error) {
	return h.dispatchSessionRuntimeRecovery(ctx, HookSessionRuntimeRecoverySucceeded, payload)
}

func (h *Hooks) DispatchSessionRuntimeRecoveryExhausted(
	ctx context.Context,
	payload SessionRuntimeRecoveryExhaustedPayload,
) (SessionRuntimeRecoveryExhaustedPayload, error) {
	return h.dispatchSessionRuntimeRecovery(ctx, HookSessionRuntimeRecoveryExhausted, payload)
}

func (h *Hooks) dispatchSessionRuntimeRecovery(
	ctx context.Context,
	event HookEvent,
	payload SessionRuntimeRecoveryPayload,
) (SessionRuntimeRecoveryPayload, error) {
	return executeDispatch(
		ctx,
		h,
		event,
		payload,
		dispatchConfig[SessionRuntimeRecoveryPayload, AuthoredContextObservationPatch]{
			match: matchSessionRuntimeRecovery,
			apply: applyNoop[SessionRuntimeRecoveryPayload, AuthoredContextObservationPatch],
		},
	)
}
