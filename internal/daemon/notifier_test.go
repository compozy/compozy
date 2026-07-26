package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHooksNotifierDispatchesLifecycleAgentAndStreamEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch lifecycle agent and stream events in order", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
		var order []string
		runtime := &fakeHookRuntime{
			onDispatchCreate: func(_ context.Context, payload hookspkg.SessionPostCreatePayload) error {
				order = append(order, "create")
				if payload.Event != hookspkg.HookSessionPostCreate {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookSessionPostCreate)
				}
				if payload.Timestamp != fixedNow {
					t.Fatalf("payload.Timestamp = %s, want %s", payload.Timestamp, fixedNow)
				}
				if payload.SessionID != "sess-created" || payload.WorkspaceID != "ws-1" {
					t.Fatalf("payload = %#v, want session metadata", payload)
				}
				return nil
			},
			onDispatchStop: func(_ context.Context, payload hookspkg.SessionPostStopPayload) error {
				order = append(order, "stop")
				if payload.Event != hookspkg.HookSessionPostStop {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookSessionPostStop)
				}
				if payload.CreatedAt.IsZero() || payload.UpdatedAt.IsZero() {
					t.Fatalf("payload timestamps = %#v, want created/updated timestamps", payload)
				}
				return nil
			},
			onTurnStart: func(_ context.Context, payload hookspkg.TurnStartPayload) error {
				order = append(order, "turn-start")
				if payload.Event != hookspkg.HookTurnStart {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookTurnStart)
				}
				return nil
			},
			onTurnEnd: func(_ context.Context, payload hookspkg.TurnEndPayload) error {
				order = append(order, "turn-end")
				if payload.Event != hookspkg.HookTurnEnd {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookTurnEnd)
				}
				return nil
			},
			onMessageStart: func(_ context.Context, payload hookspkg.MessageStartPayload) error {
				order = append(order, "message-start")
				if payload.Event != hookspkg.HookMessageStart {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookMessageStart)
				}
				return nil
			},
			onMessageDelta: func(_ context.Context, payload hookspkg.MessageDeltaPayload) error {
				order = append(order, "message-delta")
				if payload.Event != hookspkg.HookMessageDelta {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookMessageDelta)
				}
				return nil
			},
			onMessageEnd: func(_ context.Context, payload hookspkg.MessageEndPayload) error {
				order = append(order, "message-end")
				if payload.Event != hookspkg.HookMessageEnd {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookMessageEnd)
				}
				return nil
			},
			onPreCompact: func(_ context.Context, payload hookspkg.ContextPreCompactPayload) error {
				order = append(order, "context-pre")
				if payload.Event != hookspkg.HookContextPreCompact {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookContextPreCompact)
				}
				return nil
			},
			onPostCompact: func(_ context.Context, payload hookspkg.ContextPostCompactPayload) error {
				order = append(order, "context-post")
				if payload.Event != hookspkg.HookContextPostCompact {
					t.Fatalf("payload.Event = %q, want %q", payload.Event, hookspkg.HookContextPostCompact)
				}
				return nil
			},
			onToolPreCall: func(_ context.Context, payload hookspkg.ToolPreCallPayload) error {
				order = append(order, "tool-pre")
				if payload.SessionID != "sess-created" || payload.WorkspaceID != "ws-1" {
					t.Fatalf("payload session context = %#v, want session metadata", payload.SessionContext)
				}
				if payload.ToolID != "Read" {
					t.Fatalf("payload.ToolID = %q, want %q", payload.ToolID, "Read")
				}
				return nil
			},
		}
		agentEvents := &recordingNotifier{}
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(runtime, agentEvents)

		sess := &session.Session{
			ID:          "sess-created",
			Name:        "demo",
			AgentName:   "codex",
			WorkspaceID: "ws-1",
			Workspace:   "/tmp/ws-1",
			Type:        session.SessionTypeUser,
			State:       session.StateActive,
			CreatedAt:   fixedNow.Add(-time.Minute),
			UpdatedAt:   fixedNow,
		}

		if _, err := notifier.DispatchSessionPostCreate(
			testutil.Context(t),
			hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostCreate, fixedNow),
		); err != nil {
			t.Fatalf("DispatchSessionPostCreate() error = %v", err)
		}
		if _, err := notifier.DispatchSessionPostStop(
			testutil.Context(t),
			hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostStop, fixedNow),
		); err != nil {
			t.Fatalf("DispatchSessionPostStop() error = %v", err)
		}
		if _, err := notifier.DispatchTurnStart(testutil.Context(t), hookspkg.TurnStartPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookTurnStart, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
		}); err != nil {
			t.Fatalf("DispatchTurnStart() error = %v", err)
		}
		if _, err := notifier.DispatchMessageStart(testutil.Context(t), hookspkg.MessageStartPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookMessageStart, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
			MessageID:      "msg-1",
		}); err != nil {
			t.Fatalf("DispatchMessageStart() error = %v", err)
		}
		if _, err := notifier.DispatchMessageDelta(testutil.Context(t), hookspkg.MessageDeltaPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookMessageDelta, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
			MessageID:      "msg-1",
		}); err != nil {
			t.Fatalf("DispatchMessageDelta() error = %v", err)
		}
		if _, err := notifier.DispatchMessageEnd(testutil.Context(t), hookspkg.MessageEndPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookMessageEnd, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
			MessageID:      "msg-1",
		}); err != nil {
			t.Fatalf("DispatchMessageEnd() error = %v", err)
		}
		if _, err := notifier.DispatchTurnEnd(testutil.Context(t), hookspkg.TurnEndPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookTurnEnd, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
		}); err != nil {
			t.Fatalf("DispatchTurnEnd() error = %v", err)
		}
		if _, err := notifier.DispatchContextPreCompact(testutil.Context(t), hookspkg.ContextPreCompactPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookContextPreCompact, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
		}); err != nil {
			t.Fatalf("DispatchContextPreCompact() error = %v", err)
		}
		if _, err := notifier.DispatchContextPostCompact(testutil.Context(t), hookspkg.ContextPostCompactPayload{
			PayloadBase:    hookspkg.PayloadBase{Event: hookspkg.HookContextPostCompact, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{SessionID: "sess-created"},
			TurnContext:    hookspkg.TurnContext{TurnID: "turn-1"},
		}); err != nil {
			t.Fatalf("DispatchContextPostCompact() error = %v", err)
		}
		rawToolEvent, err := json.Marshal(map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    "tool-1",
			"kind":          "read",
			"rawInput": map[string]any{
				"path": "/tmp/demo.txt",
			},
			"_meta": map[string]any{
				"claudeCode": map[string]any{
					"toolName": "Read",
				},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal(tool event) error = %v", err)
		}
		notifier.OnAgentEventForSession(testutil.Context(t), sess, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			SessionID:  "acp-session-1",
			TurnID:     "turn-1",
			ToolCallID: "tool-1",
			Raw:        rawToolEvent,
		})

		wantOrder := []string{
			"create",
			"stop",
			"turn-start",
			"message-start",
			"message-delta",
			"message-end",
			"turn-end",
			"context-pre",
			"context-post",
			"tool-pre",
		}
		if !testutil.EqualStringSlices(order, wantOrder) {
			t.Fatalf("dispatch order = %#v, want %#v", order, wantOrder)
		}
		if got, want := agentEvents.events, []string{"agent"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("agent event notifier events = %#v, want %#v", got, want)
		}
	})
}

func TestHooksNotifierOnAgentEventUsesProvidedSessionIDAndLifecycleNoops(t *testing.T) {
	t.Parallel()

	t.Run("Should use the provided session ID and tolerate nil lifecycle sessions", func(t *testing.T) {
		t.Parallel()

		var gotSessionID string
		runtime := &fakeHookRuntime{
			onToolPreCall: func(_ context.Context, payload hookspkg.ToolPreCallPayload) error {
				gotSessionID = payload.SessionID
				return nil
			},
		}
		notifier := newHooksNotifier(discardLogger(), func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		})
		notifier.setRuntime(runtime, &recordingNotifier{})

		notifier.OnSessionCreated(testutil.Context(t), nil)
		notifier.OnSessionStopped(testutil.Context(t), nil)
		notifier.OnAgentEvent(testutil.Context(t), "sess-direct", acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			SessionID:  "acp-session-1",
			TurnID:     "turn-1",
			ToolCallID: "tool-1",
			Raw: mustJSON(t, map[string]any{
				"sessionUpdate": "tool_call",
				"toolCallId":    "tool-1",
				"kind":          "read",
				"_meta": map[string]any{
					"claudeCode": map[string]any{
						"toolName": "Read",
					},
				},
			}),
		})

		if gotSessionID != "sess-direct" {
			t.Fatalf("tool hook SessionID = %q, want %q", gotSessionID, "sess-direct")
		}
	})
}

func TestHooksNotifierLifecycleForwarding(t *testing.T) {
	t.Parallel()

	t.Run("Should forward full session metadata to downstream observers", func(t *testing.T) {
		t.Parallel()

		downstream := &recordingLifecycleForwardNotifier{}
		notifier := newHooksNotifier(discardLogger(), func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		})
		notifier.setRuntime(&fakeHookRuntime{}, downstream)
		child := &session.Session{
			ID:                   "sess-child",
			Name:                 "spawned worker",
			AgentName:            "reviewer",
			Provider:             "codex",
			WorkspaceID:          "ws-1",
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "default"),
			Type:                 session.SessionTypeSpawned,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-parent",
				RootSessionID:   "sess-parent",
				SpawnDepth:      1,
			},
			State:            session.StateActive,
			SoulSnapshotID:   "soul-child",
			SoulDigest:       "sha256:child",
			ParentSoulDigest: "sha256:parent",
		}

		notifier.OnSessionCreated(testutil.Context(t), child)
		notifier.OnSessionStopped(testutil.Context(t), child)

		if got := len(downstream.created); got != 1 {
			t.Fatalf("created calls = %d, want 1", got)
		}
		created := downstream.created[0].Info()
		if created.Lineage == nil ||
			created.Lineage.ParentSessionID != "sess-parent" ||
			created.ParentSoulDigest != "sha256:parent" ||
			created.SoulDigest != "sha256:child" {
			t.Fatalf("created session metadata = %#v", created)
		}
		if got := len(downstream.stopped); got != 1 {
			t.Fatalf("stopped calls = %d, want 1", got)
		}
	})
}

func TestHooksNotifierSubprocessHealthRuntimeForwarding(t *testing.T) {
	t.Parallel()

	t.Run("Should forward health and the full stopped session to the late-bound runtime", func(t *testing.T) {
		t.Parallel()

		healthRuntime := &recordingSubprocessHealthRuntime{}
		notifier := newHooksNotifier(discardLogger(), nil)
		notifier.setRuntime(&fakeHookRuntime{}, &recordingNotifier{})
		notifier.setSubprocessHealthRuntime(healthRuntime)
		observation := session.SubprocessHealthSnapshot{
			SessionID:   "sess-health",
			WorkspaceID: "ws-health",
			AgentName:   "worker",
		}
		stopped := &session.Session{
			ID:          observation.SessionID,
			WorkspaceID: observation.WorkspaceID,
			AgentName:   observation.AgentName,
			State:       session.StateStopped,
		}

		notifier.OnSubprocessHealth(testutil.Context(t), observation)
		notifier.OnSessionStopped(testutil.Context(t), stopped)

		if got, want := len(healthRuntime.observations), 1; got != want {
			t.Fatalf("health observation calls = %d, want %d", got, want)
		}
		if got := healthRuntime.observations[0].SessionID; got != observation.SessionID {
			t.Fatalf("health observation session_id = %q, want %q", got, observation.SessionID)
		}
		if got, want := len(healthRuntime.stopped), 1; got != want {
			t.Fatalf("stopped session calls = %d, want %d", got, want)
		}
		if healthRuntime.stopped[0] != stopped {
			t.Fatal("stopped session was projected, want the full runtime session pointer")
		}
	})
}

func TestHooksNotifierEmitsGlobalHookDispatchSummariesForAutonomyHooks(t *testing.T) {
	t.Parallel()

	t.Run("Should emit correlated global summaries for autonomy hooks", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 5, 4, 22, 0, 0, 0, time.UTC)
		decls := []hookspkg.HookDecl{
			{
				Name:         "coord-stop-observer",
				Event:        hookspkg.HookCoordinatorStopped,
				Mode:         hookspkg.HookModeSync,
				Source:       hookspkg.HookSourceConfig,
				Required:     true,
				Priority:     1000,
				PrioritySet:  true,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "task-release-observer",
				Event:        hookspkg.HookTaskRunReleased,
				Mode:         hookspkg.HookModeSync,
				Source:       hookspkg.HookSourceConfig,
				Required:     true,
				Priority:     1000,
				PrioritySet:  true,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		}
		executors := map[string]hookspkg.Executor{
			"coord-stop-observer": hookspkg.NewTypedNativeExecutor(
				func(
					context.Context,
					hookspkg.RegisteredHook,
					hookspkg.CoordinatorStoppedPayload,
				) (hookspkg.CoordinatorObservationPatch, error) {
					return hookspkg.CoordinatorObservationPatch{}, nil
				},
			),
			"task-release-observer": hookspkg.NewTypedNativeExecutor(
				func(
					context.Context,
					hookspkg.RegisteredHook,
					hookspkg.TaskRunReleasedPayload,
				) (hookspkg.TaskRunObservationPatch, error) {
					return hookspkg.TaskRunObservationPatch{}, nil
				},
			),
		}

		hooks := hookspkg.NewHooks(
			hookspkg.WithLogger(discardLogger()),
			hookspkg.WithNativeDeclarations(decls),
			hookspkg.WithExecutorResolver(daemonExecutorResolver(executors)),
		)
		t.Cleanup(hooks.Close)
		if err := hooks.Rebuild(testutil.Context(t)); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		summaries := &recordingEventSummaryWriter{}
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(hooks, nil, summaries)

		_, err := notifier.DispatchCoordinatorStopped(testutil.Context(t), hookspkg.CoordinatorStoppedPayload{
			PayloadBase: hookspkg.PayloadBase{
				Event:     hookspkg.HookCoordinatorStopped,
				Timestamp: fixedNow,
			},
			CoordinatorContext: hookspkg.CoordinatorContext{
				WorkspaceID:          "ws-1",
				AgentName:            "coordinator",
				CoordinatorSessionID: "sess-coordinator-1",
				TaskID:               "task-1",
				RunID:                "run-1",
				WorkflowID:           "wf-1",
			},
			StopReason: "completed",
		})
		if err != nil {
			t.Fatalf("DispatchCoordinatorStopped() error = %v", err)
		}

		_, err = notifier.DispatchTaskRunReleased(testutil.Context(t), hookspkg.TaskRunReleasedPayload{
			PayloadBase: hookspkg.PayloadBase{
				Event:     hookspkg.HookTaskRunReleased,
				Timestamp: fixedNow,
			},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:        "task-1",
				RunID:         "run-1",
				WorkspaceID:   "ws-1",
				WorkflowID:    "wf-1",
				SessionID:     "sess-worker-1",
				AgentName:     "worker",
				ActorKind:     "agent_session",
				ActorID:       "sess-worker-1",
				ReleaseReason: "manual_release",
			},
			PreviousRunStatus: "claimed",
			PreviousSessionID: "sess-worker-1",
			RecoveryReason:    "manual_release",
		})
		if err != nil {
			t.Fatalf("DispatchTaskRunReleased() error = %v", err)
		}

		records := summaries.snapshot()
		if got, want := len(records), 5; got != want {
			t.Fatalf("len(summary records) = %d, want %d", got, want)
		}

		coordinatorWatch := findEventSummaryByType(t, records, string(hookspkg.HookCoordinatorStopped))
		if got, want := coordinatorWatch.CoordinatorSessionID, "sess-coordinator-1"; got != want {
			t.Fatalf("coordinator watch CoordinatorSessionID = %q, want %q", got, want)
		}
		if got, want := coordinatorWatch.HookEvent, string(hookspkg.HookCoordinatorStopped); got != want {
			t.Fatalf("coordinator watch HookEvent = %q, want %q", got, want)
		}
		var coordinatorWatchContent map[string]any
		if err := json.Unmarshal(coordinatorWatch.Content, &coordinatorWatchContent); err != nil {
			t.Fatalf("Unmarshal(coordinator watch content) error = %v", err)
		}
		if got, want := coordinatorWatchContent[watchEventsPayloadStopReasonKey], "completed"; got != want {
			t.Fatalf("coordinator watch stop_reason = %v, want %q", got, want)
		}

		coordinatorStart := findEventSummaryByHookName(t, records, "coord-stop-observer", "hook.dispatch.start")
		if got, want := coordinatorStart.CoordinatorSessionID, "sess-coordinator-1"; got != want {
			t.Fatalf("coordinator start CoordinatorSessionID = %q, want %q", got, want)
		}
		if got, want := coordinatorStart.WorkflowID, "wf-1"; got != want {
			t.Fatalf("coordinator start WorkflowID = %q, want %q", got, want)
		}

		releaseStart := findEventSummaryByHookName(t, records, "task-release-observer", "hook.dispatch.start")
		if got, want := releaseStart.ReleaseReason, "manual_release"; got != want {
			t.Fatalf("release start ReleaseReason = %q, want %q", got, want)
		}
		if got, want := releaseStart.ActorID, "sess-worker-1"; got != want {
			t.Fatalf("release start ActorID = %q, want %q", got, want)
		}
		if got, want := releaseStart.TaskID, "task-1"; got != want {
			t.Fatalf("release start TaskID = %q, want %q", got, want)
		}
	})
}

func TestHooksNotifierWritesCoordinatorWatchSummaryWithoutConfiguredHook(t *testing.T) {
	t.Parallel()

	t.Run("Should persist a dedicated coordinator row without configured hooks", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 9, 13, 0, 0, 0, time.UTC)
		summaries := &recordingEventSummaryWriter{}
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(&fakeHookRuntime{}, nil, summaries)

		_, err := notifier.DispatchCoordinatorStopped(testutil.Context(t), hookspkg.CoordinatorStoppedPayload{
			PayloadBase: hookspkg.PayloadBase{
				Event:     hookspkg.HookCoordinatorStopped,
				Timestamp: fixedNow,
			},
			CoordinatorContext: hookspkg.CoordinatorContext{
				WorkspaceID:          "ws-1",
				AgentName:            "coordinator",
				CoordinatorSessionID: "sess-coordinator-no-hook",
				TaskID:               "task-1",
				RunID:                "run-1",
				WorkflowID:           "wf-1",
				Provider:             "mock",
			},
			StopReason: "completed",
		})
		if err != nil {
			t.Fatalf("DispatchCoordinatorStopped(no hook) error = %v", err)
		}

		records := summaries.snapshot()
		if got, want := len(records), 1; got != want {
			t.Fatalf("len(summary records) = %d, want dedicated coordinator row only", got)
		}
		record := findEventSummaryByType(t, records, string(hookspkg.HookCoordinatorStopped))
		if got, want := record.WorkspaceID, "ws-1"; got != want {
			t.Fatalf("WorkspaceID = %q, want %q", got, want)
		}
		if got, want := record.CoordinatorSessionID, "sess-coordinator-no-hook"; got != want {
			t.Fatalf("CoordinatorSessionID = %q, want %q", got, want)
		}
		if got, want := record.HookEvent, string(hookspkg.HookCoordinatorStopped); got != want {
			t.Fatalf("HookEvent = %q, want %q", got, want)
		}
		if got, want := record.WorkflowID, "wf-1"; got != want {
			t.Fatalf("WorkflowID = %q, want %q", got, want)
		}
		if record.HookName != "" {
			t.Fatalf("HookName = %q, want empty dedicated row", record.HookName)
		}
	})
}

func TestHooksNotifierDispatchesPhaseCWatchObservers(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch phase C observers with durable redacted summaries", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)
		summaries := &recordingEventSummaryWriter{}
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(&fakeHookRuntime{}, nil, summaries)
		coordinatorObserver := &recordingCoordinatorWatchObserver{}
		eventObserver := &recordingEventRecordWatchObserver{}
		notifier.AddCoordinatorWatchObserver(coordinatorObserver)
		notifier.AddEventRecordWatchObserver(eventObserver)

		if _, err := notifier.DispatchCoordinatorSpawned(
			testutil.Context(t),
			coordinatorPayloadForWatchObserverTest(hookspkg.HookCoordinatorSpawned, fixedNow),
		); err != nil {
			t.Fatalf("DispatchCoordinatorSpawned() error = %v", err)
		}
		if _, err := notifier.DispatchCoordinatorDecision(
			testutil.Context(t),
			coordinatorPayloadForWatchObserverTest(hookspkg.HookCoordinatorDecision, fixedNow),
		); err != nil {
			t.Fatalf("DispatchCoordinatorDecision() error = %v", err)
		}
		if _, err := notifier.DispatchCoordinatorStopped(
			testutil.Context(t),
			coordinatorPayloadForWatchObserverTest(hookspkg.HookCoordinatorStopped, fixedNow),
		); err != nil {
			t.Fatalf("DispatchCoordinatorStopped() error = %v", err)
		}
		if _, err := notifier.DispatchCoordinatorFailed(
			testutil.Context(t),
			coordinatorPayloadForWatchObserverTest(hookspkg.HookCoordinatorFailed, fixedNow),
		); err != nil {
			t.Fatalf("DispatchCoordinatorFailed() error = %v", err)
		}
		if _, err := notifier.DispatchEventPostRecord(testutil.Context(t), hookspkg.EventPostRecordPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookEventPostRecord, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{
				SessionID:   "sess-hot",
				WorkspaceID: "ws-1",
				AgentName:   "coder",
			},
			TurnContext: hookspkg.TurnContext{TurnID: "turn-1"},
			RecordType:  "agent_message",
			Sequence:    42,
			Content:     json.RawMessage(`{"secret":"do not leak"}`),
		}); err != nil {
			t.Fatalf("DispatchEventPostRecord() error = %v", err)
		}

		records := summaries.snapshot()
		for _, event := range []hookspkg.HookEvent{
			hookspkg.HookCoordinatorSpawned,
			hookspkg.HookCoordinatorDecision,
			hookspkg.HookCoordinatorStopped,
			hookspkg.HookCoordinatorFailed,
		} {
			record := findEventSummaryByType(t, records, string(event))
			if got, want := record.CoordinatorSessionID, "sess-coordinator-phase-c"; got != want {
				t.Fatalf("%s CoordinatorSessionID = %q, want %q", event, got, want)
			}
			if got, want := record.WorkspaceID, "ws-1"; got != want {
				t.Fatalf("%s WorkspaceID = %q, want %q", event, got, want)
			}
		}
		if got, want := coordinatorObserver.events, []hookspkg.HookEvent{
			hookspkg.HookCoordinatorSpawned,
			hookspkg.HookCoordinatorDecision,
			hookspkg.HookCoordinatorStopped,
			hookspkg.HookCoordinatorFailed,
		}; !slices.Equal(got, want) {
			t.Fatalf("coordinator observer events = %#v, want %#v", got, want)
		}
		if got, want := eventObserver.payloads, []int64{42}; !slices.Equal(got, want) {
			t.Fatalf("event observer sequences = %#v, want %#v", got, want)
		}
	})
}

func coordinatorPayloadForWatchObserverTest(
	event hookspkg.HookEvent,
	now time.Time,
) hookspkg.CoordinatorLifecyclePayload {
	return hookspkg.CoordinatorLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{Event: event, Timestamp: now},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:          "ws-1",
			AgentName:            "coordinator",
			CoordinatorSessionID: "sess-coordinator-phase-c",
			TaskID:               "task-1",
			RunID:                "run-1",
			WorkflowID:           "wf-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "chan-1",
			}),
			Provider: "mock",
			Model:    "mock-model",
		},
		DecisionKind: "next_action",
		Decision:     "continue",
		StopReason:   "completed",
		Error:        "boom",
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func findEventSummaryByType(
	t *testing.T,
	summaries []store.EventSummary,
	eventType string,
) store.EventSummary {
	t.Helper()

	for _, summary := range summaries {
		if summary.Type == eventType {
			return summary
		}
	}
	t.Fatalf("missing event summary type %q in %#v", eventType, summaries)
	return store.EventSummary{}
}

func findEventSummaryByHookName(
	t *testing.T,
	summaries []store.EventSummary,
	hookName string,
	eventType string,
) store.EventSummary {
	t.Helper()

	for _, summary := range summaries {
		if summary.HookName == hookName && summary.Type == eventType {
			return summary
		}
	}
	t.Fatalf("missing event summary for hook %q type %q in %#v", hookName, eventType, summaries)
	return store.EventSummary{}
}

type recordingLifecycleForwardNotifier struct {
	created []*session.Session
	stopped []*session.Session
}

func (n *recordingLifecycleForwardNotifier) OnSessionCreated(_ context.Context, sess *session.Session) {
	n.created = append(n.created, sess)
}

func (n *recordingLifecycleForwardNotifier) OnSessionStopped(_ context.Context, sess *session.Session) {
	n.stopped = append(n.stopped, sess)
}

func (n *recordingLifecycleForwardNotifier) OnAgentEvent(context.Context, string, any) {}

type recordingEventSummaryWriter struct {
	mu        sync.Mutex
	summaries []store.EventSummary
}

func (w *recordingEventSummaryWriter) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.summaries = append(w.summaries, summary)
	return nil
}

func (w *recordingEventSummaryWriter) snapshot() []store.EventSummary {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]store.EventSummary(nil), w.summaries...)
}

type recordingCoordinatorWatchObserver struct {
	events []hookspkg.HookEvent
}

func (o *recordingCoordinatorWatchObserver) OnCoordinatorSpawned(
	_ context.Context,
	payload hookspkg.CoordinatorSpawnedPayload,
) error {
	o.events = append(o.events, payload.Event)
	return nil
}

func (o *recordingCoordinatorWatchObserver) OnCoordinatorDecision(
	_ context.Context,
	payload hookspkg.CoordinatorDecisionPayload,
) error {
	o.events = append(o.events, payload.Event)
	return nil
}

func (o *recordingCoordinatorWatchObserver) OnCoordinatorStopped(
	_ context.Context,
	payload hookspkg.CoordinatorStoppedPayload,
) error {
	o.events = append(o.events, payload.Event)
	return nil
}

func (o *recordingCoordinatorWatchObserver) OnCoordinatorFailed(
	_ context.Context,
	payload hookspkg.CoordinatorFailedPayload,
) error {
	o.events = append(o.events, payload.Event)
	return nil
}

type recordingEventRecordWatchObserver struct {
	payloads []int64
}

func (o *recordingEventRecordWatchObserver) OnEventPostRecord(
	_ context.Context,
	payload hookspkg.EventPostRecordPayload,
) error {
	o.payloads = append(o.payloads, payload.Sequence)
	return nil
}

func TestDaemonNativeHooksDriveObserverAndDreamCallbacks(t *testing.T) {
	t.Parallel()

	t.Run("Should drive observer dream extractor and automatic title callbacks", func(t *testing.T) {
		t.Parallel()

		observer := &spyLifecycleObserver{}
		dream := &spyDreamRuntime{}
		extractor := newSpyMessagePersistedObserver()
		title := newSpyMessagePersistedObserver()
		decls, executors := daemonNativeHooks(observer, dream, extractor, title)
		hooks := hookspkg.NewHooks(
			hookspkg.WithLogger(discardLogger()),
			hookspkg.WithNativeDeclarations(decls),
			hookspkg.WithExecutorResolver(daemonExecutorResolver(executors)),
		)
		t.Cleanup(hooks.Close)

		if err := hooks.Rebuild(testutil.Context(t)); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		fixedNow := time.Date(2026, 4, 9, 15, 0, 0, 0, time.UTC)
		sess := &session.Session{
			ID:          "sess-user",
			Name:        "demo",
			AgentName:   "codex",
			WorkspaceID: "ws-1",
			Workspace:   "/tmp/ws-1",
			Type:        session.SessionTypeUser,
			State:       session.StateStopped,
			CreatedAt:   fixedNow.Add(-time.Hour),
			UpdatedAt:   fixedNow,
		}

		if _, err := hooks.DispatchSessionPostCreate(
			testutil.Context(t),
			hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostCreate, fixedNow),
		); err != nil {
			t.Fatalf("DispatchSessionPostCreate() error = %v", err)
		}
		if _, err := hooks.DispatchSessionPostStop(
			testutil.Context(t),
			hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostStop, fixedNow),
		); err != nil {
			t.Fatalf("DispatchSessionPostStop() error = %v", err)
		}
		messagePayload := hookspkg.SessionMessagePersistedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookSessionMessagePersisted, Timestamp: fixedNow},
			SessionContext: hookspkg.SessionContext{
				SessionID:   sess.ID,
				AgentName:   sess.AgentName,
				WorkspaceID: sess.WorkspaceID,
				Workspace:   sess.Workspace,
			},
			MessageID:  "msg-1",
			MessageSeq: 1,
			Role:       "assistant",
			Text:       "done",
		}
		if _, err := hooks.DispatchSessionMessagePersisted(testutil.Context(t), messagePayload); err != nil {
			t.Fatalf("DispatchSessionMessagePersisted() error = %v", err)
		}

		if got := len(observer.created); got != 1 {
			t.Fatalf("len(observer.created) = %d, want 1", got)
		}
		if got := len(observer.stopped); got != 1 {
			t.Fatalf("len(observer.stopped) = %d, want 1", got)
		}
		if observer.created[0].Info().CreatedAt != sess.CreatedAt {
			t.Fatalf("observer created CreatedAt = %s, want %s", observer.created[0].Info().CreatedAt, sess.CreatedAt)
		}
		if got, want := dream.calls, []string{"session_stop:ws-1"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("dream calls = %#v, want %#v", got, want)
		}
		gotMessage := extractor.wait(t)
		if gotMessage.SessionID != sess.ID || gotMessage.MessageID != "msg-1" {
			t.Fatalf("extractor payload = %#v, want session/message ids", gotMessage)
		}
		gotTitleMessage := title.wait(t)
		if gotTitleMessage.SessionID != sess.ID || gotTitleMessage.MessageID != "msg-1" {
			t.Fatalf("automatic title payload = %#v, want session/message ids", gotTitleMessage)
		}
	})
}

func TestDreamSessionStopExecutorSkipsDreamSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should skip dream sessions without enqueueing a check", func(t *testing.T) {
		t.Parallel()

		dream := &spyDreamRuntime{}
		type typedExecutor = hookspkg.TypedNativeExecutor[
			hookspkg.SessionLifecyclePayload,
			hookspkg.SessionPostStopPatch,
		]
		rawExecutor := dreamSessionStopExecutor(dream)
		executor, ok := rawExecutor.(*typedExecutor)
		if !ok {
			t.Fatalf("dreamSessionStopExecutor() type = %T, want typed native executor", rawExecutor)
		}

		if _, err := executor.ExecuteTyped(
			testutil.Context(t),
			hookspkg.RegisteredHook{Name: "daemon.dream.session_stop", Event: hookspkg.HookSessionPostStop},
			hookspkg.SessionLifecyclePayload{
				PayloadBase: hookspkg.PayloadBase{
					Event:     hookspkg.HookSessionPostStop,
					Timestamp: time.Date(2026, 4, 9, 15, 0, 0, 0, time.UTC),
				},
				SessionContext: hookspkg.SessionContext{
					SessionID:   "sess-dream",
					WorkspaceID: "ws-dream",
					SessionType: string(session.SessionTypeDream),
					State:       string(session.StateStopped),
				},
			},
		); err != nil {
			t.Fatalf("ExecuteTyped() error = %v", err)
		}

		if got := dream.calls; len(got) != 0 {
			t.Fatalf("dream calls = %#v, want none for dream session stop", got)
		}
	})
}

func TestMarketplaceHookAllowedHonorsConsentKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should honor slug registry and hash consent keys", func(t *testing.T) {
		t.Parallel()

		marketplaceSkill := marketplaceSkillForTest("registry.example", "@registry/hook-a", "hash-123")
		if marketplaceHookAllowed(marketplaceSkill, nil) {
			t.Fatal("marketplaceHookAllowed() = true, want false without consent")
		}

		allowed := marketplaceHookAllowlist([]string{"@registry/hook-a"})
		if !marketplaceHookAllowed(marketplaceSkill, allowed) {
			t.Fatal("marketplaceHookAllowed() = false, want true for allowed slug")
		}

		allowed = marketplaceHookAllowlist([]string{"registry.example:@registry/hook-a"})
		if !marketplaceHookAllowed(marketplaceSkill, allowed) {
			t.Fatal("marketplaceHookAllowed() = false, want true for allowed registry slug")
		}

		allowed = marketplaceHookAllowlist([]string{"hash-123"})
		if !marketplaceHookAllowed(marketplaceSkill, allowed) {
			t.Fatal("marketplaceHookAllowed() = false, want true for allowed hash")
		}
	})
}

func TestHooksBridgeHelperCloningAndTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("Should clone declarations and use the notifier clock", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(discardLogger(), nil)
		before := time.Now().UTC().Add(-time.Second)
		got := notifier.timestamp()
		after := time.Now().UTC().Add(time.Second)
		if got.Before(before) || got.After(after) {
			t.Fatalf("timestamp() = %s, want current time between %s and %s", got, before, after)
		}

		readOnly := true
		original := hookspkg.HookDecl{
			Name:     "config-hook",
			Source:   hookspkg.HookSourceConfig,
			Args:     []string{"one"},
			Env:      map[string]string{"KEY": "value"},
			Metadata: map[string]string{"note": "keep"},
			Matcher: hookspkg.HookMatcher{
				ToolReadOnly: &readOnly,
			},
		}

		filtered := filterHookDeclsBySource([]hookspkg.HookDecl{original}, hookspkg.HookSourceConfig)
		if len(filtered) != 1 {
			t.Fatalf("len(filtered) = %d, want 1", len(filtered))
		}
		filtered[0].Args[0] = "changed"
		filtered[0].Env["KEY"] = "changed"
		filtered[0].Metadata["note"] = "changed"
		*filtered[0].Matcher.ToolReadOnly = false

		if original.Args[0] != "one" {
			t.Fatalf("original.Args = %#v, want unchanged", original.Args)
		}
		if original.Env["KEY"] != "value" {
			t.Fatalf("original.Env = %#v, want unchanged", original.Env)
		}
		if original.Metadata["note"] != "keep" {
			t.Fatalf("original.Metadata = %#v, want unchanged", original.Metadata)
		}
		if !*original.Matcher.ToolReadOnly {
			t.Fatal("original matcher ToolReadOnly was mutated")
		}

		if got := cloneStringMap(nil); got != nil {
			t.Fatalf("cloneStringMap(nil) = %#v, want nil", got)
		}
	})
}

func TestScopeWorkspaceHookDeclsOnlyInjectsSupportedMatcherFields(t *testing.T) {
	t.Parallel()

	newDecls := func() []hookspkg.HookDecl {
		return []hookspkg.HookDecl{
			{
				Name:  "session",
				Event: hookspkg.HookSessionPostCreate,
			},
			{
				Name:  "task-run",
				Event: hookspkg.HookTaskRunEnqueued,
			},
			{
				Name:  "message",
				Event: hookspkg.HookMessageDelta,
			},
		}
	}
	resolved := workspaceResolvedForTest("ws-1", "/tmp/ws-1")

	testCases := []struct {
		name   string
		assert func(t *testing.T, decls []hookspkg.HookDecl, scoped []hookspkg.HookDecl)
	}{
		{
			name: "Should inject workspace id and root for session hooks",
			assert: func(t *testing.T, _ []hookspkg.HookDecl, scoped []hookspkg.HookDecl) {
				t.Helper()

				if scoped[0].Matcher.WorkspaceID != resolved.WorkspaceID {
					t.Fatalf("session WorkspaceID = %q, want %q", scoped[0].Matcher.WorkspaceID, resolved.WorkspaceID)
				}
				if scoped[0].Matcher.WorkspaceRoot != resolved.RootDir {
					t.Fatalf("session WorkspaceRoot = %q, want %q", scoped[0].Matcher.WorkspaceRoot, resolved.RootDir)
				}
				if err := hookspkg.ValidateMatcherForEvent(scoped[0].Event, scoped[0].Matcher); err != nil {
					t.Fatalf("session matcher validation error = %v", err)
				}
			},
		},
		{
			name: "Should inject only workspace id for task-run hooks",
			assert: func(t *testing.T, _ []hookspkg.HookDecl, scoped []hookspkg.HookDecl) {
				t.Helper()

				if scoped[1].Matcher.WorkspaceID != resolved.WorkspaceID {
					t.Fatalf("task-run WorkspaceID = %q, want %q", scoped[1].Matcher.WorkspaceID, resolved.WorkspaceID)
				}
				if scoped[1].Matcher.WorkspaceRoot != "" {
					t.Fatalf(
						"task-run WorkspaceRoot = %q, want empty because task-run hooks do not support it",
						scoped[1].Matcher.WorkspaceRoot,
					)
				}
				if err := hookspkg.ValidateMatcherForEvent(scoped[1].Event, scoped[1].Matcher); err != nil {
					t.Fatalf("task-run matcher validation error = %v", err)
				}
			},
		},
		{
			name: "Should not inject workspace fields for message hooks",
			assert: func(t *testing.T, _ []hookspkg.HookDecl, scoped []hookspkg.HookDecl) {
				t.Helper()

				if scoped[2].Matcher.WorkspaceID != "" || scoped[2].Matcher.WorkspaceRoot != "" {
					t.Fatalf(
						"message matcher workspace fields = %#v, want no unsupported workspace scoping",
						scoped[2].Matcher,
					)
				}
				if err := hookspkg.ValidateMatcherForEvent(scoped[2].Event, scoped[2].Matcher); err != nil {
					t.Fatalf("message matcher validation error = %v", err)
				}
			},
		},
		{
			name: "Should not mutate original declarations",
			assert: func(t *testing.T, decls []hookspkg.HookDecl, _ []hookspkg.HookDecl) {
				t.Helper()

				for idx, decl := range decls {
					if decl.Matcher.WorkspaceID != "" || decl.Matcher.WorkspaceRoot != "" {
						t.Fatalf("decls[%d] matcher workspace fields were mutated: %#v", idx, decl.Matcher)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decls := newDecls()
			scoped := scopeWorkspaceHookDecls(decls, &resolved)
			if len(scoped) != len(decls) {
				t.Fatalf("len(scoped) = %d, want %d", len(scoped), len(decls))
			}
			tc.assert(t, decls, scoped)
		})
	}
}

func TestScopeWorkspaceHookDeclsInjectsWorkspaceWorkingDirWhenUnset(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the workspace root as working directory for relative hooks", func(t *testing.T) {
		t.Parallel()

		resolved := workspaceResolvedForTest("ws-1", "/tmp/ws-1")
		decls := []hookspkg.HookDecl{
			{
				Name:  "task-run",
				Event: hookspkg.HookTaskRunEnqueued,
			},
			{
				Name:  "message",
				Event: hookspkg.HookMessageDelta,
			},
		}

		scoped := scopeWorkspaceHookDecls(decls, &resolved)
		if got, want := scoped[0].WorkingDir, resolved.RootDir; got != want {
			t.Fatalf("task-run WorkingDir = %q, want %q", got, want)
		}
		if got, want := scoped[1].WorkingDir, resolved.RootDir; got != want {
			t.Fatalf("message WorkingDir = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve an explicit working directory", func(t *testing.T) {
		t.Parallel()

		resolved := workspaceResolvedForTest("ws-1", "/tmp/ws-1")
		decls := []hookspkg.HookDecl{{
			Name:       "explicit",
			Event:      hookspkg.HookTaskRunEnqueued,
			WorkingDir: "/tmp/keep-me",
		}}

		scoped := scopeWorkspaceHookDecls(decls, &resolved)
		if got, want := scoped[0].WorkingDir, "/tmp/keep-me"; got != want {
			t.Fatalf("explicit WorkingDir = %q, want %q", got, want)
		}
	})
}

func TestDispatchRuntimeAndExecutorResolvers(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch through runtime and resolve supported executors", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(
			discardLogger(),
			func() time.Time { return time.Date(2026, 4, 9, 16, 0, 0, 0, time.UTC) },
		)
		var nilCtx context.Context
		payload, err := dispatchRuntime(
			nilCtx,
			notifier,
			hookspkg.HookSessionPostCreate,
			"seed",
			func(_ hookRuntime, _ context.Context, item string) (string, error) {
				return item + "-unused", nil
			},
		)
		if err != nil {
			t.Fatalf("dispatchRuntime(nil runtime) error = %v, want nil", err)
		}
		if payload != "seed" {
			t.Fatalf("dispatchRuntime(nil runtime) payload = %q, want %q", payload, "seed")
		}

		runtime := &fakeHookRuntime{}
		notifier.setRuntime(runtime, nil)

		result, err := dispatchRuntime(
			context.Background(),
			notifier,
			hookspkg.HookEventPreRecord,
			"seed",
			func(_ hookRuntime, _ context.Context, item string) (string, error) {
				return item + "-ok", nil
			},
		)
		if err != nil {
			t.Fatalf("dispatchRuntime(rebuild false) error = %v, want nil", err)
		}
		if result != "seed-ok" {
			t.Fatalf("dispatchRuntime(rebuild false) result = %q, want %q", result, "seed-ok")
		}

		result, err = dispatchRuntime(
			context.Background(),
			notifier,
			hookspkg.HookSessionPostCreate,
			"seed",
			func(_ hookRuntime, _ context.Context, item string) (string, error) {
				return item + "-dispatched", nil
			},
		)
		if err != nil {
			t.Fatalf("dispatchRuntime() error = %v, want nil", err)
		}
		if result != "seed-dispatched" {
			t.Fatalf("dispatchRuntime() result = %q, want %q", result, "seed-dispatched")
		}

		_, err = dispatchRuntime(
			nilCtx,
			notifier,
			hookspkg.HookSessionPostCreate,
			"seed",
			func(_ hookRuntime, _ context.Context, item string) (string, error) {
				return item, nil
			},
		)
		if err == nil || !strings.Contains(err.Error(), "requires a non-nil context") {
			t.Fatalf("dispatchRuntime(nil context) error = %v, want non-nil context detail", err)
		}

		workspaceRoot := t.TempDir()
		subprocessExecutor, err := defaultDaemonExecutorResolver(hookspkg.HookDecl{
			Name:         "subprocess",
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "/bin/sh",
			Args:         []string{"-c", "printf '%s|' \"$HOOK_SCOPE_ENV\"; pwd"},
			Env:          map[string]string{"HOOK_SCOPE_ENV": "kept"},
			Matcher:      hookspkg.HookMatcher{WorkspaceRoot: workspaceRoot},
		})
		if err != nil {
			t.Fatalf("defaultDaemonExecutorResolver(subprocess) error = %v, want nil", err)
		}
		if subprocessExecutor.Kind() != hookspkg.HookExecutorSubprocess {
			t.Fatalf(
				"subprocess executor kind = %q, want %q",
				subprocessExecutor.Kind(),
				hookspkg.HookExecutorSubprocess,
			)
		}
		output, err := subprocessExecutor.Execute(t.Context(), hookspkg.RegisteredHook{Name: "subprocess"}, nil)
		if err != nil {
			t.Fatalf("subprocess executor.Execute() error = %v, want nil", err)
		}
		resolvedWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
		if err != nil {
			t.Fatalf("EvalSymlinks(workspaceRoot) error = %v, want nil", err)
		}
		if got := strings.TrimSpace(string(output)); got != "kept|"+resolvedWorkspaceRoot {
			t.Fatalf("subprocess executor output = %q, want %q", got, "kept|"+resolvedWorkspaceRoot)
		}

		explicitDir := t.TempDir()
		subprocessExecutor, err = defaultDaemonExecutorResolver(hookspkg.HookDecl{
			Name:         "subprocess-working-dir",
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "/bin/sh",
			Args:         []string{"-c", "pwd"},
			WorkingDir:   explicitDir,
			Matcher:      hookspkg.HookMatcher{WorkspaceRoot: workspaceRoot},
		})
		if err != nil {
			t.Fatalf("defaultDaemonExecutorResolver(subprocess working dir) error = %v, want nil", err)
		}
		output, err = subprocessExecutor.Execute(
			t.Context(),
			hookspkg.RegisteredHook{Name: "subprocess-working-dir"},
			nil,
		)
		if err != nil {
			t.Fatalf("subprocess executor.Execute(working dir) error = %v, want nil", err)
		}
		resolvedExplicitDir, err := filepath.EvalSymlinks(explicitDir)
		if err != nil {
			t.Fatalf("EvalSymlinks(explicitDir) error = %v, want nil", err)
		}
		if got := strings.TrimSpace(string(output)); got != resolvedExplicitDir {
			t.Fatalf("subprocess executor working dir output = %q, want %q", got, resolvedExplicitDir)
		}

		wasmExecutor, err := defaultDaemonExecutorResolver(hookspkg.HookDecl{
			Name:         "wasm",
			ExecutorKind: hookspkg.HookExecutorWASM,
		})
		if err != nil {
			t.Fatalf("defaultDaemonExecutorResolver(wasm) error = %v, want nil", err)
		}
		if wasmExecutor.Kind() != hookspkg.HookExecutorWASM {
			t.Fatalf("wasm executor kind = %q, want %q", wasmExecutor.Kind(), hookspkg.HookExecutorWASM)
		}

		if _, err := defaultDaemonExecutorResolver(hookspkg.HookDecl{
			Name:         "native",
			ExecutorKind: hookspkg.HookExecutorNative,
		}); err == nil || !strings.Contains(err.Error(), "requires an explicit binding") {
			t.Fatalf("defaultDaemonExecutorResolver(native) error = %v, want explicit binding error", err)
		}

		if _, err := defaultDaemonExecutorResolver(hookspkg.HookDecl{
			Name:         "unknown",
			ExecutorKind: hookspkg.HookExecutorKind("mystery"),
		}); err == nil || !strings.Contains(err.Error(), "unsupported executor kind") {
			t.Fatalf("defaultDaemonExecutorResolver(unknown) error = %v, want unsupported kind error", err)
		}

		resolver := daemonExecutorResolver(map[string]hookspkg.Executor{
			"bound": hookspkg.NewTypedNativeExecutor(
				func(context.Context, hookspkg.RegisteredHook, hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		})
		nativeExecutor, err := resolver(hookspkg.HookDecl{Name: "bound", ExecutorKind: hookspkg.HookExecutorNative})
		if err != nil {
			t.Fatalf("daemonExecutorResolver(bound native) error = %v, want nil", err)
		}
		if nativeExecutor.Kind() != hookspkg.HookExecutorNative {
			t.Fatalf("native executor kind = %q, want %q", nativeExecutor.Kind(), hookspkg.HookExecutorNative)
		}

		if _, err := resolver(
			hookspkg.HookDecl{Name: "missing", ExecutorKind: hookspkg.HookExecutorNative},
		); err == nil ||
			!strings.Contains(err.Error(), "missing native hook executor") {
			t.Fatalf("daemonExecutorResolver(missing native) error = %v, want missing native executor error", err)
		}
	})
}

func TestHooksNotifierLoopStartedObserverFiltersNonRunningStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should skip coordinator backstop for queued loop starts", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 4, 11, 30, 0, 0, time.UTC)
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		loopStore := &recordingLoopHookStore{}
		backstop := &recordingLoopBackstopRunner{}
		observer, err := newLoopNativeHookObserver(loopStore, notifier, backstop, func() time.Time { return fixedNow })
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}
		notifier.AddLoopStartedObserver(observer)

		if _, err := notifier.DispatchLoopStarted(t.Context(), hookspkg.LoopStartedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopStarted, Timestamp: fixedNow},
			LoopContext: hookspkg.LoopContext{
				LoopRunID:   "queued-loop-run",
				WorkspaceID: "ws-1",
				LoopName:    "daily-review",
			},
			Status: string(looppkg.StatusQueued),
		}); err != nil {
			t.Fatalf("DispatchLoopStarted() error = %v", err)
		}

		if got := len(backstop.calls); got != 0 {
			t.Fatalf("backstop calls = %d, want 0 for queued loop start", got)
		}
	})
}

func TestHooksNotifierLoopNodeTerminalObserverFiltersAndWakes(t *testing.T) {
	t.Parallel()

	t.Run("Should filter terminal task runs and wake the loop coordinator", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
		var nodePayloads []hookspkg.LoopNodeTerminalPayload
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(&fakeHookRuntime{
			onLoopNodeTerminal: func(_ context.Context, payload hookspkg.LoopNodeTerminalPayload) error {
				nodePayloads = append(nodePayloads, payload)
				return nil
			},
		}, nil)
		loopStore := &recordingLoopHookStore{}
		backstop := &recordingLoopBackstopRunner{}
		observer, err := newLoopNativeHookObserver(loopStore, notifier, backstop, func() time.Time { return fixedNow })
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}
		notifier.AddTaskRunTerminalObserver(observer)

		if _, err := notifier.DispatchTaskRunCompleted(t.Context(), hookspkg.TaskRunCompletedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID: "task-no-loop",
				RunID:  "run-no-loop",
			},
		}); err != nil {
			t.Fatalf("DispatchTaskRunCompleted(non-loop) error = %v", err)
		}
		coordinatorKind := taskpkg.RunKindCoordinator.String()
		if _, err := notifier.DispatchTaskRunCompleted(t.Context(), hookspkg.TaskRunCompletedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:    "task-coordinator",
				RunID:     "run-coordinator",
				RunKind:   &coordinatorKind,
				LoopRunID: "loop-run-1",
			},
		}); err != nil {
			t.Fatalf("DispatchTaskRunCompleted(coordinator) error = %v", err)
		}
		workerKind := taskpkg.RunKindWorker.String()
		if _, err := notifier.DispatchTaskRunFailed(t.Context(), hookspkg.TaskRunFailedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunFailed, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:      "task-node",
				RunID:       "run-node",
				RunKind:     &workerKind,
				LoopRunID:   "loop-run-1",
				WorkspaceID: "ws-1",
				RunStatus:   taskpkg.TaskRunStatusFailed.String(),
				TaskStatus:  string(taskpkg.TaskStatusFailed),
				Error:       "node failed",
			},
		}); err != nil {
			t.Fatalf("DispatchTaskRunFailed(loop node) error = %v", err)
		}

		if got, want := len(nodePayloads), 1; got != want {
			t.Fatalf("loop.node.terminal payload count = %d, want %d", got, want)
		}
		if got, want := nodePayloads[0].LoopRunID, "loop-run-1"; got != want {
			t.Fatalf("LoopRunID = %q, want %q", got, want)
		}
		if got, want := len(loopStore.progress), 1; got != want {
			t.Fatalf("progress calls = %d, want %d", got, want)
		}
		if got, want := loopStore.progress[0].loopRunID, "loop-run-1"; got != want {
			t.Fatalf("progress loop_run_id = %q, want %q", got, want)
		}
		if got, want := len(loopStore.wakes), 1; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		wake := loopStore.wakes[0]
		if got, want := wake.loopRunID, "loop-run-1"; got != want {
			t.Fatalf("wake loop_run_id = %q, want %q", got, want)
		}
		if got, want := wake.idempotencyKey, "loop.coordinator.node_terminal.loop-run-1.run-node"; got != want {
			t.Fatalf("IdempotencyKey = %q, want %q", got, want)
		}
		if got, want := len(backstop.calls), 1; got != want {
			t.Fatalf("backstop calls = %d, want %d", got, want)
		}
	})
}

func TestHooksNotifierCoordinatorTerminalObserverWakesParentAndPromotesQueued(t *testing.T) {
	t.Parallel()

	t.Run("Should wake parent awaiters and promote the queued loop FIFO", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)
		notifier := newHooksNotifier(discardLogger(), func() time.Time { return fixedNow })
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		loopStore := &recordingLoopHookStore{}
		backstop := &recordingLoopBackstopRunner{}
		observer, err := newLoopNativeHookObserver(loopStore, notifier, backstop, func() time.Time { return fixedNow })
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}
		notifier.AddLoopTerminalObserver(observer)

		if _, err := notifier.DispatchLoopTerminal(t.Context(), hookspkg.LoopTerminalPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: fixedNow},
			LoopContext: hookspkg.LoopContext{
				LoopRunID:       "child-loop-run",
				ParentLoopRunID: "parent-loop-run",
				WorkspaceID:     "ws-1",
				LoopName:        "daily-review",
			},
			Status: "done",
		}); err != nil {
			t.Fatalf("DispatchLoopTerminal() error = %v", err)
		}

		if got, want := len(loopStore.wakes), 1; got != want {
			t.Fatalf("parent wake calls = %d, want %d", got, want)
		}
		wake := loopStore.wakes[0]
		if got, want := wake.loopRunID, "parent-loop-run"; got != want {
			t.Fatalf("parent wake loop_run_id = %q, want %q", got, want)
		}
		if got, want := wake.idempotencyKey, "loop.coordinator.child_terminal.parent-loop-run.child-loop-run"; got != want {
			t.Fatalf("parent wake key = %q, want %q", got, want)
		}
		if got, want := len(loopStore.promotions), 1; got != want {
			t.Fatalf("promotion calls = %d, want %d", got, want)
		}
		promotion := loopStore.promotions[0]
		if got, want := promotion.workspaceID, "ws-1"; got != want {
			t.Fatalf("promotion workspace_id = %q, want %q", got, want)
		}
		if got, want := promotion.loopName, "daily-review"; got != want {
			t.Fatalf("promotion loop_name = %q, want %q", got, want)
		}
		if got, want := len(backstop.calls), 1; got != want {
			t.Fatalf("backstop calls = %d, want %d", got, want)
		}
	})
}

func TestHooksNotifierCoordinatorTerminalObserversAreFailOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should continue later observers after an earlier panic", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(discardLogger(), time.Now)
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		notifier.AddLoopTerminalObserver(panickingLoopTerminalObserver{})
		recorder := &recordingLoopTerminalObserver{}
		notifier.AddLoopTerminalObserver(recorder)

		if _, err := notifier.DispatchLoopTerminal(t.Context(), hookspkg.LoopTerminalPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: time.Now().UTC()},
			LoopContext: hookspkg.LoopContext{
				LoopRunID:   "loop-run-1",
				WorkspaceID: "ws-1",
				LoopName:    "daily-review",
			},
			Status: "failed",
		}); err != nil {
			t.Fatalf("DispatchLoopTerminal() error = %v", err)
		}
		if !recorder.called {
			t.Fatal("later loop terminal observer did not run after earlier observer panic")
		}
	})
}

type recordingLoopHookStore struct {
	progress       []recordingLoopProgressCall
	wakes          []recordingLoopWakeCall
	promotions     []recordingLoopPromotionCall
	outputStatuses map[string]string
	outputs        []looppkg.GenerationOutput
	runs           map[string]taskpkg.Run
	tasks          map[string]taskpkg.Task
}

type recordingLoopBackstopRunner struct {
	calls []recordingLoopBackstopCall
}

type recordingLoopBackstopCall struct {
	now   time.Time
	actor taskpkg.ActorContext
}

func (r *recordingLoopBackstopRunner) RunLoopCoordinatorBackstop(
	_ context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (int, error) {
	r.calls = append(r.calls, recordingLoopBackstopCall{now: now, actor: actor})
	return 1, nil
}

type recordingLoopProgressCall struct {
	loopRunID string
	at        time.Time
}

type recordingLoopWakeCall struct {
	loopRunID      string
	idempotencyKey string
	origin         taskpkg.Origin
	now            time.Time
}

type recordingLoopPromotionCall struct {
	workspaceID string
	loopName    string
	origin      taskpkg.Origin
	now         time.Time
}

func (r *recordingLoopHookStore) AdvanceLoopRunProgress(
	_ context.Context,
	loopRunID string,
	at time.Time,
) error {
	r.progress = append(r.progress, recordingLoopProgressCall{loopRunID: loopRunID, at: at})
	return nil
}

func (r *recordingLoopHookStore) EnqueueLoopCoordinatorWake(
	_ context.Context,
	loopRunID string,
	idempotencyKey string,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	r.wakes = append(r.wakes, recordingLoopWakeCall{
		loopRunID:      loopRunID,
		idempotencyKey: idempotencyKey,
		origin:         origin,
		now:            now,
	})
	return taskpkg.Run{ID: "run-parent-wake", LoopRunID: loopRunID}, true, nil
}

func (r *recordingLoopHookStore) PromoteOldestQueuedLoopRun(
	_ context.Context,
	workspaceID string,
	loopName string,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	r.promotions = append(r.promotions, recordingLoopPromotionCall{
		workspaceID: workspaceID,
		loopName:    loopName,
		origin:      origin,
		now:         now,
	})
	return taskpkg.Run{ID: "run-promoted", LoopRunID: "queued-loop-run"}, true, nil
}

func (r *recordingLoopHookStore) LookupLoopGenerationOutputStatus(
	_ context.Context,
	loopRunID string,
	taskRunID string,
) (string, bool, error) {
	status, ok := r.outputStatuses[loopRunID+"/"+taskRunID]
	return status, ok, nil
}

func (r *recordingLoopHookStore) ListGenerationOutputs(
	_ context.Context,
	_ looppkg.RunID,
	_ int,
) ([]looppkg.GenerationOutput, error) {
	return append([]looppkg.GenerationOutput(nil), r.outputs...), nil
}

func (r *recordingLoopHookStore) GetTaskRun(_ context.Context, id string) (taskpkg.Run, error) {
	run, ok := r.runs[id]
	if !ok {
		return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
	}
	return run, nil
}

func (r *recordingLoopHookStore) GetTask(_ context.Context, id string) (taskpkg.Task, error) {
	taskRecord, ok := r.tasks[id]
	if !ok {
		return taskpkg.Task{}, taskpkg.ErrTaskNotFound
	}
	return taskRecord, nil
}

type panickingLoopTerminalObserver struct{}

func (panickingLoopTerminalObserver) OnLoopTerminal(
	context.Context,
	hookspkg.LoopTerminalPayload,
) error {
	panic("observer failed")
}

type recordingLoopTerminalObserver struct {
	called bool
}

func (r *recordingLoopTerminalObserver) OnLoopTerminal(
	context.Context,
	hookspkg.LoopTerminalPayload,
) error {
	r.called = true
	return nil
}

type spyLifecycleObserver struct {
	created []*session.Session
	stopped []*session.Session
}

type recordingSubprocessHealthRuntime struct {
	observations []session.SubprocessHealthSnapshot
	stopped      []*session.Session
}

func (r *recordingSubprocessHealthRuntime) OnSubprocessHealth(
	_ context.Context,
	observation session.SubprocessHealthSnapshot,
) {
	r.observations = append(r.observations, observation)
}

func (r *recordingSubprocessHealthRuntime) OnSessionStopped(
	_ context.Context,
	stopped *session.Session,
) {
	r.stopped = append(r.stopped, stopped)
}

func (s *spyLifecycleObserver) OnSessionCreated(_ context.Context, sess *session.Session) {
	s.created = append(s.created, sess)
}

func (s *spyLifecycleObserver) OnSessionStopped(_ context.Context, sess *session.Session) {
	s.stopped = append(s.stopped, sess)
}

type spyDreamRuntime struct {
	calls []string
}

func (s *spyDreamRuntime) EnqueueCheck(reason string, workspaceRef string) {
	s.calls = append(s.calls, reason+":"+workspaceRef)
}

type spyMessagePersistedObserver struct {
	ch chan hookspkg.SessionMessagePersistedPayload
}

func newSpyMessagePersistedObserver() *spyMessagePersistedObserver {
	return &spyMessagePersistedObserver{ch: make(chan hookspkg.SessionMessagePersistedPayload, 1)}
}

func (s *spyMessagePersistedObserver) HandleSessionMessagePersisted(
	_ context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) error {
	s.ch <- payload
	return nil
}

func (s *spyMessagePersistedObserver) wait(t *testing.T) hookspkg.SessionMessagePersistedPayload {
	t.Helper()
	select {
	case payload := <-s.ch:
		return payload
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for memory extractor hook")
		return hookspkg.SessionMessagePersistedPayload{}
	}
}

func marketplaceSkillForTest(registry string, slug string, hash string) *skills.Skill {
	return &skills.Skill{
		Source: skills.SourceMarketplace,
		Meta:   skills.SkillMeta{Name: "marketplace-hook"},
		Provenance: &skills.Provenance{
			Registry: registry,
			Slug:     slug,
			Hash:     hash,
		},
	}
}

func workspaceResolvedForTest(id string, root string) workspacepkg.ResolvedWorkspace {
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      id,
			RootDir: root,
		},
		WorkspaceID: id,
	}
}
