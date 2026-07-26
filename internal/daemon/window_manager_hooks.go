package daemon

import (
	"context"
	"log/slog"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/windowmanager"
)

func newWindowManagerHookObserver(state *bootState) windowmanager.EventObserver {
	return func(ctx context.Context, event windowmanager.Event) {
		if state == nil || state.hookDispatcher == nil {
			return
		}
		hookEvent, payload, ok := windowManagerHookPayload(event)
		if !ok {
			return
		}
		dispatchCtx := context.WithoutCancel(ctx)
		if err := dispatchWindowManagerHook(dispatchCtx, state.hookDispatcher, hookEvent, payload); err != nil {
			logger := state.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.WarnContext(
				dispatchCtx,
				"window_manager.hook_dispatch_failed",
				"workspace_id", event.WorkspaceID,
				"revision", event.Revision,
				"command_id", event.CommandID,
				"hook_event", hookEvent,
				"error", err,
			)
		}
	}
}

func windowManagerHookPayload(
	event windowmanager.Event,
) (hookspkg.HookEvent, hookspkg.WindowManagerPayload, bool) {
	hookEvent, ok := windowManagerHookEvent(event.CommandID)
	if !ok {
		return "", hookspkg.WindowManagerPayload{}, false
	}
	return hookEvent, hookspkg.WindowManagerPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookEvent,
			Timestamp: event.OccurredAt,
		},
		WorkspaceID: string(event.WorkspaceID),
		Revision:    uint64(event.Revision),
		CommandID:   string(event.CommandID),
		Changes: hookspkg.WindowManagerChanges{
			DesktopIDs: windowManagerHookIDs(event.Changes.DesktopIDs),
			WindowIDs:  windowManagerHookIDs(event.Changes.WindowIDs),
			GroupIDs:   windowManagerHookIDs(event.Changes.GroupIDs),
			NodeIDs:    windowManagerHookIDs(event.Changes.NodeIDs),
			ClientIDs:  windowManagerHookIDs(event.Changes.ClientIDs),
		},
		Actor: hookspkg.WindowManagerActor{
			Kind: event.Actor.Kind,
			ID:   event.Actor.ID,
		},
		Origin: event.Origin,
	}, true
}

func windowManagerHookEvent(commandID windowmanager.CommandID) (hookspkg.HookEvent, bool) {
	switch commandID {
	case windowmanager.CommandLayoutArrange, windowmanager.CommandLayoutReplace:
		return hookspkg.HookWindowManagerLayoutApplied, true
	case windowmanager.CommandDesktopCreate:
		return hookspkg.HookWindowManagerDesktopCreated, true
	case windowmanager.CommandDesktopDelete:
		return hookspkg.HookWindowManagerDesktopDeleted, true
	case windowmanager.CommandWindowMove:
		return hookspkg.HookWindowManagerWindowMoved, true
	default:
		return "", false
	}
}

func dispatchWindowManagerHook(
	ctx context.Context,
	dispatcher *hookspkg.Hooks,
	event hookspkg.HookEvent,
	payload hookspkg.WindowManagerPayload,
) error {
	switch event {
	case hookspkg.HookWindowManagerLayoutApplied:
		_, err := dispatcher.DispatchWindowManagerLayoutApplied(ctx, payload)
		return err
	case hookspkg.HookWindowManagerDesktopCreated:
		_, err := dispatcher.DispatchWindowManagerDesktopCreated(ctx, payload)
		return err
	case hookspkg.HookWindowManagerDesktopDeleted:
		_, err := dispatcher.DispatchWindowManagerDesktopDeleted(ctx, payload)
		return err
	case hookspkg.HookWindowManagerWindowMoved:
		_, err := dispatcher.DispatchWindowManagerWindowMoved(ctx, payload)
		return err
	default:
		return nil
	}
}

func windowManagerHookIDs[T ~string](values []T) []string {
	if values == nil {
		return nil
	}
	ids := make([]string, len(values))
	for index, value := range values {
		ids[index] = string(value)
	}
	return ids
}
