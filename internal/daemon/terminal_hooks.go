package daemon

import (
	"context"
	"log/slog"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func attachTerminalHookBridge(terminals *terminalpkg.Service, hooks *hookspkg.Hooks, logger *slog.Logger) {
	if terminals == nil || hooks == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	terminals.Observe(func(ctx context.Context, event terminalpkg.Event) {
		if err := dispatchTerminalHookEvent(ctx, hooks, event); err != nil {
			logger.Warn(
				"daemon: dispatch terminal hook",
				"event",
				event.Kind,
				"terminal_id",
				event.TerminalID,
				"error",
				err,
			)
		}
	})
}

func dispatchTerminalHookEvent(
	ctx context.Context,
	hooks *hookspkg.Hooks,
	event terminalpkg.Event,
) error {
	base, terminalContext := terminalHookEnvelope(event)
	switch event.Kind {
	case terminalpkg.EventKindOpened:
		base.Event = hookspkg.HookTerminalOpened
		_, err := hooks.DispatchTerminalOpened(ctx, hookspkg.TerminalOpenedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			Mode: string(event.Detail.Mode), Cwd: event.Detail.Cwd, Title: event.Detail.Title,
		})
		return err
	case terminalpkg.EventKindClosed:
		base.Event = hookspkg.HookTerminalClosed
		_, err := hooks.DispatchTerminalClosed(ctx, hookspkg.TerminalClosedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			Exit: terminalHookExit(event.Exit), Reason: terminalHookCloseReason(event.Reason),
		})
		return err
	case terminalpkg.EventKindLeaseChanged:
		base.Event = hookspkg.HookTerminalLeaseChanged
		_, err := hooks.DispatchTerminalLeaseChanged(ctx, hookspkg.TerminalLeaseChangedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			From: string(event.Detail.LeaseFrom), To: string(event.Detail.LeaseTo), Reason: event.Reason,
		})
		return err
	case terminalpkg.EventKindCommandStarted:
		base.Event = hookspkg.HookTerminalCommandStarted
		_, err := hooks.DispatchTerminalCommandStarted(ctx, hookspkg.TerminalCommandStartedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			CommandID: event.Detail.CommandID, Command: event.Detail.Command,
			Cwd: event.Detail.Cwd, DetectedBy: event.Detail.DetectedBy,
		})
		return err
	case terminalpkg.EventKindCommandFinished:
		base.Event = hookspkg.HookTerminalCommandFinished
		_, err := hooks.DispatchTerminalCommandFinished(ctx, hookspkg.TerminalCommandFinishedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			CommandID: event.Detail.CommandID, ExitCode: event.Detail.ExitCode,
			Signal: event.Detail.Signal, ExitCause: event.Detail.ExitCause,
			DurationMS: event.Detail.DurationMS, DetectedBy: event.Detail.DetectedBy,
			Approval: event.Detail.Approval,
		})
		return err
	case terminalpkg.EventKindInputRequested:
		base.Event = hookspkg.HookTerminalInputRequested
		_, err := hooks.DispatchTerminalInputRequested(ctx, hookspkg.TerminalInputRequestedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			RequestID: string(event.Detail.RequestID), Reason: event.Reason, Redacted: event.Detail.Redacted,
		})
		return err
	case terminalpkg.EventKindInputProvided:
		base.Event = hookspkg.HookTerminalInputProvided
		_, err := hooks.DispatchTerminalInputProvided(ctx, hookspkg.TerminalInputProvidedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			RequestID: string(event.Detail.RequestID), Redacted: event.Detail.Redacted,
			Length: event.Detail.Length, Outcome: event.Detail.Outcome,
		})
		return err
	default:
		return dispatchTerminalStateHook(ctx, hooks, base, terminalContext, event)
	}
}

func dispatchTerminalStateHook(
	ctx context.Context,
	hooks *hookspkg.Hooks,
	base hookspkg.PayloadBase,
	terminalContext hookspkg.TerminalContext,
	event terminalpkg.Event,
) error {
	switch event.Kind {
	case terminalpkg.EventKindRecordingStarted:
		base.Event = hookspkg.HookTerminalRecordingStarted
		_, err := hooks.DispatchTerminalRecordingStarted(ctx, hookspkg.TerminalRecordingStartedPayload{
			PayloadBase: base, TerminalContext: terminalContext, RecordingID: event.Detail.RecordingID,
		})
		return err
	case terminalpkg.EventKindRecordingStopped:
		base.Event = hookspkg.HookTerminalRecordingStopped
		_, err := hooks.DispatchTerminalRecordingStopped(ctx, hookspkg.TerminalRecordingStoppedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			RecordingID: event.Detail.RecordingID, Digest: event.Detail.Digest, Bytes: event.Detail.Bytes,
			Reason: event.Reason, Truncated: event.Detail.Truncated,
		})
		return err
	case terminalpkg.EventKindSubscriberEvicted:
		base.Event = hookspkg.HookTerminalSubscriberEvicted
		_, err := hooks.DispatchTerminalSubscriberEvicted(ctx, hookspkg.TerminalSubscriberEvictedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			Flow: event.Detail.Flow, Reason: event.Reason,
		})
		return err
	case terminalpkg.EventKindLimitRejected:
		base.Event = hookspkg.HookTerminalLimitRejected
		_, err := hooks.DispatchTerminalLimitRejected(ctx, hookspkg.TerminalLimitRejectedPayload{
			PayloadBase: base, TerminalContext: terminalContext,
			Limit: event.Detail.Limit, Current: event.Detail.Current, Max: event.Detail.Max,
		})
		return err
	default:
		return nil
	}
}

func terminalHookEnvelope(event terminalpkg.Event) (hookspkg.PayloadBase, hookspkg.TerminalContext) {
	return hookspkg.PayloadBase{Timestamp: event.At}, hookspkg.TerminalContext{
		WorkspaceID: event.WorkspaceID, ProfileID: event.ProfileID, TerminalID: string(event.TerminalID),
		ActorKind: string(event.Actor.Kind), ActorID: event.Actor.ID,
		SessionID: event.Actor.SessionID, RunID: event.Actor.RunID,
		Generation: event.Actor.Generation, At: event.At,
	}
}

func terminalHookExit(exit *terminalpkg.Exit) hookspkg.TerminalExit {
	if exit == nil {
		return hookspkg.TerminalExit{Cause: "unknown"}
	}
	return hookspkg.TerminalExit{Cause: exit.Cause, Code: exit.Code, Signal: exit.Signal}
}

func terminalHookCloseReason(reason string) string {
	switch reason {
	case "expired", "exit_retention":
		return "reaped"
	case "workspace_deleted", "profile_archived", "shutdown":
		return reason
	default:
		return "closed"
	}
}
