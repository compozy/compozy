package hooks

import "context"

// DispatchSessionMessagePersisted runs the session.message_persisted hook dispatch.
func (h *Hooks) DispatchSessionMessagePersisted(
	ctx context.Context,
	payload SessionMessagePersistedPayload,
) (SessionMessagePersistedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionMessagePersisted,
		payload,
		dispatchConfig[SessionMessagePersistedPayload, AuthoredContextObservationPatch]{
			match: matchSessionMessagePersisted,
			apply: applyNoop[SessionMessagePersistedPayload, AuthoredContextObservationPatch],
		},
	)
}

// DispatchTurnStart runs the turn.start hook pipeline.
func (h *Hooks) DispatchTurnStart(ctx context.Context, payload TurnStartPayload) (TurnStartPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTurnStart,
		payload,
		dispatchConfig[TurnStartPayload, TurnStartPatch]{
			match:  matchTurn,
			apply:  applyNoop[TurnStartPayload, TurnStartPatch],
			denied: turnPatchDenied,
		},
	)
}

// DispatchTurnEnd runs the turn.end hook pipeline.
func (h *Hooks) DispatchTurnEnd(ctx context.Context, payload TurnEndPayload) (TurnEndPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTurnEnd,
		payload,
		dispatchConfig[TurnEndPayload, TurnEndPatch]{
			match:  matchTurn,
			apply:  applyNoop[TurnEndPayload, TurnEndPatch],
			denied: turnPatchDenied,
		},
	)
}

// DispatchMessageStart runs the message.start hook pipeline.
func (h *Hooks) DispatchMessageStart(ctx context.Context, payload MessageStartPayload) (MessageStartPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookMessageStart,
		payload,
		dispatchConfig[MessageStartPayload, MessageStartPatch]{
			match:  matchMessage,
			apply:  applyMessagePatch,
			denied: messagePatchDenied,
		},
	)
}

// DispatchMessageDelta runs the message.delta hook dispatch.
func (h *Hooks) DispatchMessageDelta(ctx context.Context, payload MessageDeltaPayload) (MessageDeltaPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookMessageDelta,
		payload,
		dispatchConfig[MessageDeltaPayload, MessageDeltaPatch]{
			match:  matchMessage,
			apply:  applyMessagePatch,
			denied: messagePatchDenied,
		},
	)
}

// DispatchMessageEnd runs the message.end hook pipeline.
func (h *Hooks) DispatchMessageEnd(ctx context.Context, payload MessageEndPayload) (MessageEndPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookMessageEnd,
		payload,
		dispatchConfig[MessageEndPayload, MessageEndPatch]{
			match:  matchMessage,
			apply:  applyMessagePatch,
			denied: messagePatchDenied,
		},
	)
}
