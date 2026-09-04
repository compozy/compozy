package daemon

// Suite: terminal-to-hook composition bridge.
// Invariant: each extension-visible terminal event maps once without manager re-entry or correlation-key loss.
// Boundary IN: terminal notifier values. Boundary OUT: the hooks async queue.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func TestTerminalHookBridgeCoverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind terminalpkg.EventKind
		hook hookspkg.HookEvent
	}{
		{terminalpkg.EventKindOpened, hookspkg.HookTerminalOpened},
		{terminalpkg.EventKindClosed, hookspkg.HookTerminalClosed},
		{terminalpkg.EventKindCommandStarted, hookspkg.HookTerminalCommandStarted},
		{terminalpkg.EventKindCommandFinished, hookspkg.HookTerminalCommandFinished},
		{terminalpkg.EventKindInputRequested, hookspkg.HookTerminalInputRequested},
		{terminalpkg.EventKindInputProvided, hookspkg.HookTerminalInputProvided},
		{terminalpkg.EventKindRecordingStarted, hookspkg.HookTerminalRecordingStarted},
		{terminalpkg.EventKindRecordingStopped, hookspkg.HookTerminalRecordingStopped},
		{terminalpkg.EventKindSubscriberEvicted, hookspkg.HookTerminalSubscriberEvicted},
		{terminalpkg.EventKindLimitRejected, hookspkg.HookTerminalLimitRejected},
	}
	type delivered struct {
		event   hookspkg.HookEvent
		payload map[string]any
	}
	seen := make(chan delivered, len(cases))
	decls := make([]hookspkg.HookDecl, 0, len(cases))
	executors := make(map[string]hookspkg.Executor, len(cases))
	for _, testCase := range cases {
		name := testCase.hook.String()
		decls = append(decls, hookspkg.HookDecl{
			Name: name, ProfileID: "profile-a", Event: testCase.hook,
			Mode: hookspkg.HookModeAsync, ExecutorKind: hookspkg.HookExecutorNative,
		})
		executors[name] = hookspkg.NewNativeExecutor(func(
			_ context.Context,
			hook hookspkg.RegisteredHook,
			payload []byte,
		) ([]byte, error) {
			decoded := make(map[string]any)
			if err := json.Unmarshal(payload, &decoded); err != nil {
				return nil, err
			}
			seen <- delivered{event: hook.Event, payload: decoded}
			return []byte(`{}`), nil
		})
	}
	hooks := hookspkg.NewHooks(
		hookspkg.WithNativeDeclarations(decls),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			return executors[decl.Name], nil
		}),
		hookspkg.WithAsyncWorkerCount(1),
		hookspkg.WithAsyncQueueCapacity(len(cases)),
	)
	t.Cleanup(hooks.Close)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	for _, testCase := range cases {
		event := terminalpkg.Event{
			Kind: testCase.kind, WorkspaceID: "workspace-a", ProfileID: "profile-a", TerminalID: "term-a",
			Actor: terminalpkg.Actor{
				Kind: terminalpkg.ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
				SessionID: "session-a", RunID: "run-a", Generation: 17,
			},
			At: at, Reason: "operator_close",
			Exit: &terminalpkg.Exit{Cause: "exited", Code: new(0)},
			Detail: &terminalpkg.EventDetail{
				Mode: terminalpkg.ModePTY, Cwd: "/workspace", Title: "Build", CommandID: "cmd-a", Command: "pwd",
				DetectedBy: "marker", ExitCode: new(0), ExitCause: "exited", DurationMS: 12,
				Signal: new("TERM"), Approval: "human", RequestID: "request-a", Redacted: true,
				Length: 7, Outcome: "provided", RecordingID: "rec-a", Digest: "digest-a", Bytes: 42,
				Truncated: true,
				Flow:      "drop", Limit: "workspace", Current: 8, Max: 8,
			},
		}
		if err := dispatchTerminalHookEvent(t.Context(), hooks, event); err != nil {
			t.Fatalf("dispatch %s error = %v", testCase.kind, err)
		}
	}

	for _, testCase := range cases {
		select {
		case got := <-seen:
			if got.event != testCase.hook || got.payload["event"] != testCase.hook.String() ||
				got.payload["workspace_id"] != "workspace-a" || got.payload["profile_id"] != "profile-a" ||
				got.payload["terminal_id"] != "term-a" || got.payload["actor_id"] != "agent-a" ||
				got.payload["session_id"] != "session-a" || got.payload["run_id"] != "run-a" ||
				got.payload["generation"] != float64(17) {
				t.Fatalf("bridge payload for %s = %#v (%s)", testCase.hook, got.payload, got.event)
			}
			for key, want := range terminalHookExpectedPayload(testCase.hook) {
				if value := got.payload[key]; !reflect.DeepEqual(value, want) {
					t.Fatalf("bridge payload %s[%q] = %#v, want %#v", testCase.hook, key, value, want)
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for bridged hook %s", testCase.hook)
		}
	}
}

func terminalHookExpectedPayload(event hookspkg.HookEvent) map[string]any {
	switch event {
	case hookspkg.HookTerminalOpened:
		return map[string]any{"mode": "pty", "cwd": "/workspace", "title": "Build"}
	case hookspkg.HookTerminalClosed:
		return map[string]any{"exit": map[string]any{"cause": "exited", "code": float64(0)}, "reason": "closed"}
	case hookspkg.HookTerminalCommandStarted:
		return map[string]any{"command_id": "cmd-a", "command": "pwd", "cwd": "/workspace", "detected_by": "marker"}
	case hookspkg.HookTerminalCommandFinished:
		return map[string]any{
			"command_id":  "cmd-a",
			"exit_code":   float64(0),
			"signal":      "TERM",
			"exit_cause":  "exited",
			"duration_ms": float64(12),
			"detected_by": "marker",
			"approval":    "human",
		}
	case hookspkg.HookTerminalInputRequested:
		return map[string]any{"request_id": "request-a", "reason": "operator_close", "redacted": true}
	case hookspkg.HookTerminalInputProvided:
		return map[string]any{"request_id": "request-a", "redacted": true, "length": float64(7), "outcome": "provided"}
	case hookspkg.HookTerminalRecordingStarted:
		return map[string]any{"recording_id": "rec-a"}
	case hookspkg.HookTerminalRecordingStopped:
		return map[string]any{
			"recording_id": "rec-a",
			"digest":       "digest-a",
			"bytes":        float64(42),
			"reason":       "operator_close",
			"truncated":    true,
		}
	case hookspkg.HookTerminalSubscriberEvicted:
		return map[string]any{"flow": "drop", "reason": "operator_close"}
	case hookspkg.HookTerminalLimitRejected:
		return map[string]any{"limit": "workspace", "current": float64(8), "max": float64(8)}
	default:
		return nil
	}
}
