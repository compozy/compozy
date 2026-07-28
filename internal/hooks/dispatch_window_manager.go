package hooks

import "context"

// DispatchWindowManagerLayoutApplied dispatches window_manager.layout.applied.
func (h *Hooks) DispatchWindowManagerLayoutApplied(
	ctx context.Context,
	payload WindowManagerLayoutAppliedPayload,
) (WindowManagerLayoutAppliedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerLayoutApplied, payload)
}

// DispatchWindowManagerDesktopCreated dispatches window_manager.desktop.created.
func (h *Hooks) DispatchWindowManagerDesktopCreated(
	ctx context.Context,
	payload WindowManagerDesktopCreatedPayload,
) (WindowManagerDesktopCreatedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerDesktopCreated, payload)
}

// DispatchWindowManagerDesktopDeleted dispatches window_manager.desktop.deleted.
func (h *Hooks) DispatchWindowManagerDesktopDeleted(
	ctx context.Context,
	payload WindowManagerDesktopDeletedPayload,
) (WindowManagerDesktopDeletedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerDesktopDeleted, payload)
}

// DispatchWindowManagerWindowMoved dispatches window_manager.window.moved.
func (h *Hooks) DispatchWindowManagerWindowMoved(
	ctx context.Context,
	payload WindowManagerWindowMovedPayload,
) (WindowManagerWindowMovedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerWindowMoved, payload)
}

func executeWindowManagerDispatch(
	ctx context.Context,
	hooks *Hooks,
	event HookEvent,
	payload WindowManagerPayload,
) (WindowManagerPayload, error) {
	return executeDispatch(
		ctx,
		hooks,
		event,
		payload,
		dispatchConfig[WindowManagerPayload, WindowManagerObservationPatch]{
			match: matchWindowManager,
			apply: applyNoop[WindowManagerPayload, WindowManagerObservationPatch],
		},
	)
}
