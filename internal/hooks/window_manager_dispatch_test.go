package hooks

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDispatchWindowManagerHooksUseAsyncDurablePayloads(t *testing.T) {
	t.Parallel()

	events := []HookEvent{
		HookWindowManagerLayoutApplied,
		HookWindowManagerDesktopCreated,
		HookWindowManagerDesktopDeleted,
		HookWindowManagerWindowMoved,
	}
	seen := make(chan WindowManagerPayload, len(events))
	decls := make([]HookDecl, 0, len(events))
	executors := make(map[string]Executor, len(events))
	for _, event := range events {
		name := event.String()
		decls = append(decls, HookDecl{
			Name:         name,
			Event:        event,
			Mode:         HookModeAsync,
			Matcher:      HookMatcher{WorkspaceID: "workspace-a"},
			ExecutorKind: HookExecutorNative,
		})
		executors[name] = NewTypedNativeExecutor(
			func(
				_ context.Context,
				_ RegisteredHook,
				payload WindowManagerPayload,
			) (WindowManagerObservationPatch, error) {
				seen <- payload
				return WindowManagerObservationPatch{}, nil
			},
		)
	}
	hooks := newTestHooks(
		t,
		WithNativeDeclarations(decls),
		WithExecutorResolver(testExecutorResolver(executors)),
		WithAsyncWorkerCount(1),
		WithAsyncQueueCapacity(len(events)),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	for _, event := range events {
		t.Run(fmt.Sprintf("Should dispatch %s with durable command metadata", event), func(t *testing.T) {
			payload := windowManagerDispatchTestPayload(event)
			if err := dispatchWindowManagerTestPayload(t.Context(), hooks, event, payload); err != nil {
				t.Fatalf("dispatch %s error = %v", event, err)
			}

			select {
			case got := <-seen:
				if got.Event != event ||
					got.WorkspaceID != "workspace-a" ||
					got.Revision != 42 ||
					got.CommandID != "layout.arrange" ||
					got.Actor.Kind != "agent" ||
					got.Actor.ID != "agent-a" ||
					got.Origin != "native:agh__window_manager_layout_arrange" ||
					len(got.Changes.WindowIDs) != 1 ||
					got.Changes.WindowIDs[0] != "window-a" {
					t.Fatalf("async window-manager payload = %#v", got)
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for async window-manager hook %q", event)
			}
		})
	}
}

func windowManagerDispatchTestPayload(event HookEvent) WindowManagerPayload {
	return WindowManagerPayload{
		PayloadBase: PayloadBase{
			Event:     event,
			Timestamp: time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC),
		},
		WorkspaceID: "workspace-a",
		Revision:    42,
		CommandID:   "layout.arrange",
		Changes: WindowManagerChanges{
			DesktopIDs: []string{"desktop-a"},
			WindowIDs:  []string{"window-a"},
			GroupIDs:   []string{"group-a"},
			NodeIDs:    []string{"node-a"},
			ClientIDs:  []string{"client-a"},
		},
		Actor:  WindowManagerActor{Kind: "agent", ID: "agent-a"},
		Origin: "native:agh__window_manager_layout_arrange",
	}
}

func dispatchWindowManagerTestPayload(
	ctx context.Context,
	hooks *Hooks,
	event HookEvent,
	payload WindowManagerPayload,
) error {
	switch event {
	case HookWindowManagerLayoutApplied:
		_, err := hooks.DispatchWindowManagerLayoutApplied(ctx, payload)
		return err
	case HookWindowManagerDesktopCreated:
		_, err := hooks.DispatchWindowManagerDesktopCreated(ctx, payload)
		return err
	case HookWindowManagerDesktopDeleted:
		_, err := hooks.DispatchWindowManagerDesktopDeleted(ctx, payload)
		return err
	case HookWindowManagerWindowMoved:
		_, err := hooks.DispatchWindowManagerWindowMoved(ctx, payload)
		return err
	default:
		return fmt.Errorf("unexpected window-manager event %q", event)
	}
}
