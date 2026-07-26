package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestLoopWatchEventsObserverShouldMatchAndWake(t *testing.T) {
	t.Parallel()

	t.Run("Should enqueue one wake and coalesce the second matching doorbell", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 17, 0, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{
			watchEventsParkedSubscriptionForTest(
				`event.task_id == inputs.target_task_id && event.payload.to_status == "blocked"`,
			),
		})
		backstop := &recordingWatchEventsBackstop{}
		observer := newLoopWatchEventsObserverForTest(t, watchStore, backstop, fixedNow)

		payload := watchEventsTaskStatusPayloadForTest("ws-1", "task-target", "blocked", fixedNow)
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(first) error = %v", err)
		}
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(second) error = %v", err)
		}

		wakes := watchStore.snapshotWakes()
		if got, want := len(wakes), 2; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		if got, want := wakes[0].idempotencyKey, "loop.coordinator.watch_events.loop-run-1.watch_tasks"; got != want {
			t.Fatalf("wake key = %q, want %q", got, want)
		}
		if got, want := backstop.callCount(), 1; got != want {
			t.Fatalf("backstop calls = %d, want %d", got, want)
		}
		if got, want := countWatchEventSummaries(
			watchStore.snapshotSummaries(),
			loopWatchEventsWakeEnqueuedEvent,
		), 2; got != want {
			t.Fatalf("wake_enqueued summaries = %d, want %d", got, want)
		}
	})
}

func TestLoopWatchEventsObserverShouldApplyDoorbellIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject known foreign workspace and allow unknown workspace payloads", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 17, 15, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{
			watchEventsParkedSubscriptionForTest(`event.task_id == inputs.target_task_id`),
		})
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		foreign := watchEventsTaskStatusPayloadForTest("ws-2", "task-target", "blocked", fixedNow)
		if err := observer.OnTaskStatusChanged(t.Context(), foreign); err != nil {
			t.Fatalf("OnTaskStatusChanged(foreign) error = %v", err)
		}
		unknown := watchEventsTaskStatusPayloadForTest("", "task-target", "blocked", fixedNow)
		if err := observer.OnTaskStatusChanged(t.Context(), unknown); err != nil {
			t.Fatalf("OnTaskStatusChanged(unknown) error = %v", err)
		}

		if got, want := len(watchStore.snapshotWakes()), 1; got != want {
			t.Fatalf("wake calls = %d, want only unknown workspace wake", got)
		}
	})

	t.Run("Should degrade nodes filters to kind-only doorbell matches", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 17, 30, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{
			watchEventsParkedSubscriptionForTest(`{"candidate": nodes.previous.output.ok}.candidate == true`),
		})
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		payload := watchEventsTaskStatusPayloadForTest("ws-1", "unrelated-task", "ready", fixedNow)
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(nodes filter) error = %v", err)
		}
		if got, want := len(watchStore.snapshotWakes()), 1; got != want {
			t.Fatalf("wake calls = %d, want kind-only match", got)
		}
	})
}

func TestLoopWatchEventsObserverShouldNormalizeWakeErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should swallow conflict wake errors", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 17, 45, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{watchEventsParkedSubscriptionForTest("")})
		watchStore.setWakeError(taskpkg.ErrConflict)
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		err := observer.OnTaskStatusChanged(
			t.Context(),
			watchEventsTaskStatusPayloadForTest("ws-1", "task-target", "blocked", fixedNow),
		)
		if err != nil {
			t.Fatalf("OnTaskStatusChanged(conflict) error = %v, want nil", err)
		}
		if got := countWatchEventSummaries(watchStore.snapshotSummaries(), loopWatchEventsWakeErrorEvent); got != 0 {
			t.Fatalf("wake_error summaries = %d, want 0", got)
		}
	})

	t.Run("Should surface unexpected wake errors and emit wake_error", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 18, 0, 0, 0, time.UTC)
		errWake := errors.New("forced wake failure")
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{watchEventsParkedSubscriptionForTest("")})
		watchStore.setWakeError(errWake)
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		err := observer.OnTaskStatusChanged(
			t.Context(),
			watchEventsTaskStatusPayloadForTest("ws-1", "task-target", "blocked", fixedNow),
		)
		if !errors.Is(err, errWake) {
			t.Fatalf("OnTaskStatusChanged(unexpected) error = %v, want %v", err, errWake)
		}
		if got, want := countWatchEventSummaries(
			watchStore.snapshotSummaries(),
			loopWatchEventsWakeErrorEvent,
		), 1; got != want {
			t.Fatalf("wake_error summaries = %d, want %d", got, want)
		}
	})
}

func TestLoopWatchEventsObserverShouldRefreshIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should hydrate from parked rows and remove entries after terminal refresh", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 18, 15, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		payload := watchEventsTaskStatusPayloadForTest("ws-1", "task-target", "blocked", fixedNow)
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(empty index) error = %v", err)
		}
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{watchEventsParkedSubscriptionForTest("")})
		coordinatorRunKind := taskpkg.RunKindCoordinator.String()
		if err := observer.OnTaskRunTerminal(t.Context(), hookspkg.TaskRunLeasePayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				WorkspaceID: "ws-1",
				TaskID:      "coordinator-task",
				RunID:       "coordinator-run",
				RunKind:     &coordinatorRunKind,
				LoopRunID:   "loop-run-1",
				RunStatus:   taskpkg.TaskRunStatusCompleted.String(),
			},
		}); err != nil {
			t.Fatalf("OnTaskRunTerminal(park refresh) error = %v", err)
		}
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(hydrated) error = %v", err)
		}
		if got, want := len(watchStore.snapshotWakes()), 1; got != want {
			t.Fatalf("wake calls after hydrate = %d, want %d", got, want)
		}

		watchStore.setParked(nil)
		if err := observer.OnTaskRunTerminal(t.Context(), hookspkg.TaskRunLeasePayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				WorkspaceID: "ws-1",
				TaskID:      "coordinator-task",
				RunID:       "coordinator-run-2",
				RunKind:     &coordinatorRunKind,
				LoopRunID:   "loop-run-1",
				RunStatus:   taskpkg.TaskRunStatusCompleted.String(),
			},
		}); err != nil {
			t.Fatalf("OnTaskRunTerminal(unpark refresh) error = %v", err)
		}
		if err := observer.OnTaskStatusChanged(t.Context(), payload); err != nil {
			t.Fatalf("OnTaskStatusChanged(removed index) error = %v", err)
		}
		if got, want := len(watchStore.snapshotWakes()), 1; got != want {
			t.Fatalf("wake calls after removal = %d, want unchanged %d", got, want)
		}
		globalCalls, scopedCalls := watchStore.snapshotListCalls()
		if globalCalls != 1 || scopedCalls != 2 {
			t.Fatalf("parked index reads = global %d scoped %d, want global 1 scoped 2", globalCalls, scopedCalls)
		}
	})

	t.Run("Should not refresh the parked index for worker terminals", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 18, 20, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{watchEventsParkedSubscriptionForTest("")})
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)
		workerRunKind := taskpkg.RunKindWorker.String()

		if err := observer.OnTaskRunTerminal(t.Context(), hookspkg.TaskRunLeasePayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				WorkspaceID: "ws-1",
				RunID:       "worker-run",
				RunKind:     &workerRunKind,
				LoopRunID:   "loop-run-1",
			},
		}); err != nil {
			t.Fatalf("OnTaskRunTerminal(worker) error = %v", err)
		}
		globalCalls, scopedCalls := watchStore.snapshotListCalls()
		if globalCalls != 1 || scopedCalls != 0 {
			t.Fatalf(
				"parked index reads = global %d scoped %d, want initial global read only",
				globalCalls,
				scopedCalls,
			)
		}
	})

	t.Run("Should remove a terminal loop from memory without reading the store", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 8, 18, 25, 0, 0, time.UTC)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked([]looppkg.ParkedWatchEventSubscription{watchEventsParkedSubscriptionForTest("")})
		observer := newLoopWatchEventsObserverForTest(t, watchStore, &recordingWatchEventsBackstop{}, fixedNow)

		if err := observer.OnLoopTerminal(t.Context(), hookspkg.LoopTerminalPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: fixedNow},
			LoopContext: hookspkg.LoopContext{WorkspaceID: "ws-1", LoopRunID: "loop-run-1"},
		}); err != nil {
			t.Fatalf("OnLoopTerminal() error = %v", err)
		}
		if got := observer.subscriptionsForKind(hookspkg.HookTaskStatusChanged); len(got) != 0 {
			t.Fatalf("task status subscriptions = %#v, want terminal loop removed", got)
		}
		globalCalls, scopedCalls := watchStore.snapshotListCalls()
		if globalCalls != 1 || scopedCalls != 0 {
			t.Fatalf(
				"parked index reads = global %d scoped %d, want initial global read only",
				globalCalls,
				scopedCalls,
			)
		}
	})
}

func TestLoopWatchEventsObserverShouldWakeForTypedWatchEvents(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, 7, 8, 18, 30, 0, 0, time.UTC)
	cases := []struct {
		name     string
		kind     hookspkg.HookEvent
		stream   string
		filter   string
		dispatch func(context.Context, *loopWatchEventsObserver) error
	}{
		{
			name:   "Should wake from a task blocked payload",
			kind:   hookspkg.HookTaskBlocked,
			stream: looppkg.WatchEventsTaskStream,
			filter: `event.payload.kind == "dependency" && event.payload.details.owner == "planner"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnTaskBlocked(ctx, hookspkg.TaskBlockedPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskBlocked, Timestamp: fixedNow},
					TaskContext: watchEventsTaskContextForTest("task-target"),
					BlockID:     "block-1",
					Kind:        "dependency",
					Reason:      "waiting",
					Details:     json.RawMessage(`{"owner":"planner"}`),
				})
			},
		},
		{
			name:   "Should wake from a task unblocked payload",
			kind:   hookspkg.HookTaskUnblocked,
			stream: looppkg.WatchEventsTaskStream,
			filter: `event.payload.clear_note == "resolved" && event.payload.cleared_at != ""`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnTaskUnblocked(ctx, hookspkg.TaskUnblockedPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskUnblocked, Timestamp: fixedNow},
					TaskContext: watchEventsTaskContextForTest("task-target"),
					BlockID:     "block-1",
					Kind:        "dependency",
					Reason:      "waiting",
					ClearedAt:   fixedNow,
					ClearNote:   "resolved",
				})
			},
		},
		{
			name:   "Should wake from a task needs-attention payload",
			kind:   hookspkg.HookTaskNeedsAttention,
			stream: looppkg.WatchEventsTaskStream,
			filter: `event.payload.reason == "stalled" && event.payload.at != ""`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnTaskNeedsAttention(ctx, hookspkg.TaskNeedsAttentionPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskNeedsAttention, Timestamp: fixedNow},
					TaskContext: watchEventsTaskContextForTest("task-target"),
					Reason:      "stalled",
					Note:        "needs review",
					At:          fixedNow,
				})
			},
		},
		{
			name:   "Should wake from a task recovered payload",
			kind:   hookspkg.HookTaskRecovered,
			stream: looppkg.WatchEventsTaskStream,
			filter: `event.payload.note == "resolved"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnTaskRecovered(ctx, hookspkg.TaskRecoveredPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRecovered},
					TaskContext: watchEventsTaskContextForTest("task-target"),
					Reason:      "manual_clear",
					Note:        "resolved",
				})
			},
		},
		{
			name:   "Should wake from a failed task-run terminal payload",
			kind:   hookspkg.HookTaskRunFailed,
			stream: looppkg.WatchEventsTaskStream,
			filter: `event.channel == "chan-1" && event.payload.error == "boom" && event.loop_run_id == "loop-run-source"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnTaskRunTerminal(ctx, hookspkg.TaskRunLeasePayload{
					PayloadBase: hookspkg.PayloadBase{Timestamp: fixedNow},
					TaskRunContext: hookspkg.TaskRunContext{
						WorkspaceID:                  "ws-1",
						TaskID:                       "task-target",
						RunID:                        "run-failed",
						RunStatus:                    taskpkg.TaskRunStatusFailed.String(),
						LoopRunID:                    "loop-run-source",
						SessionID:                    "session-1",
						ResolvedNetworkParticipation: daemonTestLiveParticipationPtr("ws-1", "chan-1"),
						Error:                        "boom",
					},
					PreviousRunStatus: "claimed",
					PreviousSessionID: "session-old",
					RecoveryAction:    "none",
					RecoveryReason:    "none",
				})
			},
		},
		{
			name:   "Should wake from a loop terminal payload",
			kind:   hookspkg.HookLoopTerminal,
			stream: looppkg.WatchEventsLoopStream,
			filter: `event.channel == "chan-1" && event.payload.details.done == true && event.loop_name == "delivery"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnLoopTerminal(ctx, hookspkg.LoopTerminalPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: fixedNow},
					LoopContext: watchEventsLoopContextForTest("terminal"),
					Status:      "done",
					Cause:       "completed",
					ReasonCode:  "ok",
					Details:     json.RawMessage(`{"done":true}`),
				})
			},
		},
		{
			name:   "Should wake from a failed loop node terminal payload",
			kind:   hookspkg.HookLoopNodeTerminal,
			stream: looppkg.WatchEventsLoopStream,
			filter: `event.channel == "chan-1" && event.payload.node_id == "node-1" && event.payload.error == "boom"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnLoopNodeTerminal(ctx, hookspkg.LoopNodeTerminalPayload{
					PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopNodeTerminal, Timestamp: fixedNow},
					LoopContext: watchEventsLoopContextForTest("node-1"),
					TaskStatus:  "blocked",
					RunStatus:   taskpkg.TaskRunStatusFailed.String(),
					Error:       "boom",
					Details:     json.RawMessage(`{"attempt":"first"}`),
				})
			},
		},
		{
			name:   "Should wake from an automation run completion payload",
			kind:   hookspkg.HookAutomationRunCompleted,
			stream: looppkg.WatchEventsAutomationStream,
			filter: `event.run_id == "run-auto" && event.payload.job_id == "job-auto"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnAutomationRunCompleted(ctx, hookspkg.AutomationRunCompletedPayload{
					RunID:       "run-auto",
					JobID:       "job-auto",
					AgentName:   "researcher",
					WorkspaceID: "ws-1",
					SessionID:   "sess-auto",
					Attempt:     1,
					DurationMS:  42,
				})
			},
		},
		{
			name:   "Should wake from a network work transition payload",
			kind:   hookspkg.HookNetworkWorkTransitioned,
			stream: looppkg.WatchEventsNetworkStream,
			filter: `event.channel == "builders" && event.payload.work_state == "working"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnNetworkWorkTransitioned(ctx, hookspkg.NetworkPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookNetworkWorkTransitioned,
						Timestamp: fixedNow,
					},
					WorkspaceID: "ws-1",
					SessionID:   "sess-network",
					Channel:     "builders",
					Surface:     "thread",
					ThreadID:    "thread-1",
					MessageID:   "msg-1",
					Kind:        "trace",
					Direction:   "received",
					WorkID:      "work-1",
					WorkState:   "working",
					PeerFrom:    "reviewer.sess-1",
					PeerTo:      "coder.sess-1",
				})
			},
		},
		{
			name:   "Should wake from a stopped coordinator payload",
			kind:   hookspkg.HookCoordinatorStopped,
			stream: looppkg.WatchEventsObserveStream,
			filter: `event.session_id == "coord-1" && event.payload.stop_reason == "complete"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnCoordinatorStopped(ctx, hookspkg.CoordinatorStoppedPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookCoordinatorStopped,
						Timestamp: fixedNow,
					},
					CoordinatorContext: hookspkg.CoordinatorContext{
						WorkspaceID:          "ws-1",
						AgentName:            "planner",
						CoordinatorSessionID: "coord-1",
						TaskID:               "task-target",
						RunID:                "run-coord",
						WorkflowID:           "workflow-1",
						Provider:             "native",
					},
					DecisionKind: "terminal",
					Decision:     "stop",
					StopReason:   "complete",
				})
			},
		},
		{
			name:   "Should wake from a spawned coordinator payload",
			kind:   hookspkg.HookCoordinatorSpawned,
			stream: looppkg.WatchEventsObserveStream,
			filter: `event.session_id == "coord-1" && event.payload.decision_kind == "spawn"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnCoordinatorSpawned(ctx, hookspkg.CoordinatorSpawnedPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookCoordinatorSpawned,
						Timestamp: fixedNow,
					},
					CoordinatorContext: hookspkg.CoordinatorContext{
						WorkspaceID:          "ws-1",
						AgentName:            "planner",
						CoordinatorSessionID: "coord-1",
						TaskID:               "task-target",
						RunID:                "run-coord",
						WorkflowID:           "workflow-1",
						Provider:             "native",
					},
					DecisionKind: "spawn",
					Decision:     "launch",
				})
			},
		},
		{
			name:   "Should wake from a coordinator decision payload",
			kind:   hookspkg.HookCoordinatorDecision,
			stream: looppkg.WatchEventsObserveStream,
			filter: `event.session_id == "coord-1" && event.payload.decision == "continue"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnCoordinatorDecision(ctx, hookspkg.CoordinatorDecisionPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookCoordinatorDecision,
						Timestamp: fixedNow,
					},
					CoordinatorContext: hookspkg.CoordinatorContext{
						WorkspaceID:          "ws-1",
						AgentName:            "planner",
						CoordinatorSessionID: "coord-1",
						TaskID:               "task-target",
						RunID:                "run-coord",
						WorkflowID:           "workflow-1",
						Provider:             "native",
					},
					DecisionKind: "next",
					Decision:     "continue",
				})
			},
		},
		{
			name:   "Should wake from a failed coordinator payload",
			kind:   hookspkg.HookCoordinatorFailed,
			stream: looppkg.WatchEventsObserveStream,
			filter: `event.session_id == "coord-1" && event.payload.error == "boom"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnCoordinatorFailed(ctx, hookspkg.CoordinatorFailedPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookCoordinatorFailed,
						Timestamp: fixedNow,
					},
					CoordinatorContext: hookspkg.CoordinatorContext{
						WorkspaceID:          "ws-1",
						AgentName:            "planner",
						CoordinatorSessionID: "coord-1",
						TaskID:               "task-target",
						RunID:                "run-coord",
						WorkflowID:           "workflow-1",
						Provider:             "native",
					},
					DecisionKind: "terminal",
					Decision:     "fail",
					Error:        "boom",
				})
			},
		},
		{
			name:   "Should wake from a content-excluded event post-record payload",
			kind:   hookspkg.HookEventPostRecord,
			stream: looppkg.WatchEventsSessionStreamForSession("sess-hot"),
			filter: `event.session_id == "sess-hot" && event.payload.record_type == "agent_message"`,
			dispatch: func(ctx context.Context, observer *loopWatchEventsObserver) error {
				return observer.OnEventPostRecord(ctx, hookspkg.EventPostRecordPayload{
					PayloadBase: hookspkg.PayloadBase{
						Event:     hookspkg.HookEventPostRecord,
						Timestamp: fixedNow,
					},
					SessionContext: hookspkg.SessionContext{
						SessionID:   "sess-hot",
						AgentName:   "coder",
						WorkspaceID: "ws-1",
					},
					TurnContext: hookspkg.TurnContext{TurnID: "turn-1"},
					RecordType:  "agent_message",
					Sequence:    42,
					Content:     json.RawMessage(`{"secret":"do not leak"}`),
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			watchStore := newRecordingLoopWatchEventsStore()
			watchStore.setParked([]looppkg.ParkedWatchEventSubscription{
				watchEventsParkedSubscriptionForKindForTest(tc.kind, tc.stream, tc.filter),
			})
			observer := newLoopWatchEventsObserverForTest(
				t,
				watchStore,
				&recordingWatchEventsBackstop{},
				fixedNow,
			)

			if err := tc.dispatch(t.Context(), observer); err != nil {
				t.Fatalf("dispatch typed watch event error = %v", err)
			}

			if got, want := len(watchStore.snapshotWakes()), 1; got != want {
				t.Fatalf("wake calls = %d, want %d", got, want)
			}
			if got, want := countWatchEventSummaries(
				watchStore.snapshotSummaries(),
				loopWatchEventsMatchedEvent,
			), 1; got != want {
				t.Fatalf("matched summaries = %d, want %d", got, want)
			}
		})
	}
}

func TestHooksNotifierWatchEventsObserversShouldFailOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should continue later task status observers after an earlier panic", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(discardLogger(), time.Now)
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		notifier.AddTaskStatusChangedObserver(panickingTaskStatusObserver{})
		recorder := &recordingTaskStatusObserver{}
		notifier.AddTaskStatusChangedObserver(recorder)

		if _, err := notifier.DispatchTaskStatusChanged(t.Context(), hookspkg.TaskStatusChangedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskStatusChanged, Timestamp: time.Now().UTC()},
			TaskContext: hookspkg.TaskContext{
				WorkspaceID: "ws-1",
				TaskID:      "task-1",
			},
			FromStatus: "pending",
			ToStatus:   "blocked",
		}); err != nil {
			t.Fatalf("DispatchTaskStatusChanged() error = %v", err)
		}
		if !recorder.called {
			t.Fatal("later task status observer did not run after earlier observer panic")
		}
	})

	t.Run("Should continue later task lifecycle observers after an earlier panic", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(discardLogger(), time.Now)
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		notifier.AddTaskLifecycleWatchObserver(panickingTaskLifecycleObserver{})
		recorder := &recordingTaskLifecycleObserver{}
		notifier.AddTaskLifecycleWatchObserver(recorder)

		if _, err := notifier.DispatchTaskBlocked(t.Context(), hookspkg.TaskBlockedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskBlocked, Timestamp: time.Now().UTC()},
			TaskContext: watchEventsTaskContextForTest("task-1"),
			BlockID:     "block-1",
			Kind:        "dependency",
			Reason:      "waiting",
		}); err != nil {
			t.Fatalf("DispatchTaskBlocked() error = %v", err)
		}
		if !recorder.blocked {
			t.Fatal("later task lifecycle observer did not run after earlier observer panic")
		}
	})

	t.Run("Should notify automation and network watch observers from typed dispatch", func(t *testing.T) {
		t.Parallel()

		notifier := newHooksNotifier(discardLogger(), time.Now)
		notifier.setRuntime(&fakeHookRuntime{}, nil)
		automationRecorder := &recordingAutomationRunWatchObserver{}
		networkRecorder := &recordingNetworkWatchObserver{}
		notifier.AddAutomationRunWatchObserver(automationRecorder)
		notifier.AddNetworkWatchObserver(networkRecorder)

		if _, err := notifier.DispatchAutomationRunCompleted(t.Context(), hookspkg.AutomationRunCompletedPayload{
			RunID:       "run-auto",
			WorkspaceID: "ws-1",
		}); err != nil {
			t.Fatalf("DispatchAutomationRunCompleted() error = %v", err)
		}
		if _, err := notifier.DispatchNetworkWorkTransitioned(t.Context(), hookspkg.NetworkPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookNetworkWorkTransitioned, Timestamp: time.Now().UTC()},
			WorkspaceID: "ws-1",
			MessageID:   "msg-1",
			WorkID:      "work-1",
			WorkState:   "working",
		}); err != nil {
			t.Fatalf("DispatchNetworkWorkTransitioned() error = %v", err)
		}

		if !automationRecorder.completed {
			t.Fatal("automation run observer was not called")
		}
		if !networkRecorder.workTransitioned {
			t.Fatal("network work observer was not called")
		}
	})
}

func newLoopWatchEventsObserverForTest(
	t *testing.T,
	watchStore *recordingLoopWatchEventsStore,
	backstop *recordingWatchEventsBackstop,
	now time.Time,
) *loopWatchEventsObserver {
	t.Helper()
	observer, err := newLoopWatchEventsObserver(watchStore, backstop, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newLoopWatchEventsObserver() error = %v", err)
	}
	if err := observer.Hydrate(t.Context()); err != nil {
		t.Fatalf("Hydrate() error = %v", err)
	}
	return observer
}

func watchEventsParkedSubscriptionForTest(filter string) looppkg.ParkedWatchEventSubscription {
	return watchEventsParkedSubscriptionForKindForTest(
		hookspkg.HookTaskStatusChanged,
		looppkg.WatchEventsTaskStream,
		filter,
	)
}

func watchEventsParkedSubscriptionForKindForTest(
	kind hookspkg.HookEvent,
	stream string,
	filter string,
) looppkg.ParkedWatchEventSubscription {
	return looppkg.ParkedWatchEventSubscription{
		WorkspaceID: "ws-1",
		LoopRunID:   "loop-run-1",
		LoopName:    "delivery",
		NodeID:      "watch_tasks",
		Generation:  1,
		Inputs:      map[string]any{"target_task_id": "task-target"},
		Subscriptions: []watchpkg.EventSubscriptionRef{{
			Kind:   string(kind),
			Filter: filter,
		}},
		Cursors:   map[string]int64{stream: 0},
		Contracts: watchEventsContractsForKindForTest(kind),
	}
}

func watchEventsContractsForKindForTest(
	kind hookspkg.HookEvent,
) map[hookspkg.HookEvent]looppkg.WatchEventsContract {
	contract, ok := looppkg.SupportedWatchEvents()[kind]
	if !ok {
		panic("unsupported watch-events contract in test fixture: " + string(kind))
	}
	return map[hookspkg.HookEvent]looppkg.WatchEventsContract{kind: contract}
}

func watchEventsTaskContextForTest(taskID string) hookspkg.TaskContext {
	return hookspkg.TaskContext{
		WorkspaceID:  "ws-1",
		TaskID:       taskID,
		ParentTaskID: "parent-task",
	}
}

func watchEventsLoopContextForTest(nodeID string) hookspkg.LoopContext {
	return hookspkg.LoopContext{
		WorkspaceID:                  "ws-1",
		LoopRunID:                    "loop-run-source",
		LoopName:                     "delivery",
		Generation:                   1,
		TaskID:                       "task-target",
		RunID:                        "run-loop-node",
		NodeID:                       nodeID,
		SessionID:                    "session-1",
		ResolvedNetworkParticipation: daemonTestLiveParticipationPtr("ws-1", "chan-1"),
	}
}

func watchEventsTaskStatusPayloadForTest(
	workspaceID string,
	taskID string,
	toStatus string,
	at time.Time,
) hookspkg.TaskStatusChangedPayload {
	return hookspkg.TaskStatusChangedPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskStatusChanged, Timestamp: at},
		TaskContext: hookspkg.TaskContext{
			WorkspaceID:  workspaceID,
			TaskID:       taskID,
			ParentTaskID: "parent-task",
		},
		FromStatus: "pending",
		ToStatus:   toStatus,
	}
}

type recordingLoopWatchEventsStore struct {
	mu              sync.Mutex
	parked          []looppkg.ParkedWatchEventSubscription
	wakes           []recordingWatchEventsWake
	summaries       []store.EventSummary
	wakeErr         error
	seenKeys        map[string]struct{}
	globalListCalls int
	scopedListCalls int
}

type recordingWatchEventsWake struct {
	loopRunID      string
	idempotencyKey string
	origin         taskpkg.Origin
	now            time.Time
}

func newRecordingLoopWatchEventsStore() *recordingLoopWatchEventsStore {
	return &recordingLoopWatchEventsStore{seenKeys: map[string]struct{}{}}
}

func (s *recordingLoopWatchEventsStore) setParked(entries []looppkg.ParkedWatchEventSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parked = cloneParkedWatchEventSubscriptionsForTest(entries)
}

func (s *recordingLoopWatchEventsStore) setWakeError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wakeErr = err
}

func (s *recordingLoopWatchEventsStore) ListParkedWatchEventSubscriptions(
	context.Context,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.globalListCalls++
	return cloneParkedWatchEventSubscriptionsForTest(s.parked), nil
}

func (s *recordingLoopWatchEventsStore) ListParkedWatchEventSubscriptionsForLoopRun(
	_ context.Context,
	loopRunID string,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scopedListCalls++
	entries := make([]looppkg.ParkedWatchEventSubscription, 0)
	for _, entry := range s.parked {
		if entry.LoopRunID == loopRunID {
			entries = append(entries, entry)
		}
	}
	return cloneParkedWatchEventSubscriptionsForTest(entries), nil
}

func (s *recordingLoopWatchEventsStore) snapshotListCalls() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.globalListCalls, s.scopedListCalls
}

func (s *recordingLoopWatchEventsStore) EnqueueLoopCoordinatorWake(
	_ context.Context,
	loopRunID string,
	idempotencyKey string,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wakes = append(s.wakes, recordingWatchEventsWake{
		loopRunID:      loopRunID,
		idempotencyKey: idempotencyKey,
		origin:         origin,
		now:            now,
	})
	if s.wakeErr != nil {
		return taskpkg.Run{}, false, s.wakeErr
	}
	if _, ok := s.seenKeys[idempotencyKey]; ok {
		return taskpkg.Run{ID: "run-coalesced", LoopRunID: loopRunID}, false, nil
	}
	s.seenKeys[idempotencyKey] = struct{}{}
	return taskpkg.Run{ID: "run-watch-events", LoopRunID: loopRunID}, true, nil
}

func (s *recordingLoopWatchEventsStore) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *recordingLoopWatchEventsStore) snapshotWakes() []recordingWatchEventsWake {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordingWatchEventsWake(nil), s.wakes...)
}

func (s *recordingLoopWatchEventsStore) snapshotSummaries() []store.EventSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.EventSummary(nil), s.summaries...)
}

type recordingWatchEventsBackstop struct {
	mu    sync.Mutex
	calls int
}

func (r *recordingWatchEventsBackstop) RunLoopCoordinatorBackstop(
	context.Context,
	time.Time,
	taskpkg.ActorContext,
) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return 1, nil
}

func (r *recordingWatchEventsBackstop) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type panickingTaskStatusObserver struct{}

func (panickingTaskStatusObserver) OnTaskStatusChanged(
	context.Context,
	hookspkg.TaskStatusChangedPayload,
) error {
	panic("task status observer failed")
}

type panickingTaskLifecycleObserver struct{}

func (panickingTaskLifecycleObserver) OnTaskBlocked(context.Context, hookspkg.TaskBlockedPayload) error {
	panic("task blocked observer failed")
}

func (panickingTaskLifecycleObserver) OnTaskUnblocked(context.Context, hookspkg.TaskUnblockedPayload) error {
	panic("task unblocked observer failed")
}

func (panickingTaskLifecycleObserver) OnTaskNeedsAttention(
	context.Context,
	hookspkg.TaskNeedsAttentionPayload,
) error {
	panic("task needs-attention observer failed")
}

func (panickingTaskLifecycleObserver) OnTaskRecovered(context.Context, hookspkg.TaskRecoveredPayload) error {
	panic("task recovered observer failed")
}

type recordingTaskStatusObserver struct {
	called bool
}

func (r *recordingTaskStatusObserver) OnTaskStatusChanged(
	context.Context,
	hookspkg.TaskStatusChangedPayload,
) error {
	r.called = true
	return nil
}

type recordingTaskLifecycleObserver struct {
	blocked bool
}

func (r *recordingTaskLifecycleObserver) OnTaskBlocked(context.Context, hookspkg.TaskBlockedPayload) error {
	r.blocked = true
	return nil
}

func (r *recordingTaskLifecycleObserver) OnTaskUnblocked(context.Context, hookspkg.TaskUnblockedPayload) error {
	return nil
}

func (r *recordingTaskLifecycleObserver) OnTaskNeedsAttention(
	context.Context,
	hookspkg.TaskNeedsAttentionPayload,
) error {
	return nil
}

func (r *recordingTaskLifecycleObserver) OnTaskRecovered(context.Context, hookspkg.TaskRecoveredPayload) error {
	return nil
}

type recordingAutomationRunWatchObserver struct {
	completed bool
	failed    bool
}

func (r *recordingAutomationRunWatchObserver) OnAutomationRunCompleted(
	context.Context,
	hookspkg.AutomationRunCompletedPayload,
) error {
	r.completed = true
	return nil
}

func (r *recordingAutomationRunWatchObserver) OnAutomationRunFailed(
	context.Context,
	hookspkg.AutomationRunFailedPayload,
) error {
	r.failed = true
	return nil
}

type recordingNetworkWatchObserver struct {
	threadOpened     bool
	directOpened     bool
	messagePersisted bool
	workOpened       bool
	workTransitioned bool
	workClosed       bool
}

func (r *recordingNetworkWatchObserver) OnNetworkThreadOpened(
	context.Context,
	hookspkg.NetworkThreadOpenedPayload,
) error {
	r.threadOpened = true
	return nil
}

func (r *recordingNetworkWatchObserver) OnNetworkDirectRoomOpened(
	context.Context,
	hookspkg.NetworkDirectRoomOpenedPayload,
) error {
	r.directOpened = true
	return nil
}

func (r *recordingNetworkWatchObserver) OnNetworkMessagePersisted(
	context.Context,
	hookspkg.NetworkMessagePersistedPayload,
) error {
	r.messagePersisted = true
	return nil
}

func (r *recordingNetworkWatchObserver) OnNetworkWorkOpened(
	context.Context,
	hookspkg.NetworkWorkOpenedPayload,
) error {
	r.workOpened = true
	return nil
}

func (r *recordingNetworkWatchObserver) OnNetworkWorkTransitioned(
	context.Context,
	hookspkg.NetworkWorkTransitionedPayload,
) error {
	r.workTransitioned = true
	return nil
}

func (r *recordingNetworkWatchObserver) OnNetworkWorkClosed(
	context.Context,
	hookspkg.NetworkWorkClosedPayload,
) error {
	r.workClosed = true
	return nil
}

func countWatchEventSummaries(summaries []store.EventSummary, eventType string) int {
	count := 0
	for _, summary := range summaries {
		if summary.Type == eventType {
			count++
		}
	}
	return count
}

func cloneParkedWatchEventSubscriptionsForTest(
	entries []looppkg.ParkedWatchEventSubscription,
) []looppkg.ParkedWatchEventSubscription {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]looppkg.ParkedWatchEventSubscription, 0, len(entries))
	for _, entry := range entries {
		cloned = append(cloned, cloneParkedWatchEventSubscription(entry))
	}
	return cloned
}
