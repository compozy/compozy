package hooks

import "context"

func (h *Hooks) dispatchCall(
	ctx context.Context,
	event HookEvent,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return executeDispatch(ctx, h, event, payload, dispatchConfig[CallObservationPayload, CallObservationPatch]{
		match: matchCall,
		apply: applyNoop[CallObservationPayload, CallObservationPatch],
	})
}

// DispatchCallCreated dispatches HookCallCreated after durable admission.
func (h *Hooks) DispatchCallCreated(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallCreated, payload)
}

// DispatchCallStateChanged dispatches HookCallStateChanged after a durable state transition.
func (h *Hooks) DispatchCallStateChanged(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallStateChanged, payload)
}

// DispatchCallSettled dispatches HookCallSettled after terminal settlement.
func (h *Hooks) DispatchCallSettled(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallSettled, payload)
}

// DispatchCallCanceled dispatches HookCallCanceled after cancellation wins its fence.
func (h *Hooks) DispatchCallCanceled(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallCanceled, payload)
}

// DispatchCallPublished dispatches HookCallPublished after evidence publication.
func (h *Hooks) DispatchCallPublished(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallPublished, payload)
}

// DispatchCallMessageSent dispatches HookCallMessageSent after mailbox admission.
func (h *Hooks) DispatchCallMessageSent(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageSent, payload)
}

// DispatchCallMessageDelivered dispatches HookCallMessageDelivered after boundary delivery.
func (h *Hooks) DispatchCallMessageDelivered(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageDelivered, payload)
}

// DispatchCallMessageRejected dispatches HookCallMessageRejected after a sender-side brake rejects admission.
func (h *Hooks) DispatchCallMessageRejected(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallMessageRejected, payload)
}

// DispatchCallRevived dispatches HookCallRevived after a parked child resumes.
func (h *Hooks) DispatchCallRevived(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallRevived, payload)
}

// DispatchCallReaped dispatches HookCallReaped after a parked child is reaped.
func (h *Hooks) DispatchCallReaped(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallReaped, payload)
}

// DispatchCallSubtreeDrained dispatches HookCallSubtreeDrained after governed-tree cleanup.
func (h *Hooks) DispatchCallSubtreeDrained(
	ctx context.Context,
	payload CallObservationPayload,
) (CallObservationPayload, error) {
	return h.dispatchCall(ctx, HookCallSubtreeDrained, payload)
}
