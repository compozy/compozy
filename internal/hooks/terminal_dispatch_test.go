package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestDispatchTerminalHooksUseAsyncProfileOwnedPayloads(t *testing.T) {
	t.Parallel()

	events := []HookEvent{
		HookTerminalOpened,
		HookTerminalClosed,
		HookTerminalLeaseChanged,
		HookTerminalCommandStarted,
		HookTerminalCommandFinished,
		HookTerminalInputRequested,
		HookTerminalInputProvided,
		HookTerminalRecordingStarted,
		HookTerminalRecordingStopped,
		HookTerminalSubscriberEvicted,
		HookTerminalLimitRejected,
	}
	seen := make(chan map[string]any, len(events))
	decls := make([]HookDecl, 0, len(events)+1)
	executors := make(map[string]Executor, len(events)+1)
	for _, event := range events {
		name := event.String()
		decls = append(decls, HookDecl{
			Name: name, ProfileID: "profile-a", Event: event, Mode: HookModeAsync,
			Matcher: HookMatcher{WorkspaceID: "workspace-a"}, ExecutorKind: HookExecutorNative,
		})
		executors[name] = terminalCaptureExecutor(t, seen)
	}
	decls = append(decls, HookDecl{
		Name: "other-profile", ProfileID: "profile-b", Event: HookTerminalOpened,
		Mode: HookModeAsync, ExecutorKind: HookExecutorNative,
	})
	executors["other-profile"] = terminalCaptureExecutor(t, seen)
	hooks := newTestHooks(
		t,
		WithNativeDeclarations(decls),
		WithExecutorResolver(testExecutorResolver(executors)),
		WithAsyncWorkerCount(1),
		WithAsyncQueueCapacity(len(events)+1),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	for _, event := range events {
		if err := dispatchTerminalTestPayload(t.Context(), hooks, event); err != nil {
			t.Fatalf("dispatch %s error = %v", event, err)
		}
	}
	for _, event := range events {
		select {
		case payload := <-seen:
			if payload["event"] != event.String() || payload["workspace_id"] != "workspace-a" ||
				payload["profile_id"] != "profile-a" || payload["actor_id"] != "operator" {
				t.Fatalf("terminal payload for %s = %#v", event, payload)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for terminal hook %s", event)
		}
	}
	// Close drains every accepted async task, so an empty channel after this
	// barrier proves the cross-profile declaration did not run.
	hooks.Close()
	select {
	case payload := <-seen:
		t.Fatalf("cross-profile terminal hook executed: %#v", payload)
	default:
	}
}

func terminalCaptureExecutor(t *testing.T, seen chan<- map[string]any) Executor {
	t.Helper()
	return NewNativeExecutor(func(_ context.Context, _ RegisteredHook, payload []byte) ([]byte, error) {
		decoded := make(map[string]any)
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, err
		}
		seen <- decoded
		return []byte(`{}`), nil
	})
}

func dispatchTerminalTestPayload(ctx context.Context, hooks *Hooks, event HookEvent) error {
	base := PayloadBase{Event: event, Timestamp: time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)}
	terminalContext := TerminalContext{
		WorkspaceID: "workspace-a", ProfileID: "profile-a", TerminalID: "term-a",
		ActorKind: "human", ActorID: "operator", At: base.Timestamp,
	}
	switch event {
	case HookTerminalOpened:
		_, err := hooks.DispatchTerminalOpened(
			ctx,
			TerminalOpenedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalClosed:
		_, err := hooks.DispatchTerminalClosed(
			ctx,
			TerminalClosedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalLeaseChanged:
		_, err := hooks.DispatchTerminalLeaseChanged(
			ctx,
			TerminalLeaseChangedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalCommandStarted:
		_, err := hooks.DispatchTerminalCommandStarted(
			ctx,
			TerminalCommandStartedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalCommandFinished:
		_, err := hooks.DispatchTerminalCommandFinished(
			ctx,
			TerminalCommandFinishedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalInputRequested:
		_, err := hooks.DispatchTerminalInputRequested(
			ctx,
			TerminalInputRequestedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalInputProvided:
		_, err := hooks.DispatchTerminalInputProvided(
			ctx,
			TerminalInputProvidedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalRecordingStarted:
		_, err := hooks.DispatchTerminalRecordingStarted(
			ctx,
			TerminalRecordingStartedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalRecordingStopped:
		_, err := hooks.DispatchTerminalRecordingStopped(
			ctx,
			TerminalRecordingStoppedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalSubscriberEvicted:
		_, err := hooks.DispatchTerminalSubscriberEvicted(
			ctx,
			TerminalSubscriberEvictedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	case HookTerminalLimitRejected:
		_, err := hooks.DispatchTerminalLimitRejected(
			ctx,
			TerminalLimitRejectedPayload{PayloadBase: base, TerminalContext: terminalContext},
		)
		return err
	default:
		return fmt.Errorf("unexpected terminal event %q", event)
	}
}
