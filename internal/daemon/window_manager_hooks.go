package daemon

import (
	"context"
	"log/slog"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/windowmanager"
)

func newWindowManagerHookObserver(state *bootState) windowmanager.EventObserver {
	return func(ctx context.Context, event windowmanager.Event) {
		if state == nil || state.hookDispatcher == nil {
			return
		}
		dispatches := windowManagerHookPayloads(event)
		if len(dispatches) == 0 {
			return
		}
		dispatchCtx := context.WithoutCancel(ctx)
		for _, dispatch := range dispatches {
			if err := dispatchWindowManagerHook(
				dispatchCtx,
				state.hookDispatcher,
				dispatch.event,
				dispatch.payload,
			); err != nil {
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
					"hook_event", dispatch.event,
					"error", err,
				)
			}
		}
	}
}

type windowManagerHookDispatch struct {
	event   hookspkg.HookEvent
	payload hookspkg.WindowManagerPayload
}

func windowManagerHookPayloads(event windowmanager.Event) []windowManagerHookDispatch {
	hookEvents := windowManagerHookEventsForCommit(event)
	dispatches := make([]windowManagerHookDispatch, 0, len(hookEvents))
	for _, hookEvent := range hookEvents {
		dispatches = append(dispatches, windowManagerHookDispatch{
			event: hookEvent,
			payload: hookspkg.WindowManagerPayload{
				Event:       hookEvent,
				Timestamp:   event.OccurredAt,
				WorkspaceID: string(event.WorkspaceID),
				Revision:    uint64(event.Revision),
				CommandID:   string(event.CommandID),
				Changes: hookspkg.WindowManagerChanges{
					DesktopIDs:     windowManagerHookIDs(event.Changes.DesktopIDs),
					TabIDs:         windowManagerHookIDs(event.Changes.TabIDs),
					TabSplit:       windowManagerHookIDs(event.Changes.TabSplit),
					TabUnsplit:     windowManagerHookIDs(event.Changes.TabUnsplit),
					WindowIDs:      windowManagerHookIDs(event.Changes.WindowIDs),
					GroupIDs:       windowManagerHookIDs(event.Changes.GroupIDs),
					NodeIDs:        windowManagerHookIDs(event.Changes.NodeIDs),
					ClientIDs:      windowManagerHookIDs(event.Changes.ClientIDs),
					StackGrouped:   windowManagerHookIDs(event.Changes.StackGrouped),
					StackUngrouped: windowManagerHookIDs(event.Changes.StackUngrouped),
				},
				Actor: hookspkg.WindowManagerActor{
					Kind: event.Actor.Kind,
					ID:   event.Actor.ID,
				},
				Origin: event.Origin,
			},
		})
	}
	return dispatches
}

func windowManagerHookEventsForCommit(event windowmanager.Event) []hookspkg.HookEvent {
	hookEvents := make([]hookspkg.HookEvent, 0, 3)
	switch event.CommandID {
	case windowmanager.CommandLayoutArrange, windowmanager.CommandLayoutReplace:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerLayoutApplied)
	case windowmanager.CommandDesktopCreate:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerDesktopCreated)
	case windowmanager.CommandDesktopDelete:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerDesktopDeleted)
	case windowmanager.CommandWindowMove:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerWindowMoved)
	case windowmanager.CommandWindowOpen, windowmanager.CommandWindowReopen:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerWindowOpened)
	case windowmanager.CommandWindowClose:
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerWindowClosed)
	case windowmanager.CommandWindowTabActivate:
		hookEvents = append(
			hookEvents,
			hookspkg.HookWindowManagerTabActivated,
			hookspkg.HookWindowManagerStackActivated,
		)
	}
	if len(event.Changes.TabSplit) > 0 {
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerTabSplit)
	}
	if len(event.Changes.TabUnsplit) > 0 {
		hookEvents = append(hookEvents, hookspkg.HookWindowManagerTabUnsplit)
	}
	return hookEvents
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
	case hookspkg.HookWindowManagerWindowOpened:
		_, err := dispatcher.DispatchWindowManagerWindowOpened(ctx, payload)
		return err
	case hookspkg.HookWindowManagerWindowClosed:
		_, err := dispatcher.DispatchWindowManagerWindowClosed(ctx, payload)
		return err
	case hookspkg.HookWindowManagerTabSplit:
		_, err := dispatcher.DispatchWindowManagerTabSplit(ctx, payload)
		return err
	case hookspkg.HookWindowManagerTabUnsplit:
		_, err := dispatcher.DispatchWindowManagerTabUnsplit(ctx, payload)
		return err
	case hookspkg.HookWindowManagerTabActivated:
		_, err := dispatcher.DispatchWindowManagerTabActivated(ctx, payload)
		return err
	case hookspkg.HookWindowManagerStackActivated:
		_, err := dispatcher.DispatchWindowManagerStackActivated(ctx, payload)
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
