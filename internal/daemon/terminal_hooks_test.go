package daemon

// Suite: terminal-to-hook composition bridge.
// Invariant: each extension-visible terminal event maps once without manager re-entry or correlation-key loss.
// Boundary IN: terminal.EventBus values. Boundary OUT: the hooks async queue.

import (
	"context"
	"encoding/json"
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
		{terminalpkg.EventKindLeaseChanged, hookspkg.HookTerminalLeaseChanged},
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
		event := terminalpkg.TerminalEvent{
			Kind: testCase.kind, WorkspaceID: "workspace-a", ProfileID: "profile-a", TerminalID: "term-a",
			Actor: terminalpkg.Actor{
				Kind: terminalpkg.ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
				SessionID: "session-a", RunID: "run-a",
			},
			At: at, Reason: "operator_close",
			Exit: &terminalpkg.Exit{Cause: "exited", Code: intPointer(0)},
			Detail: terminalpkg.EventDetail{
				Mode: terminalpkg.ModePTY, Cwd: "/workspace", CommandID: "cmd-a", Command: "pwd",
				DetectedBy: "marker", ExitCode: intPointer(0), ExitCause: "exited", DurationMS: 12,
				Approval: "human", RecordingID: "rec-a", Digest: "digest-a", Bytes: 42,
				Flow: "drop", Limit: "workspace", Current: 8, Max: 8,
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
				got.payload["session_id"] != "session-a" || got.payload["run_id"] != "run-a" {
				t.Fatalf("bridge payload for %s = %#v (%s)", testCase.hook, got.payload, got.event)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for bridged hook %s", testCase.hook)
		}
	}
}

func intPointer(value int) *int { return &value }
