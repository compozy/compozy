package hooks

import "context"

func (h *Hooks) dispatchCall(ctx context.Context, event HookEvent, payload CallPayload) (CallPayload, error) {
	return executeDispatch(ctx, h, event, payload, dispatchConfig[CallPayload, CallObservationPatch]{
		match: matchCall,
		apply: applyNoop[CallPayload, CallObservationPatch],
	})
}

func (h *Hooks) DispatchCallCreated(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallCreated, payload)
}
func (h *Hooks) DispatchCallSettled(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallSettled, payload)
}
func (h *Hooks) DispatchCallCanceled(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallCanceled, payload)
}
func (h *Hooks) DispatchCallPublished(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallPublished, payload)
}
func (h *Hooks) DispatchCallMessageSent(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageSent, payload)
}
func (h *Hooks) DispatchCallMessageDelivered(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageDelivered, payload)
}
func (h *Hooks) DispatchCallSubtreeDrained(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallSubtreeDrained, payload)
}
