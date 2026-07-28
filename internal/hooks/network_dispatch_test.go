package hooks

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDispatchNetworkHooksUseAsyncObservationPayloads(t *testing.T) {
	t.Parallel()

	events := []HookEvent{
		HookNetworkPeerJoined,
		HookNetworkPeerLeft,
		HookNetworkThreadOpened,
		HookNetworkDirectRoomOpened,
		HookNetworkMessagePersisted,
		HookNetworkWorkOpened,
		HookNetworkWorkTransitioned,
		HookNetworkWorkClosed,
	}
	seen := make(chan HookEvent, len(events))
	decls := make([]HookDecl, 0, len(events))
	executors := make(map[string]Executor, len(events))
	for _, event := range events {
		matcher := &NetworkMatcher{Channel: "builders", Surface: "thread"}
		if event == HookNetworkDirectRoomOpened {
			matcher.Surface = "direct"
		}
		name := event.String()
		decls = append(decls, HookDecl{
			Name:         name,
			Event:        event,
			Mode:         HookModeAsync,
			Matcher:      HookMatcher{NetworkMatcher: matcher},
			ExecutorKind: HookExecutorNative,
		})
		executors[name] = NewTypedNativeExecutor(
			func(_ context.Context, _ RegisteredHook, payload NetworkPayload) (NetworkObservationPatch, error) {
				seen <- payload.Event
				return NetworkObservationPatch{}, nil
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
		t.Run(fmt.Sprintf("Should dispatch %s async observation payload", event), func(t *testing.T) {
			payload := networkDispatchTestPayload(event)
			var err error
			switch event {
			case HookNetworkPeerJoined:
				_, err = hooks.DispatchNetworkPeerJoined(t.Context(), payload)
			case HookNetworkPeerLeft:
				_, err = hooks.DispatchNetworkPeerLeft(t.Context(), payload)
			case HookNetworkThreadOpened:
				_, err = hooks.DispatchNetworkThreadOpened(t.Context(), payload)
			case HookNetworkDirectRoomOpened:
				_, err = hooks.DispatchNetworkDirectRoomOpened(t.Context(), payload)
			case HookNetworkMessagePersisted:
				_, err = hooks.DispatchNetworkMessagePersisted(t.Context(), payload)
			case HookNetworkWorkOpened:
				_, err = hooks.DispatchNetworkWorkOpened(t.Context(), payload)
			case HookNetworkWorkTransitioned:
				_, err = hooks.DispatchNetworkWorkTransitioned(t.Context(), payload)
			case HookNetworkWorkClosed:
				_, err = hooks.DispatchNetworkWorkClosed(t.Context(), payload)
			}
			if err != nil {
				t.Fatalf("dispatch %s error = %v", event, err)
			}

			select {
			case got := <-seen:
				if got != event {
					t.Fatalf("async network hook event = %q, want %q", got, event)
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for async network hook %q", event)
			}
		})
	}
}

func networkDispatchTestPayload(event HookEvent) NetworkPayload {
	payload := NetworkPayload{
		PayloadBase: PayloadBase{
			Event:     event,
			Timestamp: time.Date(2026, time.May, 5, 12, 0, 0, 0, time.UTC),
		},
		SessionID:   "sess-coder",
		Channel:     "builders",
		Surface:     "thread",
		ThreadID:    "thread_hooks_01",
		MessageID:   "msg-hook",
		Kind:        "trace",
		Direction:   "received",
		WorkID:      "work_hooks_01",
		WorkState:   "completed",
		PeerFrom:    "coder.sess-abc",
		PeerTo:      "reviewer.sess-xyz",
		TraceID:     "trace-hooks-01",
		CausationID: "cause-hooks-01",
	}
	if event == HookNetworkDirectRoomOpened {
		payload.Surface = "direct"
		payload.ThreadID = ""
		payload.DirectID = "direct_99401d24bee62651d189e5a561785466"
	}
	return payload
}
