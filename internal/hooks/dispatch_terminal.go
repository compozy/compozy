package hooks

import "context"

type terminalHookPayload interface {
	HookProfileID() string
	hookTerminalContext() TerminalContext
}

func (h *Hooks) dispatchTerminal[P terminalHookPayload](
	ctx context.Context,
	event HookEvent,
	payload P,
) (P, error) {
	return h.executeDispatch(ctx, event, payload, dispatchConfig[P, TerminalObservationPatch]{
		match: func(matcher HookMatcher, candidate P) bool {
			return matchStringField(matcher.WorkspaceID, candidate.hookTerminalContext().WorkspaceID)
		},
		apply: applyNoop[P, TerminalObservationPatch],
	})
}

func (h *Hooks) DispatchTerminalOpened(ctx context.Context, p TerminalOpenedPayload) (TerminalOpenedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalOpened, p)
}

func (h *Hooks) DispatchTerminalClosed(ctx context.Context, p TerminalClosedPayload) (TerminalClosedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalClosed, p)
}

func (h *Hooks) DispatchTerminalCommandStarted(
	ctx context.Context,
	p TerminalCommandStartedPayload,
) (TerminalCommandStartedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalCommandStarted, p)
}

func (h *Hooks) DispatchTerminalCommandFinished(
	ctx context.Context,
	p TerminalCommandFinishedPayload,
) (TerminalCommandFinishedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalCommandFinished, p)
}

func (h *Hooks) DispatchTerminalInputRequested(
	ctx context.Context,
	p TerminalInputRequestedPayload,
) (TerminalInputRequestedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalInputRequested, p)
}

func (h *Hooks) DispatchTerminalInputProvided(
	ctx context.Context,
	p TerminalInputProvidedPayload,
) (TerminalInputProvidedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalInputProvided, p)
}

func (h *Hooks) DispatchTerminalRecordingStarted(
	ctx context.Context,
	p TerminalRecordingStartedPayload,
) (TerminalRecordingStartedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalRecordingStarted, p)
}

func (h *Hooks) DispatchTerminalRecordingStopped(
	ctx context.Context,
	p TerminalRecordingStoppedPayload,
) (TerminalRecordingStoppedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalRecordingStopped, p)
}

func (h *Hooks) DispatchTerminalSubscriberEvicted(
	ctx context.Context,
	p TerminalSubscriberEvictedPayload,
) (TerminalSubscriberEvictedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalSubscriberEvicted, p)
}

func (h *Hooks) DispatchTerminalLimitRejected(
	ctx context.Context,
	p TerminalLimitRejectedPayload,
) (TerminalLimitRejectedPayload, error) {
	return h.dispatchTerminal(ctx, HookTerminalLimitRejected, p)
}
