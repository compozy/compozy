//go:build integration

package task_test

import (
	"context"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestTaskRunHooksDispatchAfterAuditEventsIntegration(t *testing.T) {
	t.Run("Should dispatch post-claim after the claim audit commits", func(t *testing.T) {
		t.Parallel()

		db := openTaskManagerGlobalDB(t)
		postClaimSawCommittedAudit := false
		manager := newTaskManagerIntegration(t, db, taskpkg.WithTaskRunHooks(integrationTaskRunHooks{
			postClaim: func(
				ctx context.Context,
				payload hookspkg.TaskRunPostClaimPayload,
			) (hookspkg.TaskRunPostClaimPayload, error) {
				events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{
					TaskID: payload.TaskID,
					RunID:  payload.RunID,
				})
				if err != nil {
					t.Fatalf("ListTaskEvents() during post-claim hook error = %v", err)
				}
				if !containsEventType(events, "task.run_claimed") {
					t.Fatalf(
						"event types during post-claim hook = %#v, want task.run_claimed",
						sortedEventTypes(events),
					)
				}
				postClaimSawCommittedAudit = true
				return payload, nil
			},
		}))

		actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "compozy task run")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Post-claim hook ordering",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if _, err := claimExactRunIntegration(testutil.Context(t), manager, db, run.ID, actor); err != nil {
			t.Fatalf("claimExactRunIntegration() error = %v", err)
		}
		if !postClaimSawCommittedAudit {
			t.Fatal("post-claim hook did not observe committed claim audit event")
		}
	})

	t.Run("Should dispatch release and status hooks after the full settlement commits", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		statusReadyCalls := 0
		releasedCalls := 0
		var taskID string
		var runID string
		hooks := integrationTaskRunHooks{
			statusChanged: func(
				hookCtx context.Context,
				payload hookspkg.TaskStatusChangedPayload,
			) (hookspkg.TaskStatusChangedPayload, error) {
				if payload.ToStatus != string(taskpkg.TaskStatusReady) || taskID == "" || runID == "" {
					return payload, nil
				}
				statusReadyCalls++
				assertReleasedRunSettlementCommitted(t, hookCtx, db, taskID, runID)
				return payload, nil
			},
			released: func(
				hookCtx context.Context,
				payload hookspkg.TaskRunReleasedPayload,
			) (hookspkg.TaskRunReleasedPayload, error) {
				releasedCalls++
				assertReleasedRunSettlementCommitted(t, hookCtx, db, payload.TaskID, payload.RunID)
				return payload, nil
			},
		}
		manager := newTaskManagerIntegration(
			t,
			db,
			taskpkg.WithSessionExecutor(&integrationSessionExecutor{}),
			taskpkg.WithTaskRunHooks(hooks),
		)
		actor, err := taskpkg.DeriveHumanActorContext("user-2", taskpkg.OriginKindCLI, "compozy task run")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Post-release hook ordering",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claim, err := claimExactRunIntegration(ctx, manager, db, run.ID, actor)
		if err != nil {
			t.Fatalf("claimExactRunIntegration() error = %v", err)
		}
		if _, err := manager.StartRun(ctx, run.ID, taskpkg.StartRun{
			ClaimToken: claim.ClaimToken,
		}, actor); err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		taskID = taskRecord.ID
		runID = run.ID

		released, err := manager.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Reason:     "handoff",
			Now:        time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC),
		}, actor)
		if err != nil {
			t.Fatalf("ReleaseRunLease() error = %v", err)
		}
		if got, want := released.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("released.Status = %q, want %q", got, want)
		}
		if got, want := statusReadyCalls, 1; got != want {
			t.Fatalf("status-ready hook calls = %d, want %d", got, want)
		}
		if got, want := releasedCalls, 1; got != want {
			t.Fatalf("released hook calls = %d, want %d", got, want)
		}
	})

	t.Run("Should dispatch failure and status hooks after the full settlement commits", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		statusReadyCalls := 0
		failedCalls := 0
		var taskID string
		var runID string
		var rawClaimToken string
		hooks := integrationTaskRunHooks{
			statusChanged: func(
				hookCtx context.Context,
				payload hookspkg.TaskStatusChangedPayload,
			) (hookspkg.TaskStatusChangedPayload, error) {
				if payload.ToStatus != string(taskpkg.TaskStatusReady) || taskID == "" || runID == "" {
					return payload, nil
				}
				statusReadyCalls++
				assertFailedRunSettlementCommitted(t, hookCtx, db, taskID, runID)
				return payload, nil
			},
			failed: func(
				hookCtx context.Context,
				payload hookspkg.TaskRunFailedPayload,
			) (hookspkg.TaskRunFailedPayload, error) {
				failedCalls++
				if rawClaimToken == "" || strings.Contains(payload.Error, rawClaimToken) {
					t.Fatalf("failed hook error = %q, want raw claim bearer redacted", payload.Error)
				}
				if got, want := payload.Error, "worker failed with compozy_claim_[REDACTED]"; got != want {
					t.Fatalf("failed hook error = %q, want %q", got, want)
				}
				assertFailedRunSettlementCommitted(t, hookCtx, db, payload.TaskID, payload.RunID)
				return payload, nil
			},
		}
		manager := newTaskManagerIntegration(
			t,
			db,
			taskpkg.WithSessionExecutor(&integrationSessionExecutor{}),
			taskpkg.WithTaskRunHooks(hooks),
		)
		actor, err := taskpkg.DeriveHumanActorContext("user-3", taskpkg.OriginKindCLI, "compozy task run")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Post-failure hook ordering",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claim, err := claimExactRunIntegration(ctx, manager, db, run.ID, actor)
		if err != nil {
			t.Fatalf("claimExactRunIntegration() error = %v", err)
		}
		if _, err := manager.StartRun(ctx, run.ID, taskpkg.StartRun{
			ClaimToken: claim.ClaimToken,
		}, actor); err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		taskID = taskRecord.ID
		runID = run.ID
		rawClaimToken = claim.ClaimToken

		failed, err := manager.FailRunLease(ctx, taskpkg.LeaseFailure{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Failure: taskpkg.RunFailure{
				Error: "worker failed with " + claim.ClaimToken,
			},
			Now: time.Date(2026, 7, 31, 18, 30, 0, 0, time.UTC),
		}, actor)
		if err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}
		if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("failed.Status = %q, want %q", got, want)
		}
		if got, want := statusReadyCalls, 1; got != want {
			t.Fatalf("status-ready hook calls = %d, want %d", got, want)
		}
		if got, want := failedCalls, 1; got != want {
			t.Fatalf("failed hook calls = %d, want %d", got, want)
		}
	})
}

func assertReleasedRunSettlementCommitted(
	t *testing.T,
	ctx context.Context,
	db taskpkg.Store,
	taskID string,
	runID string,
) {
	t.Helper()

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskID, RunID: runID})
	if err != nil {
		t.Fatalf("ListTaskEvents() during release hook error = %v", err)
	}
	if !containsEventType(events, "task.run_released") {
		t.Fatalf("event types during release hook = %#v, want task.run_released", sortedEventTypes(events))
	}
	run, err := db.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun() during release hook error = %v", err)
	}
	if run.Status != taskpkg.TaskRunStatusQueued || run.SessionID != "" || run.ClaimTokenHash != "" {
		t.Fatalf("run during release hook = %#v, want committed reusable queued run", run)
	}
	taskRecord, err := db.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTask() during release hook error = %v", err)
	}
	if taskRecord.Status != taskpkg.TaskStatusReady || taskRecord.CurrentRunID != "" {
		t.Fatalf("task during release hook = %#v, want committed ready/cleared projection", taskRecord)
	}
}

func assertFailedRunSettlementCommitted(
	t *testing.T,
	ctx context.Context,
	db taskpkg.Store,
	taskID string,
	runID string,
) {
	t.Helper()

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskID, RunID: runID})
	if err != nil {
		t.Fatalf("ListTaskEvents() during failure hook error = %v", err)
	}
	if !containsEventType(events, "task.run.failed") {
		t.Fatalf("event types during failure hook = %#v, want task.run.failed", sortedEventTypes(events))
	}
	run, err := db.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun() during failure hook error = %v", err)
	}
	if run.Status != taskpkg.TaskRunStatusFailed ||
		run.Error != "worker failed with compozy_claim_[REDACTED]" ||
		!run.LeaseUntil.IsZero() {
		t.Fatalf("run during failure hook = %#v, want committed failed run", run)
	}
	taskRecord, err := db.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTask() during failure hook error = %v", err)
	}
	if taskRecord.Status != taskpkg.TaskStatusReady || taskRecord.CurrentRunID != "" {
		t.Fatalf("task during failure hook = %#v, want committed ready/cleared projection", taskRecord)
	}
}

type integrationTaskRunHooks struct {
	statusChanged func(
		context.Context,
		hookspkg.TaskStatusChangedPayload,
	) (hookspkg.TaskStatusChangedPayload, error)
	enqueued  func(context.Context, hookspkg.TaskRunEnqueuedPayload) (hookspkg.TaskRunEnqueuedPayload, error)
	preClaim  func(context.Context, hookspkg.TaskRunPreClaimPayload) (hookspkg.TaskRunPreClaimPayload, error)
	postClaim func(context.Context, hookspkg.TaskRunPostClaimPayload) (hookspkg.TaskRunPostClaimPayload, error)
	recovered func(
		context.Context,
		hookspkg.TaskRunLeaseRecoveredPayload,
	) (hookspkg.TaskRunLeaseRecoveredPayload, error)
	released  func(context.Context, hookspkg.TaskRunReleasedPayload) (hookspkg.TaskRunReleasedPayload, error)
	completed func(context.Context, hookspkg.TaskRunCompletedPayload) (hookspkg.TaskRunCompletedPayload, error)
	failed    func(context.Context, hookspkg.TaskRunFailedPayload) (hookspkg.TaskRunFailedPayload, error)
}

func (h integrationTaskRunHooks) DispatchTaskBlocked(
	_ context.Context,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskUnblocked(
	_ context.Context,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskNeedsAttention(
	_ context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRecovered(
	_ context.Context,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskStatusChanged(
	ctx context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	if h.statusChanged != nil {
		return h.statusChanged(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunEnqueued(
	ctx context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) (hookspkg.TaskRunEnqueuedPayload, error) {
	if h.enqueued != nil {
		return h.enqueued(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunPreClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPreClaimPayload,
) (hookspkg.TaskRunPreClaimPayload, error) {
	if h.preClaim != nil {
		return h.preClaim(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunPostClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPostClaimPayload,
) (hookspkg.TaskRunPostClaimPayload, error) {
	if h.postClaim != nil {
		return h.postClaim(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunLeaseExtended(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExtendedPayload,
) (hookspkg.TaskRunLeaseExtendedPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunLeaseExpired(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExpiredPayload,
) (hookspkg.TaskRunLeaseExpiredPayload, error) {
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunLeaseRecovered(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseRecoveredPayload,
) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
	if h.recovered != nil {
		return h.recovered(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunReleased(
	ctx context.Context,
	payload hookspkg.TaskRunReleasedPayload,
) (hookspkg.TaskRunReleasedPayload, error) {
	if h.released != nil {
		return h.released(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunCompleted(
	ctx context.Context,
	payload hookspkg.TaskRunCompletedPayload,
) (hookspkg.TaskRunCompletedPayload, error) {
	if h.completed != nil {
		return h.completed(ctx, payload)
	}
	return payload, nil
}

func (h integrationTaskRunHooks) DispatchTaskRunFailed(
	ctx context.Context,
	payload hookspkg.TaskRunFailedPayload,
) (hookspkg.TaskRunFailedPayload, error) {
	if h.failed != nil {
		return h.failed(ctx, payload)
	}
	return payload, nil
}
