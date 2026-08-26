package hooks

import "context"

func (h *Hooks) dispatchCall(ctx context.Context, event HookEvent, payload CallPayload) (CallPayload, error) {
	return executeDispatch(ctx, h, event, payload, dispatchConfig[CallPayload, CallObservationPatch]{
		match: matchCall,
		apply: applyNoop[CallPayload, CallObservationPatch],
	})
}

// DispatchCallCreated dispatches HookCallCreated after durable admission.
func (h *Hooks) DispatchCallCreated(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallCreated, payload)
}

// DispatchCallSettled dispatches HookCallSettled after terminal settlement.
func (h *Hooks) DispatchCallSettled(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallSettled, payload)
}

// DispatchCallCanceled dispatches HookCallCanceled after cancellation wins its fence.
func (h *Hooks) DispatchCallCanceled(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallCanceled, payload)
}

// DispatchCallPublished dispatches HookCallPublished after evidence publication.
func (h *Hooks) DispatchCallPublished(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallPublished, payload)
}

// DispatchCallMessageSent dispatches HookCallMessageSent after mailbox admission.
func (h *Hooks) DispatchCallMessageSent(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageSent, payload)
}

// DispatchCallMessageDelivered dispatches HookCallMessageDelivered after boundary delivery.
func (h *Hooks) DispatchCallMessageDelivered(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageDelivered, payload)
}

// DispatchCallSubtreeDrained dispatches HookCallSubtreeDrained after governed-tree cleanup.
func (h *Hooks) DispatchCallSubtreeDrained(ctx context.Context, payload CallPayload) (CallPayload, error) {
	return h.dispatchCall(ctx, HookCallSubtreeDrained, payload)
}
