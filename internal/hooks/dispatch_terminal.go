package hooks

import "context"

type terminalHookPayload interface {
	HookProfileID() string
	hookTerminalContext() TerminalContext
}

func dispatchTerminal[P terminalHookPayload](
	ctx context.Context,
	hooks *Hooks,
	event HookEvent,
	payload P,
) (P, error) {
	return executeDispatch(ctx, hooks, event, payload, dispatchConfig[P, TerminalObservationPatch]{
		match: func(matcher HookMatcher, candidate P) bool {
			return matchStringField(matcher.WorkspaceID, candidate.hookTerminalContext().WorkspaceID)
		},
		apply: applyNoop[P, TerminalObservationPatch],
	})
}

func (h *Hooks) DispatchTerminalOpened(ctx context.Context, p TerminalOpenedPayload) (TerminalOpenedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalOpened, p)
}

func (h *Hooks) DispatchTerminalClosed(ctx context.Context, p TerminalClosedPayload) (TerminalClosedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalClosed, p)
}

func (h *Hooks) DispatchTerminalCommandStarted(
	ctx context.Context,
	p TerminalCommandStartedPayload,
) (TerminalCommandStartedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalCommandStarted, p)
}

func (h *Hooks) DispatchTerminalCommandFinished(
	ctx context.Context,
	p TerminalCommandFinishedPayload,
) (TerminalCommandFinishedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalCommandFinished, p)
}

func (h *Hooks) DispatchTerminalInputRequested(
	ctx context.Context,
	p TerminalInputRequestedPayload,
) (TerminalInputRequestedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalInputRequested, p)
}

func (h *Hooks) DispatchTerminalInputProvided(
	ctx context.Context,
	p TerminalInputProvidedPayload,
) (TerminalInputProvidedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalInputProvided, p)
}

func (h *Hooks) DispatchTerminalRecordingStarted(
	ctx context.Context,
	p TerminalRecordingStartedPayload,
) (TerminalRecordingStartedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalRecordingStarted, p)
}

func (h *Hooks) DispatchTerminalRecordingStopped(
	ctx context.Context,
	p TerminalRecordingStoppedPayload,
) (TerminalRecordingStoppedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalRecordingStopped, p)
}

func (h *Hooks) DispatchTerminalSubscriberEvicted(
	ctx context.Context,
	p TerminalSubscriberEvictedPayload,
) (TerminalSubscriberEvictedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalSubscriberEvicted, p)
}

func (h *Hooks) DispatchTerminalLimitRejected(
	ctx context.Context,
	p TerminalLimitRejectedPayload,
) (TerminalLimitRejectedPayload, error) {
	return dispatchTerminal(ctx, h, HookTerminalLimitRejected, p)
}
