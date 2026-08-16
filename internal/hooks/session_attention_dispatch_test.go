package hooks

import (
	"context"
	"testing"
	"time"
)

func TestDispatchSessionAttentionChangedContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose an async-only introspection descriptor", func(t *testing.T) {
		t.Parallel()

		descriptors := FilterEventDescriptors(EventFilter{Family: HookEventFamilySession})
		var descriptor EventDescriptor
		for _, candidate := range descriptors {
			if candidate.Event == HookSessionAttentionChanged {
				descriptor = candidate
				break
			}
		}
		if descriptor.Event == "" {
			t.Fatal("session.attention.changed descriptor is missing")
		}
		if descriptor.Family != HookEventFamilySession || descriptor.SyncEligible ||
			descriptor.PayloadSchema != "SessionAttentionChangedPayload" ||
			descriptor.PatchSchema != "SessionAttentionObservationPatch" {
			t.Fatalf("session attention descriptor = %#v, want complete async session contract", descriptor)
		}
	})

	t.Run("Should clone the durable payload before asynchronous delivery", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		release := make(chan struct{})
		seenWorktree := make(chan string, 1)
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name: "attention-observer", Event: HookSessionAttentionChanged,
				Mode: HookModeAsync, ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"attention-observer": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						payload SessionAttentionChangedPayload,
					) (SessionAttentionObservationPatch, error) {
						close(started)
						<-release
						seenWorktree <- payload.WorktreeIDValue()
						payload.WorktreeID = "executor-mutation"
						return SessionAttentionObservationPatch{}, nil
					},
				),
			})),
			WithAsyncWorkerCount(1),
			WithAsyncQueueCapacity(1),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}
		runtimeContext := &SessionRuntimeContext{WorktreeID: "worktree-before"}
		payload := sessionAttentionDispatchPayload(runtimeContext)
		if _, err := hooks.DispatchSessionAttentionChanged(t.Context(), payload); err != nil {
			t.Fatalf("DispatchSessionAttentionChanged() error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for session attention hook")
		}
		runtimeContext.WorktreeID = "worktree-after"
		close(release)
		select {
		case got := <-seenWorktree:
			if got != "worktree-before" {
				t.Fatalf("async payload worktree = %q, want cloned worktree-before", got)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for cloned session attention payload")
		}
		if runtimeContext.WorktreeID != "worktree-after" {
			t.Fatalf("source runtime context = %q, want executor mutation isolated", runtimeContext.WorktreeID)
		}
	})

	t.Run("Should isolate a throwing observer and continue delivery", func(t *testing.T) {
		t.Parallel()

		observerCalled := make(chan struct{}, 1)
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{
				{
					Name: "attention-panic", Event: HookSessionAttentionChanged,
					Mode: HookModeAsync, ExecutorKind: HookExecutorNative,
				},
				{
					Name: "attention-observer", Event: HookSessionAttentionChanged,
					Mode: HookModeAsync, ExecutorKind: HookExecutorNative,
				},
			}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"attention-panic": NewTypedNativeExecutor(
					func(
						context.Context,
						RegisteredHook,
						SessionAttentionChangedPayload,
					) (SessionAttentionObservationPatch, error) {
						panic("attention observer failed")
					},
				),
				"attention-observer": NewTypedNativeExecutor(
					func(
						context.Context,
						RegisteredHook,
						SessionAttentionChangedPayload,
					) (SessionAttentionObservationPatch, error) {
						observerCalled <- struct{}{}
						return SessionAttentionObservationPatch{}, nil
					},
				),
			})),
			WithAsyncWorkerCount(1),
			WithAsyncQueueCapacity(2),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}
		if _, err := hooks.DispatchSessionAttentionChanged(
			t.Context(),
			sessionAttentionDispatchPayload(nil),
		); err != nil {
			t.Fatalf("DispatchSessionAttentionChanged() error = %v, want fail-open nil", err)
		}
		select {
		case <-observerCalled:
		case <-time.After(time.Second):
			t.Fatal("healthy attention observer did not run after throwing observer")
		}
	})
}

func sessionAttentionDispatchPayload(runtimeContext *SessionRuntimeContext) SessionAttentionChangedPayload {
	at := time.Date(2026, 8, 15, 20, 15, 0, 0, time.UTC)
	return SessionAttentionChangedPayload{
		PayloadBase: PayloadBase{Event: HookSessionAttentionChanged, Timestamp: at},
		SessionContext: SessionContext{
			SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder",
			SessionRuntimeContext: runtimeContext, CreatedAt: at.Add(-time.Hour), UpdatedAt: at,
		},
		From: "idle", To: "done", Class: "finished", At: at,
	}
}
