package hooks

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
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
		HookWindowManagerWindowOpened,
		HookWindowManagerWindowClosed,
		HookWindowManagerTabSplit,
		HookWindowManagerTabUnsplit,
		HookWindowManagerTabActivated,
		HookWindowManagerStackActivated,
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
					got.Origin != "native:compozy__window_manager_layout_arrange" ||
					len(got.Changes.WindowIDs) != 1 ||
					got.Changes.WindowIDs[0] != "window-a" {
					t.Fatalf("async window-manager payload = %#v", got)
				}
				assertWindowManagerEventChanges(t, event, got.Changes)
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for async window-manager hook %q", event)
			}
		})
	}
}

func windowManagerDispatchTestPayload(event HookEvent) WindowManagerPayload {
	payload := WindowManagerPayload{
		Event:       event,
		Timestamp:   time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC),
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
		Origin: "native:compozy__window_manager_layout_arrange",
	}
	switch event {
	case HookWindowManagerTabSplit:
		payload.Changes.TabSplit = []string{"tab-a"}
	case HookWindowManagerTabUnsplit:
		payload.Changes.TabUnsplit = []string{"tab-a"}
	}
	return payload
}

func assertWindowManagerEventChanges(t *testing.T, event HookEvent, changes WindowManagerChanges) {
	t.Helper()
	switch event {
	case HookWindowManagerTabSplit:
		if len(changes.TabSplit) != 1 || changes.TabSplit[0] != "tab-a" ||
			len(changes.TabUnsplit) != 0 {
			t.Fatalf("grouped stack changes = %#v", changes)
		}
	case HookWindowManagerTabUnsplit:
		if len(changes.TabUnsplit) != 1 || changes.TabUnsplit[0] != "tab-a" ||
			len(changes.TabSplit) != 0 {
			t.Fatalf("ungrouped stack changes = %#v", changes)
		}
	case HookWindowManagerStackActivated:
		if len(changes.NodeIDs) != 1 || changes.NodeIDs[0] != "node-a" ||
			len(changes.StackGrouped) != 0 || len(changes.StackUngrouped) != 0 {
			t.Fatalf("activated stack changes = %#v", changes)
		}
	default:
		if len(changes.StackGrouped) != 0 || len(changes.StackUngrouped) != 0 {
			t.Fatalf("non-stack changes = %#v", changes)
		}
	}
}

func dispatchWindowManagerTestPayload(
	ctx context.Context,
	hooks *Hooks,
	event HookEvent,
	payload WindowManagerPayload,
) error {
	switch event {
	case HookWindowManagerTabSplit:
		_, err := hooks.DispatchWindowManagerTabSplit(ctx, payload)
		return err
	case HookWindowManagerTabUnsplit:
		_, err := hooks.DispatchWindowManagerTabUnsplit(ctx, payload)
		return err
	case HookWindowManagerTabActivated:
		_, err := hooks.DispatchWindowManagerTabActivated(ctx, payload)
		return err
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
	case HookWindowManagerWindowOpened:
		_, err := hooks.DispatchWindowManagerWindowOpened(ctx, payload)
		return err
	case HookWindowManagerWindowClosed:
		_, err := hooks.DispatchWindowManagerWindowClosed(ctx, payload)
		return err
	case HookWindowManagerStackGrouped:
		_, err := hooks.DispatchWindowManagerStackGrouped(ctx, payload)
		return err
	case HookWindowManagerStackUngrouped:
		_, err := hooks.DispatchWindowManagerStackUngrouped(ctx, payload)
		return err
	case HookWindowManagerStackActivated:
		_, err := hooks.DispatchWindowManagerStackActivated(ctx, payload)
		return err
	default:
		return fmt.Errorf("unexpected window-manager event %q", event)
	}
}

// IT-015: hook loading owns one warning per retired event and load-only dispatch.
func TestRetiredWindowManagerHooksLoadWithoutDispatch(t *testing.T) {
	t.Run("Should warn once for load-only hooks without dispatching them", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		var calls atomic.Int64
		events := []HookEvent{HookWindowManagerStackGrouped, HookWindowManagerStackUngrouped}
		declarations := []HookDecl{}
		for _, event := range events {
			declarations = append(
				declarations,
				HookDecl{Name: event.String(), Event: event, Mode: HookModeAsync, ExecutorKind: HookExecutorNative},
			)
		}
		h := newTestHooks(
			t,
			WithLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
			WithNativeDeclarations(declarations),
			WithExecutorResolver(func(HookDecl) (Executor, error) {
				return NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ WindowManagerPayload) (WindowManagerObservationPatch, error) {
						calls.Add(1)
						return WindowManagerObservationPatch{}, nil
					},
				), nil
			}),
		)
		for range 2 {
			if err := h.Rebuild(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		for _, event := range events {
			if strings.Count(logs.String(), `"surface":"`+event.String()+`"`) != 1 {
				t.Fatalf("warnings: %s", logs.String())
			}
			if err := dispatchWindowManagerTestPayload(
				t.Context(),
				h,
				event,
				windowManagerDispatchTestPayload(event),
			); err != nil {
				t.Fatal(err)
			}
		}
		for _, event := range []HookEvent{HookWindowManagerTabSplit, HookWindowManagerTabUnsplit} {
			if err := dispatchWindowManagerTestPayload(
				t.Context(),
				h,
				event,
				windowManagerDispatchTestPayload(event),
			); err != nil {
				t.Fatal(err)
			}
		}
		// Close joins the async workers; no elapsed-time assertion is needed.
		h.Close()
		if calls.Load() != 0 {
			t.Fatalf("retired hook fired %d times", calls.Load())
		}
	})
}
