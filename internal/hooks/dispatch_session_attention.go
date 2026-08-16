package hooks

import "context"

// DispatchSessionAttentionChanged dispatches session.attention.changed after its canonical commit.
func (h *Hooks) DispatchSessionAttentionChanged(
	ctx context.Context,
	payload SessionAttentionChangedPayload,
) (SessionAttentionChangedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionAttentionChanged,
		payload,
		dispatchConfig[SessionAttentionChangedPayload, SessionAttentionObservationPatch]{
			match: matchSessionAttention,
			apply: applyNoop[SessionAttentionChangedPayload, SessionAttentionObservationPatch],
		},
	)
}
