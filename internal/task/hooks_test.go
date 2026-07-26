package task

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

func TestNoopRunHookDispatcherPreservesRunLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve run lifecycle with the no-op dispatcher", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "No-op hook task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claimed, err := claimExactRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("claimExactRunForTest() error = %v", err)
		}
		if got, want := claimed.Status, TaskRunStatusClaimed; got != want {
			t.Fatalf("claimed.Status = %q, want %q", got, want)
		}

		events, err := store.ListTaskEvents(
			context.Background(),
			EventQuery{TaskID: taskRecord.ID, RunID: run.ID},
		)
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if !containsEventType(events, taskEventRunEnqueued) || !containsEventType(events, taskEventRunClaimed) {
			t.Fatalf("event types = %#v, want enqueue and claim audit events", sortedEventTypes(events))
		}

		statusPayload := hookspkg.TaskStatusChangedPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskStatusChanged},
			TaskContext: hookspkg.TaskContext{TaskID: taskRecord.ID},
			FromStatus:  string(TaskStatusReady),
			ToStatus:    string(TaskStatusInProgress),
		}
		gotStatus, err := noopTaskRunHooks{}.DispatchTaskStatusChanged(context.Background(), statusPayload)
		if err != nil {
			t.Fatalf("DispatchTaskStatusChanged() error = %v", err)
		}
		if gotStatus != statusPayload {
			t.Fatalf("DispatchTaskStatusChanged() = %#v, want %#v", gotStatus, statusPayload)
		}
	})
}

func TestTaskStatusChangedHookShouldReflectOnlyRealTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore normalized no-op status changes", func(t *testing.T) {
		t.Parallel()

		dispatches := 0
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			statusChanged: func(
				_ context.Context,
				payload hookspkg.TaskStatusChangedPayload,
			) (hookspkg.TaskStatusChangedPayload, error) {
				dispatches++
				return payload, nil
			},
		}))
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "No-op status hook task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTaskStatusChanged(
			context.Background(),
			*taskRecord,
			TaskStatusPending,
			Status(" pending "),
			actor,
		)

		if dispatches != 0 {
			t.Fatalf("status_changed dispatches = %d, want 0 for normalized no-op", dispatches)
		}
	})

	t.Run("Should dispatch exactly once for a real status transition", func(t *testing.T) {
		t.Parallel()

		dispatches := make([]hookspkg.TaskStatusChangedPayload, 0, 1)
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			statusChanged: func(
				_ context.Context,
				payload hookspkg.TaskStatusChangedPayload,
			) (hookspkg.TaskStatusChangedPayload, error) {
				dispatches = append(dispatches, payload)
				return payload, nil
			},
		}))
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Real status hook task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTaskStatusChanged(
			context.Background(),
			*taskRecord,
			TaskStatusPending,
			TaskStatusInProgress,
			actor,
		)

		if got, want := len(dispatches), 1; got != want {
			t.Fatalf("status_changed dispatches = %d, want %d", got, want)
		}
		if got := dispatches[0]; got.FromStatus != string(TaskStatusPending) ||
			got.ToStatus != string(TaskStatusInProgress) {
			t.Fatalf("status_changed payload = %#v, want pending -> in_progress", got)
		}
	})
}

func TestTaskRunPreClaimHookDenialPreservesQueuedRun(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve a queued run after pre-claim denial", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			preClaim: func(
				_ context.Context,
				payload hookspkg.TaskRunPreClaimPayload,
			) (hookspkg.TaskRunPreClaimPayload, error) {
				payload.Denied = true
				payload.DenyReason = "agent lacks reviewer capability"
				return payload, nil
			},
		}))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Denied claim task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		_, err = claimExactRunForTest(context.Background(), manager, run.ID, actor)
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("claimExactRunForTest() error = %v, want %v", err, ErrPermissionDenied)
		}

		storedRun, err := store.GetTaskRun(context.Background(), run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, TaskRunStatusQueued; got != want {
			t.Fatalf("storedRun.Status = %q, want %q", got, want)
		}
		events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID, RunID: run.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if containsEventType(events, taskEventRunClaimed) {
			t.Fatalf(
				"event types = %#v, want no claimed audit event after denied pre-claim hook",
				sortedEventTypes(events),
			)
		}
	})
}

func TestTaskRunEnqueuedHookIncludesActorAndOrigin(t *testing.T) {
	t.Parallel()
	t.Run("Should include actor and origin in the enqueued hook", func(t *testing.T) {
		t.Parallel()

		var got hookspkg.TaskRunEnqueuedPayload
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			enqueued: func(
				_ context.Context,
				payload hookspkg.TaskRunEnqueuedPayload,
			) (hookspkg.TaskRunEnqueuedPayload, error) {
				got = payload
				return payload, nil
			},
		}))
		actor, err := DeriveHumanActorContext("operator-1", OriginKindCLI, "agh task start")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Hook context task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		execution, err := manager.StartTask(context.Background(), taskRecord.ID, ExecutionRequest{}, actor)
		if err != nil {
			t.Fatalf("StartTask() error = %v", err)
		}
		if got.TaskID != taskRecord.ID || got.RunID != execution.Run.ID {
			t.Fatalf("hook context ids = %#v, want task/run ids", got.TaskRunContext)
		}
		if got.ActorKind != string(ActorKindHuman) || got.ActorID != "operator-1" {
			t.Fatalf("hook actor context = %#v, want operator actor", got.TaskRunContext)
		}
		if got.OriginKind != string(OriginKindCLI) || got.OriginRef != "agh task start" {
			t.Fatalf("hook origin context = %#v, want cli origin", got.TaskRunContext)
		}
	})
}

func TestTaskRunObservationHooksDetachFromCallerCancellation(t *testing.T) {
	t.Parallel()

	var enqueuedCtx context.Context
	var postClaimCtx context.Context
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
		enqueued: func(
			ctx context.Context,
			payload hookspkg.TaskRunEnqueuedPayload,
		) (hookspkg.TaskRunEnqueuedPayload, error) {
			enqueuedCtx = ctx
			return payload, nil
		},
		postClaim: func(
			ctx context.Context,
			payload hookspkg.TaskRunPostClaimPayload,
		) (hookspkg.TaskRunPostClaimPayload, error) {
			postClaimCtx = ctx
			return payload, nil
		},
	}))
	actor := validActorContext()
	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Observation hook context task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	enqueueCtx, cancelEnqueue := context.WithCancel(context.Background())
	run, err := manager.EnqueueRun(enqueueCtx, EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	cancelEnqueue()
	t.Run("Should keep enqueued hook context active", func(t *testing.T) {
		t.Parallel()
		assertContextStillActive(enqueuedCtx, t, "enqueued")
	})

	claimCtx, cancelClaim := context.WithCancel(context.Background())
	if _, err := claimExactRunForTest(claimCtx, manager, run.ID, actor); err != nil {
		t.Fatalf("claimExactRunForTest() error = %v", err)
	}
	cancelClaim()
	t.Run("Should keep post-claim hook context active", func(t *testing.T) {
		t.Parallel()
		assertContextStillActive(postClaimCtx, t, "post-claim")
	})
}

func TestTaskRunPreClaimHookUsesCallerCancellation(t *testing.T) {
	t.Parallel()
	t.Run("Should cancel the pre-claim hook with the caller context", func(t *testing.T) {
		t.Parallel()

		var preClaimCtx context.Context
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			preClaim: func(
				ctx context.Context,
				payload hookspkg.TaskRunPreClaimPayload,
			) (hookspkg.TaskRunPreClaimPayload, error) {
				preClaimCtx = ctx
				return payload, nil
			},
		}))
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Pre-claim hook context task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}

		claimCtx, cancelClaim := context.WithCancel(context.Background())
		if _, err := claimExactRunForTest(claimCtx, manager, run.ID, actor); err != nil {
			t.Fatalf("claimExactRunForTest() error = %v", err)
		}
		cancelClaim()

		if preClaimCtx == nil {
			t.Fatal("pre-claim hook context was not captured")
		}
		select {
		case <-preClaimCtx.Done():
		default:
			t.Fatal("pre-claim hook context was not canceled with caller context")
		}
	})
}

func TestTokenFencedLeaseTransitionsDispatchTaskRunHooks(t *testing.T) {
	t.Parallel()
	t.Run("Should dispatch hooks for token-fenced lease transitions", func(t *testing.T) {
		t.Parallel()

		var events []hookspkg.HookEvent
		record := func(event hookspkg.HookEvent) {
			events = append(events, event)
		}

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			postClaim: func(
				_ context.Context,
				payload hookspkg.TaskRunPostClaimPayload,
			) (hookspkg.TaskRunPostClaimPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			leaseExtended: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseExtendedPayload,
			) (hookspkg.TaskRunLeaseExtendedPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			leaseExpired: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseExpiredPayload,
			) (hookspkg.TaskRunLeaseExpiredPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			recovered: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseRecoveredPayload,
			) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			released: func(
				_ context.Context,
				payload hookspkg.TaskRunReleasedPayload,
			) (hookspkg.TaskRunReleasedPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			completed: func(
				_ context.Context,
				payload hookspkg.TaskRunCompletedPayload,
			) (hookspkg.TaskRunCompletedPayload, error) {
				record(payload.Event)
				return payload, nil
			},
			failed: func(
				_ context.Context,
				payload hookspkg.TaskRunFailedPayload,
			) (hookspkg.TaskRunFailedPayload, error) {
				record(payload.Event)
				return payload, nil
			},
		}))
		actor := validActorContext()
		agent := agentActorContextForTest("sess-hooks", "ws-hooks")
		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		maxAttempts := 4

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeGlobal,
			Title:       "Hooked lease task",
			MaxAttempts: &maxAttempts,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		claimAndRun := func(label string, at time.Time) *ClaimResult {
			t.Helper()
			if _, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			); err != nil {
				t.Fatalf("EnqueueRun(%s) error = %v", label, err)
			}
			claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
				Scope:            ScopeGlobal,
				ClaimerSessionID: "sess-hooks",
				LeaseDuration:    2 * time.Minute,
				Now:              at,
			}, agent)
			if err != nil {
				t.Fatalf("ClaimNextRun(%s) error = %v", label, err)
			}
			return claim
		}

		claim := claimAndRun("complete", now)
		if _, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
			RunID:         claim.Run.ID,
			ClaimToken:    claim.ClaimToken,
			LeaseDuration: 2 * time.Minute,
			Now:           now.Add(10 * time.Second),
		}, agent); err != nil {
			t.Fatalf("HeartbeatRunLease() error = %v", err)
		}
		if _, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result:     RunResult{},
			Now:        now.Add(20 * time.Second),
		}, agent); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}

		claim = claimAndRun("release", now.Add(time.Minute))
		if _, err := manager.ReleaseRunLease(context.Background(), LeaseRelease{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Reason:     "handoff",
			Now:        now.Add(70 * time.Second),
		}, agent); err != nil {
			t.Fatalf("ReleaseRunLease() error = %v", err)
		}

		claim, err = manager.ClaimNextRun(context.Background(), ClaimCriteria{
			Scope:            ScopeGlobal,
			ClaimerSessionID: "sess-hooks",
			LeaseDuration:    2 * time.Minute,
			Now:              now.Add(2 * time.Minute),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(fail) error = %v", err)
		}
		if _, err := manager.FailRunLease(context.Background(), LeaseFailure{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Failure:    RunFailure{Error: "boom"},
			Now:        now.Add(130 * time.Second),
		}, agent); err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}

		expiring := claimAndRun("expire", now.Add(3*time.Minute))
		expiredRun := store.runs[expiring.Run.ID]
		expiredRun.LeaseUntil = now.Add(3*time.Minute - time.Second)
		store.runs[expiring.Run.ID] = expiredRun
		if recovered, err := manager.RecoverExpiredRunLeases(context.Background(), ExpiredLeaseRecovery{
			Now:    now.Add(3 * time.Minute),
			Reason: "orphaned_on_boot",
		}, agent); err != nil {
			t.Fatalf("RecoverExpiredRunLeases() error = %v", err)
		} else if got, want := len(recovered), 1; got != want {
			t.Fatalf("len(RecoverExpiredRunLeases()) = %d, want %d", got, want)
		}

		want := []hookspkg.HookEvent{
			hookspkg.HookTaskRunPostClaim,
			hookspkg.HookTaskRunLeaseExtended,
			hookspkg.HookTaskRunCompleted,
			hookspkg.HookTaskRunPostClaim,
			hookspkg.HookTaskRunReleased,
			hookspkg.HookTaskRunPostClaim,
			hookspkg.HookTaskRunFailed,
			hookspkg.HookTaskRunPostClaim,
			hookspkg.HookTaskRunLeaseExpired,
			hookspkg.HookTaskRunLeaseRecovered,
		}
		if len(events) != len(want) {
			t.Fatalf("events = %#v, want %#v", events, want)
		}
		for idx := range want {
			if events[idx] != want[idx] {
				t.Fatalf("events[%d] = %q, want %q (events=%#v)", idx, events[idx], want[idx], events)
			}
		}
	})

	t.Run("Should skip the recovered hook when the shared attempt budget is exhausted", func(t *testing.T) {
		t.Parallel()

		var expiredCount int
		var recoveredCount int
		var needsAttention hookspkg.TaskNeedsAttentionPayload
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			leaseExpired: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseExpiredPayload,
			) (hookspkg.TaskRunLeaseExpiredPayload, error) {
				expiredCount++
				return payload, nil
			},
			recovered: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseRecoveredPayload,
			) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
				recoveredCount++
				return payload, nil
			},
			needsAttention: func(
				_ context.Context,
				payload hookspkg.TaskNeedsAttentionPayload,
			) (hookspkg.TaskNeedsAttentionPayload, error) {
				needsAttention = payload
				return payload, nil
			},
		}))
		actor := validActorContext()
		agent := agentActorContextForTest("sess-exhausted", "ws-exhausted")
		now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
		maxAttempts := 1
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeGlobal,
			Title:       "Exhausted lease task",
			MaxAttempts: &maxAttempts,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			RunID:            run.ID,
			Scope:            ScopeGlobal,
			ClaimerSessionID: "sess-exhausted",
			LeaseDuration:    time.Minute,
			Now:              now,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}

		results, err := manager.RecoverExpiredRunLeases(context.Background(), ExpiredLeaseRecovery{
			Now:    claim.LeaseUntil.Add(time.Second),
			Reason: "orphaned_on_boot",
		}, actor)
		if err != nil {
			t.Fatalf("RecoverExpiredRunLeases() error = %v", err)
		}
		if len(results) != 1 || !results[0].Exhausted ||
			results[0].Run.Status != TaskRunStatusNeedsAttention {
			t.Fatalf("recovery results = %#v, want one exhausted run", results)
		}
		if expiredCount != 1 || recoveredCount != 0 {
			t.Fatalf("lease hooks = expired:%d recovered:%d, want 1:0", expiredCount, recoveredCount)
		}
		if needsAttention.Reason != LeaseRecoveryExhaustedReason {
			t.Fatalf("needs-attention reason = %q, want %q", needsAttention.Reason, LeaseRecoveryExhaustedReason)
		}
	})
}

func TestTaskLevelHooksDispatchAtServiceCallSites(t *testing.T) {
	t.Parallel()
	t.Run("Should dispatch task-level hooks at owning service call sites", func(t *testing.T) {
		t.Parallel()

		var blocked hookspkg.TaskBlockedPayload
		var unblocked hookspkg.TaskUnblockedPayload
		var needsAttention hookspkg.TaskNeedsAttentionPayload
		var recovered hookspkg.TaskRecoveredPayload
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			blocked: func(
				_ context.Context,
				payload hookspkg.TaskBlockedPayload,
			) (hookspkg.TaskBlockedPayload, error) {
				blocked = payload
				return payload, nil
			},
			unblocked: func(
				_ context.Context,
				payload hookspkg.TaskUnblockedPayload,
			) (hookspkg.TaskUnblockedPayload, error) {
				unblocked = payload
				return payload, nil
			},
			needsAttention: func(
				_ context.Context,
				payload hookspkg.TaskNeedsAttentionPayload,
			) (hookspkg.TaskNeedsAttentionPayload, error) {
				needsAttention = payload
				return payload, nil
			},
			taskRecovered: func(
				_ context.Context,
				payload hookspkg.TaskRecoveredPayload,
			) (hookspkg.TaskRecoveredPayload, error) {
				recovered = payload
				return payload, nil
			},
		}))
		operator := validActorContext()
		agent := agentActorContextForTest("sess-task-hooks", "ws-task-hooks")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-task-hooks",
			Title:       "Task hook dispatch",
			Metadata: json.RawMessage(
				`{"workflow_id":"wf-task-hooks","agent_name":"worker-a"}`,
			),
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			Scope:            ScopeWorkspace,
			WorkspaceID:      taskRecord.WorkspaceID,
			ClaimerSessionID: "sess-task-hooks",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if claim.Run.ID != run.ID {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", claim.Run.ID, run.ID)
		}

		block, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID:     taskRecord.ID,
			Kind:       BlockKindNeedsInput,
			Reason:     "creator clarification required",
			Details:    json.RawMessage(`{"prompt_id":"prompt-1"}`),
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
		}, agent)
		if err != nil {
			t.Fatalf("BlockTask(active run) error = %v", err)
		}
		if got, want := blocked.Event, hookspkg.HookTaskBlocked; got != want {
			t.Fatalf("blocked.Event = %q, want %q", got, want)
		}
		if blocked.TaskID != taskRecord.ID ||
			blocked.WorkspaceID != taskRecord.WorkspaceID ||
			blocked.RunID != run.ID ||
			blocked.WorkflowID != "wf-task-hooks" ||
			blocked.AgentName != "worker-a" ||
			blocked.ReleaseReason != "blocked" {
			t.Fatalf(
				"blocked.TaskContext = %#v, want correlation keys for task/run/workflow/release",
				blocked.TaskContext,
			)
		}
		if blocked.ResolvedNetworkParticipation == nil ||
			*blocked.ResolvedNetworkParticipation != participation.LocalSpec() {
			t.Fatalf(
				"blocked.ResolvedNetworkParticipation = %#v, want canonical Local snapshot",
				blocked.ResolvedNetworkParticipation,
			)
		}
		if blocked.ClaimTokenHash == "" || !VerifyClaimToken(claim.ClaimToken, blocked.ClaimTokenHash) {
			t.Fatalf("blocked.ClaimTokenHash = %q, want hash verifying raw token", blocked.ClaimTokenHash)
		}
		if strings.Contains(blocked.ClaimTokenHash, claim.ClaimToken) {
			t.Fatalf("blocked.ClaimTokenHash leaks raw claim token: %q", blocked.ClaimTokenHash)
		}

		cleared, err := manager.ClearTaskBlock(
			context.Background(),
			taskRecord.ID,
			block.ID,
			"creator answered",
			operator,
		)
		if err != nil {
			t.Fatalf("ClearTaskBlock() error = %v", err)
		}
		if cleared.ID != block.ID {
			t.Fatalf("ClearTaskBlock().ID = %q, want %q", cleared.ID, block.ID)
		}
		if got, want := unblocked.Event, hookspkg.HookTaskUnblocked; got != want {
			t.Fatalf("unblocked.Event = %q, want %q", got, want)
		}
		if unblocked.TaskID != taskRecord.ID ||
			unblocked.WorkspaceID != taskRecord.WorkspaceID ||
			unblocked.BlockID != block.ID ||
			unblocked.ClearNote != "creator answered" ||
			unblocked.ClearedAt.IsZero() {
			t.Fatalf("unblocked payload = %#v, want clear correlation and audit fields", unblocked)
		}
		if got, want := string(unblocked.Details), `{"prompt_id":"prompt-1"}`; got != want {
			t.Fatalf("unblocked.Details = %s, want %s", got, want)
		}

		currentTask, err := store.GetTask(context.Background(), taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		escalatedAt := claimNow.Add(time.Minute)
		manager.dispatchTaskNeedsAttention(
			context.Background(),
			currentTask,
			agent,
			"breaker limit reached",
			escalatedAt,
			nil,
		)
		manager.dispatchTaskRecovered(
			context.Background(),
			currentTask,
			operator,
			"operator resumed",
			escalatedAt.Add(time.Minute),
		)
		if needsAttention.Event != hookspkg.HookTaskNeedsAttention ||
			needsAttention.TaskID != taskRecord.ID ||
			needsAttention.Reason != "breaker limit reached" ||
			!needsAttention.At.Equal(escalatedAt) {
			t.Fatalf("needsAttention payload = %#v, want task.needs_attention correlation", needsAttention)
		}
		if recovered.Event != hookspkg.HookTaskRecovered ||
			recovered.TaskID != taskRecord.ID ||
			recovered.Note != "operator resumed" ||
			!recovered.At.Equal(escalatedAt.Add(time.Minute)) {
			t.Fatalf("recovered payload = %#v, want task.recovered correlation", recovered)
		}

		storedBlocks, err := manager.ListTaskBlocks(context.Background(), taskRecord.ID, true, operator)
		if err != nil {
			t.Fatalf("ListTaskBlocks(includeCleared) error = %v", err)
		}
		assertJSONDoesNotContain(t, "stored task blocks", storedBlocks, claim.ClaimToken)
		assertJSONDoesNotContain(t, "task.blocked payload", blocked, claim.ClaimToken)

		events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		watchRows := make(map[string]bool)
		for _, event := range events {
			switch event.EventType {
			case string(hookspkg.HookTaskBlocked), string(hookspkg.HookTaskUnblocked):
				watchRows[event.EventType] = true
			case string(hookspkg.HookTaskNeedsAttention), string(hookspkg.HookTaskRecovered):
				t.Fatalf(
					"event table contains hook event %q without the owning state mutation",
					event.EventType,
				)
			}
		}
		for _, eventType := range []string{string(hookspkg.HookTaskBlocked), string(hookspkg.HookTaskUnblocked)} {
			if !watchRows[eventType] {
				t.Fatalf("event table missing %q durable row", eventType)
			}
		}
	})
}

func TestTaskHookContextCarriesParentTaskID(t *testing.T) {
	t.Parallel()

	t.Run("Should populate parent task ID through blocked payloads", func(t *testing.T) {
		t.Parallel()

		blockedByTaskID := make(map[string]hookspkg.TaskBlockedPayload)
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			blocked: func(
				_ context.Context,
				payload hookspkg.TaskBlockedPayload,
			) (hookspkg.TaskBlockedPayload, error) {
				blockedByTaskID[payload.TaskID] = payload
				return payload, nil
			},
		}))
		actor := validActorContext()
		parent, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Root task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		child, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-child",
			Title:       "Child task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateChildTask() error = %v", err)
		}

		if _, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID: parent.ID,
			Kind:   BlockKindNeedsInput,
			Reason: "root needs input",
		}, actor); err != nil {
			t.Fatalf("BlockTask(parent) error = %v", err)
		}
		if _, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID: child.ID,
			Kind:   BlockKindNeedsInput,
			Reason: "child needs input",
		}, actor); err != nil {
			t.Fatalf("BlockTask(child) error = %v", err)
		}

		rootPayload := blockedByTaskID[parent.ID]
		if rootPayload.TaskID == "" {
			t.Fatal("root blocked payload missing")
		}
		if rootPayload.ParentTaskID != "" {
			t.Fatalf("root ParentTaskID = %q, want empty", rootPayload.ParentTaskID)
		}
		childPayload := blockedByTaskID[child.ID]
		if childPayload.TaskID == "" {
			t.Fatal("child blocked payload missing")
		}
		if got, want := childPayload.ParentTaskID, parent.ID; got != want {
			t.Fatalf("child ParentTaskID = %q, want %q", got, want)
		}
	})
}

func assertContextStillActive(ctx context.Context, t *testing.T, label string) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("%s hook context was not captured", label)
	}
	select {
	case <-ctx.Done():
		t.Fatalf("%s hook context canceled after caller returned: %v", label, ctx.Err())
	default:
	}
}

func assertJSONDoesNotContain(t *testing.T, label string, value any, forbidden string) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", label, err)
	}
	if strings.Contains(string(encoded), forbidden) {
		t.Fatalf("%s contains forbidden raw claim token %q: %s", label, forbidden, string(encoded))
	}
}

func TestTaskRunHookContextCarriesLoopFilterKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should carry loop filter keys", func(t *testing.T) {
		t.Parallel()
		testTaskRunHookContextCarriesLoopFilterKeys(t)
	})

	t.Run("Should carry taskless wake correlation from the run snapshot", func(t *testing.T) {
		t.Parallel()

		manager := newTaskManagerForTest(t, newInMemoryManagerStore())
		run := Run{
			ID:          "run-wake",
			WorkspaceID: "ws-wake",
			RunKind:     RunKindNetworkWake,
			Status:      TaskRunStatusRunning,
		}
		liveSpec := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-wake",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "wake-channel",
			Source:          participation.SourceExplicitRequest,
		}
		run.SetNetworkState(liveSpec, "wake-1", "sess-target", "owner-1")

		payload := manager.taskRunHookContext(run, Task{}, validActorContext())
		if payload.TaskID != "" || payload.WorkspaceID != "ws-wake" ||
			payload.WakeID != "wake-1" || payload.TargetSessionID != "sess-target" ||
			payload.OwnerKey != "owner-1" {
			t.Fatalf("wake hook context = %#v, want taskless durable correlation", payload)
		}
		if payload.RunKind == nil || *payload.RunKind != RunKindNetworkWake.String() {
			t.Fatalf("wake hook run kind = %#v, want %q", payload.RunKind, RunKindNetworkWake)
		}
		if payload.ResolvedNetworkParticipation == nil || *payload.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf("wake hook participation = %#v, want %#v", payload.ResolvedNetworkParticipation, liveSpec)
		}
	})
}

func TestTaskRunPreClaimCriteriaCarriesExactWakeIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should carry exact wake identity through pre-claim criteria", func(t *testing.T) {
		t.Parallel()

		var got hookspkg.TaskRunPreClaimPayload
		manager := newTaskManagerForTestWithOptions(
			t,
			newInMemoryManagerStore(),
			WithTaskRunHooks(recordingTaskRunHooks{
				preClaim: func(
					_ context.Context,
					payload hookspkg.TaskRunPreClaimPayload,
				) (hookspkg.TaskRunPreClaimPayload, error) {
					got = payload
					return payload, nil
				},
			}),
		)
		criteria := ClaimCriteria{
			RunID: "run-wake", RunKind: RunKindNetworkWake, Scope: ScopeWorkspace,
			WorkspaceID: "ws-wake", TargetSessionID: "sess-target", ClaimerSessionID: "sess-target",
			CallerNetworkParticipation: participation.CloneSpec(participation.Spec{
				Version: participation.SpecVersion, Mode: participation.ModeLive, WorkspaceID: "ws-wake",
				ChannelStrategy: participation.StrategyNamed, ChannelID: "wake-channel",
				Source: participation.SourceExplicitRequest, Bounds: participation.Bounds{
					MaxWakes: 1, MaxWakeWallTime: "1s", MaxTotalWallTime: "1s",
					MaxInputTokens: 10, MaxOutputTokens: 10, MaxWakeDepth: 1, CoalesceWindow: "100ms",
				},
			}),
			Now: time.Date(2026, 7, 13, 23, 0, 0, 0, time.UTC),
		}
		if _, err := manager.dispatchTaskRunPreClaimCriteria(
			context.Background(),
			criteria,
			validActorContext(),
		); err != nil {
			t.Fatalf("dispatchTaskRunPreClaimCriteria() error = %v", err)
		}
		if got.TaskRunContext == nil || got.RunID != "run-wake" || got.RunKind == nil ||
			*got.RunKind != RunKindNetworkWake.String() || got.TargetSessionID != "sess-target" {
			t.Fatalf("pre-claim wake context = %#v, want exact wake identity", got.TaskRunContext)
		}
		if got.Criteria.RunID != "run-wake" || got.Criteria.RunKind != RunKindNetworkWake.String() ||
			got.Criteria.TargetSessionID != "sess-target" {
			t.Fatalf("pre-claim wake criteria = %#v, want exact wake identity", got.Criteria)
		}
		if got.ResolvedNetworkParticipation == nil ||
			*got.ResolvedNetworkParticipation != *criteria.CallerNetworkParticipation {
			t.Fatalf(
				"pre-claim participation = %#v, want trusted caller snapshot %#v",
				got.ResolvedNetworkParticipation,
				criteria.CallerNetworkParticipation,
			)
		}
	})
}

func testTaskRunHookContextCarriesLoopFilterKeys(t *testing.T) {
	t.Helper()

	manager := newTaskManagerForTest(t, newInMemoryManagerStore())
	actor := validActorContext()
	run := Run{
		ID:        "run-loop-node",
		TaskID:    "task-node",
		RunKind:   RunKindWorker,
		LoopRunID: "loop-run-1",
		Status:    TaskRunStatusCompleted,
	}
	liveSpec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-1",
		ChannelStrategy: participation.StrategyLoopRun,
		ChannelID:       "loop-run-1",
		Source:          participation.SourceLoopDefinition,
	}
	run.SetNetworkState(liveSpec, "", "", "")
	taskRecord := Task{
		ID:           "task-node",
		ParentTaskID: "task-coordinator",
		WorkspaceID:  "ws-1",
		Status:       TaskStatusCompleted,
	}

	payload := manager.taskRunHookContext(run, taskRecord, actor)
	if got, want := payload.LoopRunID, "loop-run-1"; got != want {
		t.Fatalf("LoopRunID = %q, want %q", got, want)
	}
	if payload.RunKind == nil {
		t.Fatal("RunKind = nil, want worker")
	}
	if got, want := *payload.RunKind, RunKindWorker.String(); got != want {
		t.Fatalf("RunKind = %q, want %q", got, want)
	}
	if got := payload.ResolvedNetworkParticipation; got == nil || *got != liveSpec {
		t.Fatalf("ResolvedNetworkParticipation = %#v, want %#v", got, liveSpec)
	}

	taskPayload := manager.taskHookContext(taskRecord, actor, &BlockTaskAndReleaseRunResult{
		Run:           run,
		ReleaseReason: "blocked",
	})
	if got := taskPayload.ResolvedNetworkParticipation; got == nil || *got != liveSpec {
		t.Fatalf("TaskContext.ResolvedNetworkParticipation = %#v, want %#v", got, liveSpec)
	}
}

type recordingTaskRunHooks struct {
	blocked        func(context.Context, hookspkg.TaskBlockedPayload) (hookspkg.TaskBlockedPayload, error)
	unblocked      func(context.Context, hookspkg.TaskUnblockedPayload) (hookspkg.TaskUnblockedPayload, error)
	needsAttention func(
		context.Context,
		hookspkg.TaskNeedsAttentionPayload,
	) (hookspkg.TaskNeedsAttentionPayload, error)
	taskRecovered func(context.Context, hookspkg.TaskRecoveredPayload) (hookspkg.TaskRecoveredPayload, error)
	statusChanged func(context.Context, hookspkg.TaskStatusChangedPayload) (hookspkg.TaskStatusChangedPayload, error)
	enqueued      func(context.Context, hookspkg.TaskRunEnqueuedPayload) (hookspkg.TaskRunEnqueuedPayload, error)
	preClaim      func(context.Context, hookspkg.TaskRunPreClaimPayload) (hookspkg.TaskRunPreClaimPayload, error)
	postClaim     func(context.Context, hookspkg.TaskRunPostClaimPayload) (hookspkg.TaskRunPostClaimPayload, error)
	leaseExtended func(
		context.Context,
		hookspkg.TaskRunLeaseExtendedPayload,
	) (hookspkg.TaskRunLeaseExtendedPayload, error)
	leaseExpired func(
		context.Context,
		hookspkg.TaskRunLeaseExpiredPayload,
	) (hookspkg.TaskRunLeaseExpiredPayload, error)
	recovered func(
		context.Context,
		hookspkg.TaskRunLeaseRecoveredPayload,
	) (hookspkg.TaskRunLeaseRecoveredPayload, error)
	released     func(context.Context, hookspkg.TaskRunReleasedPayload) (hookspkg.TaskRunReleasedPayload, error)
	completed    func(context.Context, hookspkg.TaskRunCompletedPayload) (hookspkg.TaskRunCompletedPayload, error)
	failed       func(context.Context, hookspkg.TaskRunFailedPayload) (hookspkg.TaskRunFailedPayload, error)
	loopTerminal func(
		context.Context,
		hookspkg.LoopTerminalPayload,
	) (hookspkg.LoopTerminalPayload, error)
}

func (h recordingTaskRunHooks) DispatchTaskBlocked(
	ctx context.Context,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	if h.blocked != nil {
		return h.blocked(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskUnblocked(
	ctx context.Context,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	if h.unblocked != nil {
		return h.unblocked(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskNeedsAttention(
	ctx context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	if h.needsAttention != nil {
		return h.needsAttention(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRecovered(
	ctx context.Context,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	if h.taskRecovered != nil {
		return h.taskRecovered(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskStatusChanged(
	ctx context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	if h.statusChanged != nil {
		return h.statusChanged(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunEnqueued(
	ctx context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) (hookspkg.TaskRunEnqueuedPayload, error) {
	if h.enqueued != nil {
		return h.enqueued(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunPreClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPreClaimPayload,
) (hookspkg.TaskRunPreClaimPayload, error) {
	if h.preClaim != nil {
		return h.preClaim(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunPostClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPostClaimPayload,
) (hookspkg.TaskRunPostClaimPayload, error) {
	if h.postClaim != nil {
		return h.postClaim(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunLeaseExtended(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseExtendedPayload,
) (hookspkg.TaskRunLeaseExtendedPayload, error) {
	if h.leaseExtended != nil {
		return h.leaseExtended(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunLeaseExpired(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseExpiredPayload,
) (hookspkg.TaskRunLeaseExpiredPayload, error) {
	if h.leaseExpired != nil {
		return h.leaseExpired(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunLeaseRecovered(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseRecoveredPayload,
) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
	if h.recovered != nil {
		return h.recovered(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunReleased(
	ctx context.Context,
	payload hookspkg.TaskRunReleasedPayload,
) (hookspkg.TaskRunReleasedPayload, error) {
	if h.released != nil {
		return h.released(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunCompleted(
	ctx context.Context,
	payload hookspkg.TaskRunCompletedPayload,
) (hookspkg.TaskRunCompletedPayload, error) {
	if h.completed != nil {
		return h.completed(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchTaskRunFailed(
	ctx context.Context,
	payload hookspkg.TaskRunFailedPayload,
) (hookspkg.TaskRunFailedPayload, error) {
	if h.failed != nil {
		return h.failed(ctx, payload)
	}
	return payload, nil
}

func (h recordingTaskRunHooks) DispatchLoopTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	if h.loopTerminal != nil {
		return h.loopTerminal(ctx, payload)
	}
	return payload, nil
}
