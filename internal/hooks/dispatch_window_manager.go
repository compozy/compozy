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

// DispatchWindowManagerWindowOpened dispatches window_manager.window.opened.
func (h *Hooks) DispatchWindowManagerWindowOpened(
	ctx context.Context,
	payload WindowManagerWindowOpenedPayload,
) (WindowManagerWindowOpenedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerWindowOpened, payload)
}

// DispatchWindowManagerWindowClosed dispatches window_manager.window.closed.
func (h *Hooks) DispatchWindowManagerWindowClosed(
	ctx context.Context,
	payload WindowManagerWindowClosedPayload,
) (WindowManagerWindowClosedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerWindowClosed, payload)
}

// DispatchWindowManagerStackGrouped is a load-only compatibility no-op until v0.8.0.
func (h *Hooks) DispatchWindowManagerStackGrouped(
	_ context.Context,
	payload WindowManagerStackGroupedPayload,
) (WindowManagerStackGroupedPayload, error) {
	return payload, nil
}

// DispatchWindowManagerStackUngrouped is a load-only compatibility no-op until v0.8.0.
func (h *Hooks) DispatchWindowManagerStackUngrouped(
	_ context.Context,
	payload WindowManagerStackUngroupedPayload,
) (WindowManagerStackUngroupedPayload, error) {
	return payload, nil
}

// DispatchWindowManagerStackActivated dispatches window_manager.stack.activated.
func (h *Hooks) DispatchWindowManagerStackActivated(
	ctx context.Context,
	payload WindowManagerStackActivatedPayload,
) (WindowManagerStackActivatedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerStackActivated, payload)
}

func executeWindowManagerDispatch(
	ctx context.Context,
	hooks *Hooks,
	event HookEvent,
	payload WindowManagerPayload,
) (WindowManagerPayload, error) {
	return hooks.executeDispatch(ctx, event,
		payload,
		dispatchConfig[WindowManagerPayload, WindowManagerObservationPatch]{
			match: matchWindowManager,
			apply: applyNoop[WindowManagerPayload, WindowManagerObservationPatch],
		},
	)
}

func (h *Hooks) DispatchWindowManagerTabSplit(
	ctx context.Context,
	payload WindowManagerTabSplitPayload,
) (WindowManagerTabSplitPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerTabSplit, payload)
}

func (h *Hooks) DispatchWindowManagerTabUnsplit(
	ctx context.Context,
	payload WindowManagerTabUnsplitPayload,
) (WindowManagerTabUnsplitPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerTabUnsplit, payload)
}

func (h *Hooks) DispatchWindowManagerTabActivated(
	ctx context.Context,
	payload WindowManagerTabActivatedPayload,
) (WindowManagerTabActivatedPayload, error) {
	return executeWindowManagerDispatch(ctx, h, HookWindowManagerTabActivated, payload)
}
