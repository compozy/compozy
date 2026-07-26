package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestGlobalDBClaimNextRunConcurrentSingleWinner(t *testing.T) {
	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	taskRecord := taskRecordForTest("task-claim-concurrent")
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-claim-concurrent", taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	type claimAttempt struct {
		result taskpkg.ClaimResult
		err    error
	}
	attempts := make([]claimAttempt, 5)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(attempts))
	for idx := range attempts {
		go func() {
			defer wg.Done()
			<-start
			attempts[idx].result, attempts[idx].err = globalDB.ClaimNextRun(
				ctx,
				taskpkg.ClaimCriteria{
					Scope:            taskpkg.ScopeGlobal,
					ClaimerSessionID: "sess-race-" + string(rune('a'+idx)),
					LeaseDuration:    time.Minute,
					Now:              time.Date(2026, 4, 26, 12, 0, 0, idx, time.UTC),
				},
			)
		}()
	}
	close(start)
	wg.Wait()

	successes := 0
	for idx, attempt := range attempts {
		if attempt.err == nil {
			successes++
			if got, want := attempt.result.Run.ID, run.ID; got != want {
				t.Fatalf("attempt %d claimed run %q, want %q", idx, got, want)
			}
			if attempt.result.ClaimToken == "" {
				t.Fatalf("attempt %d returned empty claim token", idx)
			}
			if !taskpkg.VerifyClaimToken(
				attempt.result.ClaimToken,
				attempt.result.Run.ClaimTokenHash,
			) {
				t.Fatalf("attempt %d claim token does not match stored hash", idx)
			}
			continue
		}
		if !errors.Is(attempt.err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("attempt %d error = %v, want %v", idx, attempt.err, taskpkg.ErrNoClaimableRun)
		}
	}
	if successes != 1 {
		t.Fatalf("successful claims = %d, want exactly 1 (attempts=%#v)", successes, attempts)
	}

	stored, err := globalDB.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if got, want := stored.Status, taskpkg.TaskRunStatusClaimed; got != want {
		t.Fatalf("stored.Status = %q, want %q", got, want)
	}
	if stored.SessionID == "" {
		t.Fatal("stored.SessionID = empty, want winning session id")
	}
	t.Logf(
		"claim attempts=%d successes=%d winner_session_id=%s run_id=%s",
		len(attempts),
		successes,
		stored.SessionID,
		stored.ID,
	)
}

func TestGlobalDBClaimNextRunExactRunID(t *testing.T) {
	t.Parallel()

	t.Run("Should claim only the requested queued run with the canonical lease", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 21, 0, 0, 0, time.UTC)
		createExactRun(ctx, t, globalDB, "task-exact-older", "run-exact-older", taskpkg.PriorityUrgent, now)
		target := createExactRun(
			ctx, t, globalDB, "task-exact-target", "run-exact-target", taskpkg.PriorityLow, now.Add(time.Minute),
		)

		result, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: target.ID, Scope: taskpkg.ScopeGlobal, ClaimerSessionID: "sess-exact",
			LeaseDuration: 2 * time.Minute, Now: now.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(exact) error = %v", err)
		}
		if result.Run.ID != target.ID {
			t.Fatalf("ClaimNextRun(exact) run = %q, want %q", result.Run.ID, target.ID)
		}
		if result.Task == nil || result.Task.ID != target.TaskID {
			t.Fatalf("ClaimNextRun(exact) task = %#v, want %q", result.Task, target.TaskID)
		}
		if result.ClaimToken == "" || !taskpkg.VerifyClaimToken(result.ClaimToken, result.Run.ClaimTokenHash) {
			t.Fatal("ClaimNextRun(exact) returned an invalid canonical claim token")
		}
		if want := now.Add(4 * time.Minute); !result.LeaseUntil.Equal(want) {
			t.Fatalf("ClaimNextRun(exact) lease_until = %s, want %s", result.LeaseUntil, want)
		}
	})

	t.Run("Should not fall back when the exact target is unavailable", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 21, 30, 0, 0, time.UTC)
		target := createExactRun(
			ctx, t, globalDB, "task-exact-busy", "run-exact-busy", taskpkg.PriorityMedium, now,
		)
		fallback := createExactRun(
			ctx, t, globalDB, "task-exact-fallback", "run-exact-fallback", taskpkg.PriorityUrgent, now,
		)
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: target.ID, Scope: taskpkg.ScopeGlobal, ClaimerSessionID: "sess-first", Now: now,
		}); err != nil {
			t.Fatalf("ClaimNextRun(first exact) error = %v", err)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: target.ID, Scope: taskpkg.ScopeGlobal, ClaimerSessionID: "sess-second", Now: now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(unavailable exact) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		storedFallback, err := globalDB.GetTaskRun(ctx, fallback.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(fallback) error = %v", err)
		}
		if storedFallback.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			t.Fatalf("fallback status = %s, want queued", storedFallback.Status)
		}
	})

	t.Run("Should reject an exact target with an unresolved dependency", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
		prerequisite := taskRecordForTest("task-exact-prerequisite")
		prerequisite.Status = taskpkg.TaskStatusPending
		if err := globalDB.CreateTask(ctx, prerequisite); err != nil {
			t.Fatalf("CreateTask(prerequisite) error = %v", err)
		}
		target := createExactRun(
			ctx,
			t,
			globalDB,
			"task-exact-dependent",
			"run-exact-dependent",
			taskpkg.PriorityMedium,
			now,
		)
		if err := globalDB.CreateDependency(ctx, taskpkg.Dependency{
			TaskID:          target.TaskID,
			DependsOnTaskID: prerequisite.ID,
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("CreateDependency() error = %v", err)
		}

		_, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: target.ID, Scope: taskpkg.ScopeGlobal, ClaimerSessionID: "sess-exact-dependent",
			LeaseDuration: time.Minute, Now: now.Add(2 * time.Minute),
		})
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(unresolved dependency) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
	})

	t.Run("Should admit an exact target when its dependency has a completed latest run", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
		prerequisite := taskRecordForTest("task-exact-completed-prerequisite")
		prerequisite.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, prerequisite); err != nil {
			t.Fatalf("CreateTask(prerequisite) error = %v", err)
		}
		completedRun := taskRunForTest("run-exact-completed-prerequisite", prerequisite.ID)
		completedRun.Status = taskpkg.TaskRunStatusCompleted
		completedRun.StartedAt = now
		completedRun.EndedAt = now.Add(time.Minute)
		if err := globalDB.CreateTaskRun(ctx, completedRun); err != nil {
			t.Fatalf("CreateTaskRun(completed prerequisite) error = %v", err)
		}
		target := createExactRun(
			ctx,
			t,
			globalDB,
			"task-exact-released-dependent",
			"run-exact-released-dependent",
			taskpkg.PriorityMedium,
			now.Add(2*time.Minute),
		)
		if err := globalDB.CreateDependency(ctx, taskpkg.Dependency{
			TaskID:          target.TaskID,
			DependsOnTaskID: prerequisite.ID,
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now.Add(3 * time.Minute),
		}); err != nil {
			t.Fatalf("CreateDependency() error = %v", err)
		}

		claimed, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: target.ID, Scope: taskpkg.ScopeGlobal, ClaimerSessionID: "sess-exact-released",
			LeaseDuration: time.Minute, Now: now.Add(4 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(completed dependency) error = %v", err)
		}
		if claimed.Run.ID != target.ID {
			t.Fatalf("ClaimNextRun(completed dependency) run = %q, want %q", claimed.Run.ID, target.ID)
		}
	})

	t.Run("Should allow a workspace caller to claim global work without crossing workspaces", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 21, 45, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "exact-scope", t.TempDir())
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"exact-scope-foreign",
			t.TempDir(),
		)
		globalRun := createExactRun(
			ctx,
			t,
			globalDB,
			"task-exact-global",
			"run-exact-global",
			taskpkg.PriorityMedium,
			now,
		)
		foreignTask := taskRecordForTest("task-exact-foreign")
		foreignTask.Scope = taskpkg.ScopeWorkspace
		foreignTask.WorkspaceID = foreignWorkspaceID
		foreignTask.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, foreignTask); err != nil {
			t.Fatalf("CreateTask(foreign) error = %v", err)
		}
		foreignRun := taskRunForTest("run-exact-foreign", foreignTask.ID)
		if err := globalDB.CreateTaskRun(ctx, foreignRun); err != nil {
			t.Fatalf("CreateTaskRun(foreign) error = %v", err)
		}

		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: globalRun.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			ClaimerSessionID: "sess-exact-global", Now: now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(global from workspace) error = %v", err)
		}
		if claim.Run.ID != globalRun.ID {
			t.Fatalf("ClaimNextRun(global from workspace) run = %q, want %q", claim.Run.ID, globalRun.ID)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: foreignRun.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			ClaimerSessionID: "sess-exact-foreign", Now: now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(foreign workspace) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		storedForeign, err := globalDB.GetTaskRun(ctx, foreignRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(foreign) error = %v", err)
		}
		if storedForeign.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			t.Fatalf("foreign run status = %s, want queued", storedForeign.Status)
		}
	})

	t.Run("Should preserve exact owner fences for pool and agent session work", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 21, 50, 0, 0, time.UTC)
		createOwnedRun := func(taskID, runID string, owner *taskpkg.Ownership) taskpkg.Run {
			t.Helper()
			taskRecord := taskRecordForTest(taskID)
			taskRecord.Status = taskpkg.TaskStatusReady
			taskRecord.Owner = owner
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", taskID, err)
			}
			run := taskRunForTest(runID, taskID)
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(%q) error = %v", runID, err)
			}
			return run
		}

		poolRun := createOwnedRun(
			"task-exact-pool",
			"run-exact-pool",
			ownershipForTest(taskpkg.OwnerKindPool, "builder"),
		)
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: poolRun.ID, Scope: taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-wrong-pool", AgentName: "reviewer", Now: now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(unmatched pool) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: poolRun.ID, Scope: taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-matched-pool", AgentName: "builder", Now: now,
		}); err != nil {
			t.Fatalf("ClaimNextRun(matched pool) error = %v", err)
		}

		sessionRun := createOwnedRun(
			"task-exact-session",
			"run-exact-session",
			ownershipForTest(taskpkg.OwnerKindAgentSession, "sess-owner"),
		)
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: sessionRun.ID, Scope: taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-foreign", Now: now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(foreign session owner) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: sessionRun.ID, Scope: taskpkg.ScopeGlobal,
			ClaimerSessionID: "daemon-recovery", ClaimedBy: &taskpkg.ActorIdentity{
				Kind: taskpkg.ActorKindDaemon, Ref: "boot-recovery",
			}, Now: now,
		}); err != nil {
			t.Fatalf("ClaimNextRun(daemon recovery) error = %v", err)
		}
	})
}

func TestGlobalDBNetworkWakeRunLeaseLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should target claim while hiding generic settlement authority", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 22, 0, 0, 0, time.UTC)
		registerNetworkWakeRunSessionsForClaimTest(t, globalDB, now, "sess-target", "sess-active")
		activeTask := workspaceTaskRecordForTest("task-wake-cap-active", "ws-wake")
		activeTask.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, activeTask); err != nil {
			t.Fatalf("CreateTask(active) error = %v", err)
		}
		activeRun := taskRunForTest("run-wake-cap-active", activeTask.ID)
		if err := globalDB.CreateTaskRun(ctx, activeRun); err != nil {
			t.Fatalf("CreateTaskRun(active) error = %v", err)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: activeRun.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: "ws-wake",
			ClaimerSessionID: "sess-active", LeaseDuration: time.Minute, Now: now,
			WorkspaceActiveRunCap: 1,
		}); err != nil {
			t.Fatalf("ClaimNextRun(active workspace run) error = %v", err)
		}
		anchor := taskRecordForTest("task-wake-anchor")
		anchor.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, anchor); err != nil {
			t.Fatalf("CreateTask(anchor) error = %v", err)
		}
		wake := networkWakeRunForClaimTest("run-network-wake", "wake-1", "sess-target", "owner-1", now)
		createNetworkWakeRunForClaimTest(t, globalDB, wake, now)
		queued, err := globalDB.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(network wake) error = %v", err)
		}
		if len(queued) != 1 || queued[0].ID != wake.ID || !queued[0].IsNetworkWake() || queued[0].TaskID != "" {
			t.Fatalf("queued wake projection = %#v, want one badged taskless wake", queued)
		}

		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: wake.ID, RunKind: taskpkg.RunKindNetworkWake,
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: "ws-wake",
			TargetSessionID: "sess-foreign", ClaimerSessionID: "sess-foreign", Now: now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(foreign wake) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: wake.ID, RunKind: taskpkg.RunKindNetworkWake,
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: "ws-wake",
			TargetSessionID: "sess-target", ClaimerSessionID: "sess-target", Now: now,
			WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(target wake) error = %v", err)
		}
		if claim.Task != nil {
			t.Fatalf("ClaimNextRun(target wake) task = %#v, want nil", claim.Task)
		}
		handles, err := globalDB.ListAutonomyLeaseHandles(ctx, "sess-target")
		if err != nil {
			t.Fatalf("ListAutonomyLeaseHandles(network wake) error = %v", err)
		}
		if len(handles) != 0 {
			t.Fatalf("ListAutonomyLeaseHandles(network wake) = %#v, want no generic handle", handles)
		}
		if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			Actor: coordinatorActorContextForTest(),
			RunID: wake.ID, ClaimToken: claim.ClaimToken, LeaseDuration: time.Minute,
			Now: now.Add(time.Second), TokensUsed: 17,
		}); err != nil {
			t.Fatalf("HeartbeatRunLease(network wake) error = %v", err)
		}
		_, err = globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor: coordinatorActorContextForTest(), RunID: wake.ID, ClaimToken: claim.ClaimToken,
			Result: taskpkg.RunResult{Value: json.RawMessage(`{"delivered":true}`)},
			Now:    now.Add(2 * time.Second), TokensUsed: 23,
		})
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("CompleteRunLease(network wake) error = %v, want %v", err, taskpkg.ErrValidation)
		}
		storedAnchor, err := globalDB.GetTask(ctx, anchor.ID)
		if err != nil {
			t.Fatalf("GetTask(anchor) error = %v", err)
		}
		if storedAnchor.Status.Normalize() != taskpkg.TaskStatusReady || storedAnchor.CurrentRunID != "" {
			t.Fatalf(
				"anchor projection = status %s current_run %q, want ready/empty",
				storedAnchor.Status,
				storedAnchor.CurrentRunID,
			)
		}
	})

	t.Run("Should release fail and recover wakes through the standard token-fenced lifecycle", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 22, 30, 0, 0, time.UTC)
		registerNetworkWakeRunSessionsForClaimTest(
			t,
			globalDB,
			now,
			"sess-release",
			"sess-fail",
			"sess-expire",
		)
		for _, run := range []taskpkg.Run{
			networkWakeRunForClaimTest("run-wake-release", "wake-release", "sess-release", "owner-release", now),
			networkWakeRunForClaimTest("run-wake-fail", "wake-fail", "sess-fail", "owner-fail", now),
			networkWakeRunForClaimTest("run-wake-expire", "wake-expire", "sess-expire", "owner-expire", now),
		} {
			createNetworkWakeRunForClaimTest(t, globalDB, run, now)
		}
		claimWake := func(runID, sessionID string, at time.Time) taskpkg.ClaimResult {
			t.Helper()
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID: runID, RunKind: taskpkg.RunKindNetworkWake,
				Scope: taskpkg.ScopeWorkspace, WorkspaceID: "ws-wake",
				TargetSessionID: sessionID, ClaimerSessionID: sessionID,
				LeaseDuration: time.Minute, Now: at,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(%q) error = %v", runID, err)
			}
			return claim
		}
		releaseClaim := claimWake("run-wake-release", "sess-release", now)
		released, err := globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
			Actor: coordinatorActorContextForTest(),
			RunID: releaseClaim.Run.ID, ClaimToken: releaseClaim.ClaimToken,
			Reason: "handoff", Now: now.Add(time.Second),
		})
		if err != nil || released.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			t.Fatalf("ReleaseRunLease(network wake) = %#v, %v, want queued", released, err)
		}

		failClaim := claimWake("run-wake-fail", "sess-fail", now)
		_, err = globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
			Actor: coordinatorActorContextForTest(), RunID: failClaim.Run.ID,
			ClaimToken: failClaim.ClaimToken, Failure: taskpkg.RunFailure{Error: "provider failed"},
			Now: now.Add(time.Second),
		})
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("FailRunLease(network wake) error = %v, want %v", err, taskpkg.ErrValidation)
		}
		if _, err := globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
			Actor: coordinatorActorContextForTest(), RunID: failClaim.Run.ID,
			ClaimToken: failClaim.ClaimToken, Reason: "settlement handoff",
			Now: now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("ReleaseRunLease(after generic failure rejection) error = %v", err)
		}

		expiringClaim := claimWake("run-wake-expire", "sess-expire", now)
		if expiringClaim.Run.ID != "run-wake-expire" || expiringClaim.ClaimToken == "" {
			t.Fatalf("expiring wake claim = %#v, want exact run and raw token", expiringClaim)
		}
		recovered, err := globalDB.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
			Now: now.Add(2 * time.Minute), Reason: "expired", Limit: 10,
		})
		if err != nil {
			t.Fatalf("RecoverExpiredRunLeases(network wake) error = %v", err)
		}
		if len(recovered) != 1 || recovered[0].Run.ID != "run-wake-expire" ||
			recovered[0].Run.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			t.Fatalf("recovered wakes = %#v, want one queued expired wake", recovered)
		}
	})
}

func createExactRun(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	runID string,
	priority taskpkg.Priority,
	queuedAt time.Time,
) taskpkg.Run {
	t.Helper()
	taskRecord := taskRecordForTest(taskID)
	taskRecord.Status = taskpkg.TaskStatusReady
	taskRecord.Priority = priority
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask(%q) error = %v", taskID, err)
	}
	run := taskRunForTest(runID, taskID)
	run.QueuedAt = queuedAt
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", runID, err)
	}
	return run
}

func networkWakeRunForClaimTest(
	runID string,
	wakeID string,
	targetSessionID string,
	ownerKey string,
	queuedAt time.Time,
) taskpkg.Run {
	run := taskpkg.Run{
		ID: runID, RunKind: taskpkg.RunKindNetworkWake, Status: taskpkg.TaskRunStatusQueued,
		WorkspaceID: "ws-wake",
		Attempt:     1, Origin: taskpkg.Origin{Kind: taskpkg.OriginKindNetwork, Ref: "network.accept"},
		QueuedAt: queuedAt,
	}
	run.SetNetworkState(participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-wake",
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "wake-channel",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}, wakeID, targetSessionID, ownerKey)
	return run
}

func createNetworkWakeRunForClaimTest(
	t *testing.T,
	globalDB *GlobalDB,
	run taskpkg.Run,
	now time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
	}
	wakeID, _, ownerKey := run.NetworkWakeCorrelation()
	if _, err := globalDB.db.ExecContext(ctx, `
INSERT INTO network_live_wakes (
  wake_id, task_run_id, owner_key, workspace_id, channel, root_id, depth,
  state, coalesce_until, reserved_wall_ms, reserved_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wakeID,
		run.ID,
		ownerKey,
		run.WorkspaceID,
		run.NetworkSpecSnapshot().ChannelID,
		wakeID,
		0,
		"open",
		store.FormatTimestamp(now.Add(250*time.Millisecond)),
		int64(30_000),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("insert network_live_wakes(%q) error = %v", wakeID, err)
	}
}

func registerNetworkWakeRunSessionsForClaimTest(
	t *testing.T,
	globalDB *GlobalDB,
	now time.Time,
	sessionIDs ...string,
) {
	t.Helper()
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"wake",
		filepath.Join(t.TempDir(), "wake"),
	)
	for _, sessionID := range sessionIDs {
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ID: sessionID, AgentName: "coder", Provider: "claude",
			WorkspaceID: workspaceID, State: "active", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
		}
	}
}

func TestGlobalDBClaimNextRunFiltersByCapabilitiesAndScope(t *testing.T) {
	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"claim-filters",
		filepath.Join(t.TempDir(), "claim-filters"),
	)
	otherWorkspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"claim-filters-other",
		filepath.Join(t.TempDir(), "claim-filters-other"),
	)

	matchingTask := taskRecordForTest("task-claim-match")
	matchingTask.Scope = taskpkg.ScopeWorkspace
	matchingTask.WorkspaceID = workspaceID
	matchingTask.Status = taskpkg.TaskStatusReady
	matchingTask.Priority = taskpkg.PriorityHigh
	if err := globalDB.CreateTask(ctx, matchingTask); err != nil {
		t.Fatalf("CreateTask(matching) error = %v", err)
	}
	matchingRun := taskRunForTest("run-claim-match", matchingTask.ID)
	matchingRun.RequiredCapabilities = []string{"golang", "sqlite"}
	matchingRun.PreferredCapabilities = []string{"codex"}
	if err := globalDB.CreateTaskRun(ctx, matchingRun); err != nil {
		t.Fatalf("CreateTaskRun(matching) error = %v", err)
	}

	missingCapabilityTask := taskRecordForTest("task-claim-rust")
	missingCapabilityTask.Scope = taskpkg.ScopeWorkspace
	missingCapabilityTask.WorkspaceID = workspaceID
	missingCapabilityTask.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, missingCapabilityTask); err != nil {
		t.Fatalf("CreateTask(missing capability) error = %v", err)
	}
	missingCapabilityRun := taskRunForTest("run-claim-rust", missingCapabilityTask.ID)
	missingCapabilityRun.RequiredCapabilities = []string{"rust"}
	if err := globalDB.CreateTaskRun(ctx, missingCapabilityRun); err != nil {
		t.Fatalf("CreateTaskRun(missing capability) error = %v", err)
	}

	otherWorkspaceTask := taskRecordForTest("task-claim-other-workspace")
	otherWorkspaceTask.Scope = taskpkg.ScopeWorkspace
	otherWorkspaceTask.WorkspaceID = otherWorkspaceID
	otherWorkspaceTask.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, otherWorkspaceTask); err != nil {
		t.Fatalf("CreateTask(other workspace) error = %v", err)
	}
	otherWorkspaceRun := taskRunForTest("run-claim-other-workspace", otherWorkspaceTask.ID)
	otherWorkspaceRun.RequiredCapabilities = []string{"golang"}
	if err := globalDB.CreateTaskRun(ctx, otherWorkspaceRun); err != nil {
		t.Fatalf("CreateTaskRun(other workspace) error = %v", err)
	}

	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:                taskpkg.ScopeWorkspace,
		WorkspaceID:          workspaceID,
		ClaimerSessionID:     "sess-capable",
		RequiredCapabilities: []string{"golang", "sqlite", "codex"},
		LeaseDuration:        time.Minute,
		Now:                  time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	if got, want := claim.Run.ID, matchingRun.ID; got != want {
		t.Fatalf("ClaimNextRun() run id = %q, want %q", got, want)
	}

	if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:                taskpkg.ScopeWorkspace,
		WorkspaceID:          workspaceID,
		ClaimerSessionID:     "sess-golang-only",
		RequiredCapabilities: []string{"golang"},
		LeaseDuration:        time.Minute,
		Now:                  time.Date(2026, 4, 26, 12, 1, 0, 0, time.UTC),
	}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
		t.Fatalf("ClaimNextRun(golang only) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
	}

	storedOther, err := globalDB.GetTaskRun(ctx, otherWorkspaceRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(other workspace) error = %v", err)
	}
	if got, want := storedOther.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("other workspace run status = %q, want %q", got, want)
	}
}

func TestGlobalDBClaimNextRunScopesCoordinationMetadataToRunWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should scope coordination metadata to the claimed run workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceA := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"claim-channel-scope-a",
			filepath.Join(t.TempDir(), "claim-channel-scope-a"),
		)
		workspaceB := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"claim-channel-scope-b",
			filepath.Join(t.TempDir(), "claim-channel-scope-b"),
		)
		now := time.Date(2026, 7, 13, 17, 0, 0, 0, time.UTC)
		for _, channel := range []store.NetworkChannelEntry{
			{
				Channel:     "operations",
				WorkspaceID: workspaceA,
				Purpose:     "Workspace A operations",
				CreatedBy:   "agent-a",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				Channel:     "operations",
				WorkspaceID: workspaceB,
				Purpose:     "Workspace B operations",
				CreatedBy:   "agent-b",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		} {
			if err := globalDB.WriteNetworkChannel(ctx, channel); err != nil {
				t.Fatalf("WriteNetworkChannel(%q) error = %v", channel.WorkspaceID, err)
			}
		}

		createRun := func(taskID, runID, workspaceID string) {
			t.Helper()
			taskRecord := taskRecordForTest(taskID)
			taskRecord.Scope = taskpkg.ScopeWorkspace
			taskRecord.WorkspaceID = workspaceID
			taskRecord.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", workspaceID, err)
			}
			run := taskRunForTest(runID, taskID)
			run.SetNetworkState(participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				WorkspaceID:     workspaceID,
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       "operations",
				Source:          participation.SourceExplicitRequest,
				Bounds: participation.Bounds{
					MaxWakes:         4,
					MaxWakeWallTime:  "30s",
					MaxTotalWallTime: "2m",
					MaxInputTokens:   4096,
					MaxOutputTokens:  4096,
					MaxWakeDepth:     4,
					CoalesceWindow:   "250ms",
				},
			}, "", "", "")
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(%q) error = %v", workspaceID, err)
			}
		}
		createRun("task-channel-scope-a", "run-channel-scope-a", workspaceA)
		createRun("task-channel-scope-b", "run-channel-scope-b", workspaceB)

		for index, expected := range []struct {
			workspaceID string
			purpose     string
		}{
			{workspaceID: workspaceA, purpose: "Workspace A operations"},
			{workspaceID: workspaceB, purpose: "Workspace B operations"},
		} {
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:                taskpkg.ScopeWorkspace,
				WorkspaceID:          expected.workspaceID,
				ClaimerSessionID:     fmt.Sprintf("sess-channel-scope-%d", index),
				ParticipationChannel: "operations",
				LeaseDuration:        time.Minute,
				Now:                  now.Add(time.Duration(index) * time.Second),
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(%q) error = %v", expected.workspaceID, err)
			}
			if claim.CoordinationChannel == nil {
				t.Fatalf("ClaimNextRun(%q) coordination metadata = nil", expected.workspaceID)
			}
			if got := claim.CoordinationChannel.WorkspaceID; got != expected.workspaceID {
				t.Fatalf("coordination workspace = %q, want %q", got, expected.workspaceID)
			}
			if got := claim.CoordinationChannel.Purpose; got != expected.purpose {
				t.Fatalf("coordination purpose = %q, want %q", got, expected.purpose)
			}
		}
	})
}

func TestGlobalDBClaimNextRunRespectsSchedulerPause(t *testing.T) {
	t.Run("Should stop new claims while preserving queued runs", func(t *testing.T) {
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 21, 10, 30, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-claim-scheduler-paused")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-claim-scheduler-paused", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.SetSchedulerPaused(ctx, "operator:ops", "maintenance"); err != nil {
			t.Fatalf("SetSchedulerPaused() error = %v", err)
		}

		_, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-paused",
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(paused) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		stored, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("paused run status = %q, want %q", got, want)
		}
		if _, err := globalDB.SetSchedulerResumed(ctx); err != nil {
			t.Fatalf("SetSchedulerResumed() error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-resumed",
			LeaseDuration:    time.Minute,
			Now:              now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(resumed) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("claim.Run.ID = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBClaimNextRunShouldFilterByRunKind(t *testing.T) {
	t.Parallel()

	t.Run("Should filter by run kind", func(t *testing.T) {
		t.Parallel()
		testGlobalDBClaimNextRunShouldFilterByRunKind(t)
	})
}

func testGlobalDBClaimNextRunShouldFilterByRunKind(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 17, 0, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-claim-kind", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	workerTask := taskRecordForTest("task-claim-kind-worker")
	workerTask.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, workerTask); err != nil {
		t.Fatalf("CreateTask(worker) error = %v", err)
	}
	if _, _, _, err := globalDB.ReserveQueuedRun(ctx, queuedRunReservationForTest(
		workerTask.ID,
		"run-claim-kind-worker",
		"claim-kind-worker",
		taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
		nil,
		now,
	)); err != nil {
		t.Fatalf("ReserveQueuedRun(worker) error = %v", err)
	}
	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      string(loopRun.WorkspaceID),
		RunKind:          taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-coordinator",
		ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  "loop-coordinator",
		},
		LeaseDuration: time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(coordinator) error = %v", err)
	}
	if got, want := claim.Run.ID, loopCoordinatorRunID(loopRun.ID, loopRun.Generation+1); got != want {
		t.Fatalf("claim.Run.ID = %q, want %q", got, want)
	}
	workerRun, err := globalDB.GetTaskRun(ctx, "run-claim-kind-worker")
	if err != nil {
		t.Fatalf("GetTaskRun(worker) error = %v", err)
	}
	if workerRun.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("worker run status = %q, want queued", workerRun.Status)
	}
}

func TestGlobalDBClaimNextRunSkipsNeedsAttention(t *testing.T) {
	t.Parallel()

	t.Run("Should not return a run escalated to needs_attention", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-needs-attention")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-needs-attention", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET status = 'needs_attention' WHERE id = ?`,
			run.ID,
		); err != nil {
			t.Fatalf("escalate to needs_attention error = %v", err)
		}

		_, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-needs-attention",
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf(
				"ClaimNextRun(needs_attention) error = %v, want %v",
				err,
				taskpkg.ErrNoClaimableRun,
			)
		}
	})

	t.Run(
		"Should skip a task escalated to needs_attention across a populated queue",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC)
			escalatedTask := taskRecordForTest("task-needs-attention-status")
			escalatedTask.Status = taskpkg.TaskStatusNeedsAttention
			escalatedTask.NeedsAttention = &taskpkg.NeedsAttention{
				Reason: "unblock loop detected",
				At:     now.Add(-time.Minute),
				By:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "breaker"},
			}
			if err := globalDB.CreateTask(ctx, escalatedTask); err != nil {
				t.Fatalf("CreateTask(escalated) error = %v", err)
			}
			escalatedRun := taskRunForTest("run-needs-attention-status", escalatedTask.ID)
			if err := globalDB.CreateTaskRun(ctx, escalatedRun); err != nil {
				t.Fatalf("CreateTaskRun(escalated) error = %v", err)
			}
			readyTask := taskRecordForTest("task-needs-attention-peer")
			readyTask.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, readyTask); err != nil {
				t.Fatalf("CreateTask(ready) error = %v", err)
			}
			readyRun := taskRunForTest("run-needs-attention-peer", readyTask.ID)
			if err := globalDB.CreateTaskRun(ctx, readyRun); err != nil {
				t.Fatalf("CreateTaskRun(ready) error = %v", err)
			}

			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-needs-attention-status",
				LeaseDuration:    time.Minute,
				Now:              now,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(populated needs_attention queue) error = %v", err)
			}
			if got, want := claim.Run.ID, readyRun.ID; got != want {
				t.Fatalf("ClaimNextRun() run id = %q, want %q", got, want)
			}
		},
	)
}

func TestGlobalDBClaimNextRunRespectsEffectiveTaskPause(t *testing.T) {
	t.Run(
		"Should block descendants of a paused task without mutating child rows",
		func(t *testing.T) {
			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
			parent := taskRecordForTest("task-claim-paused-parent")
			parent.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, parent); err != nil {
				t.Fatalf("CreateTask(parent) error = %v", err)
			}
			child := taskRecordForTest("task-claim-paused-child")
			child.Status = taskpkg.TaskStatusReady
			child.ParentTaskID = parent.ID
			if err := globalDB.CreateTask(ctx, child); err != nil {
				t.Fatalf("CreateTask(child) error = %v", err)
			}
			run := taskRunForTest("run-claim-paused-child", child.ID)
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(child) error = %v", err)
			}
			if _, err := globalDB.PauseTask(ctx, taskpkg.PauseMutation{
				TaskID:   parent.ID,
				Actor:    "operator:ops",
				Reason:   "parent incident",
				PausedAt: now,
			}); err != nil {
				t.Fatalf("PauseTask(parent) error = %v", err)
			}

			paused, pausedBy, err := globalDB.IsTaskEffectivelyPaused(ctx, child.ID)
			if err != nil {
				t.Fatalf("IsTaskEffectivelyPaused() error = %v", err)
			}
			if !paused || pausedBy != parent.ID {
				t.Fatalf("effective pause = (%v, %q), want true by %q", paused, pausedBy, parent.ID)
			}
			if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-child-paused",
				LeaseDuration:    time.Minute,
				Now:              now.Add(time.Second),
			}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
				t.Fatalf(
					"ClaimNextRun(paused child) error = %v, want %v",
					err,
					taskpkg.ErrNoClaimableRun,
				)
			}
			storedChild, err := globalDB.GetTask(ctx, child.ID)
			if err != nil {
				t.Fatalf("GetTask(child) error = %v", err)
			}
			if storedChild.Paused {
				t.Fatal("child.Paused = true, want inherited pause without mutating child row")
			}
			if _, err := globalDB.ResumeTask(ctx, taskpkg.ResumeMutation{
				TaskID:    parent.ID,
				ResumedAt: now.Add(2 * time.Second),
			}); err != nil {
				t.Fatalf("ResumeTask(parent) error = %v", err)
			}
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-child-resumed",
				LeaseDuration:    time.Minute,
				Now:              now.Add(3 * time.Second),
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(resumed child) error = %v", err)
			}
			if got, want := claim.Run.ID, run.ID; got != want {
				t.Fatalf("claim.Run.ID = %q, want %q", got, want)
			}
		},
	)
}

func TestGlobalDBClaimNextRunAppliesExecutionProfileEligibility(t *testing.T) {
	t.Run("Should reject ineligible agents and missing profile capabilities", func(t *testing.T) {
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 4, 26, 12, 15, 0, 0, time.UTC)

		taskRecord := taskRecordForTest("task-profile-claim")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-profile-claim", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.UpsertExecutionProfile(ctx, &taskpkg.ExecutionProfile{
			TaskID: taskRecord.ID,
			Worker: taskpkg.WorkerProfile{
				Mode:                 taskpkg.WorkerModeSelect,
				AgentName:            "codex-worker",
				AllowedAgentNames:    []string{"codex-worker"},
				RequiredCapabilities: []string{"golang"},
			},
			Participants: taskpkg.ParticipantPolicy{
				AllowedAgentNames:    []string{"codex-worker"},
				RequiredCapabilities: []string{"sqlite"},
			},
		}); err != nil {
			t.Fatalf("UpsertExecutionProfile() error = %v", err)
		}

		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeGlobal,
			ClaimerSessionID:     "sess-wrong-agent",
			AgentName:            "other-worker",
			RequiredCapabilities: []string{"golang", "sqlite"},
			LeaseDuration:        time.Minute,
			Now:                  now,
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf(
				"ClaimNextRun(wrong agent) error = %v, want %v",
				err,
				taskpkg.ErrNoClaimableRun,
			)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeGlobal,
			ClaimerSessionID:     "sess-missing-agent-name",
			RequiredCapabilities: []string{"golang", "sqlite"},
			LeaseDuration:        time.Minute,
			Now:                  now.Add(500 * time.Millisecond),
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf(
				"ClaimNextRun(blank agent) error = %v, want %v",
				err,
				taskpkg.ErrNoClaimableRun,
			)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeGlobal,
			ClaimerSessionID:     "sess-missing-capability",
			AgentName:            "codex-worker",
			RequiredCapabilities: []string{"golang"},
			LeaseDuration:        time.Minute,
			Now:                  now.Add(time.Second),
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf(
				"ClaimNextRun(missing capability) error = %v, want %v",
				err,
				taskpkg.ErrNoClaimableRun,
			)
		}
		storedQueued, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before eligible claim) error = %v", err)
		}
		if got, want := storedQueued.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("run status before eligible claim = %q, want %q", got, want)
		}

		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeGlobal,
			ClaimerSessionID:     "sess-codex-worker",
			AgentName:            "codex-worker",
			RequiredCapabilities: []string{"golang", "sqlite"},
			LeaseDuration:        time.Minute,
			Now:                  now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(eligible) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun(eligible) run id = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBClaimNextRunFiltersByTaskOwner(t *testing.T) {
	t.Parallel()

	t.Run("Should require a matching pool owner agent name", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"claim-owner-filter",
			filepath.Join(t.TempDir(), "claim-owner-filter"),
		)
		now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

		taskRecord := taskRecordForTest("task-owner-filter")
		taskRecord.Scope = taskpkg.ScopeWorkspace
		taskRecord.WorkspaceID = workspaceID
		taskRecord.Status = taskpkg.TaskStatusReady
		taskRecord.Owner = &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindPool,
			Ref:  "frontend-engineer-agent",
		}
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-owner-filter", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		_, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-wrong-agent",
			AgentName:        "analytics-engineer-agent",
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf(
				"ClaimNextRun(wrong owner) error = %v, want %v",
				err,
				taskpkg.ErrNoClaimableRun,
			)
		}

		stored, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after wrong owner) error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("stored.Status = %q, want %q", got, want)
		}

		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-frontend",
			AgentName:        "frontend-engineer-agent",
			LeaseDuration:    time.Minute,
			Now:              now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(matching owner) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun(matching owner) run = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBClaimNextRunManualAndAgentCreatedRunsSharePrimitive(t *testing.T) {
	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	humanTask := taskRecordForTest("task-human-created-claim")
	humanTask.Status = taskpkg.TaskStatusReady
	humanTask.CreatedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"}
	if err := globalDB.CreateTask(ctx, humanTask); err != nil {
		t.Fatalf("CreateTask(human) error = %v", err)
	}
	humanRun := taskRunForTest("run-human-created-claim", humanTask.ID)
	humanRun.QueuedAt = now
	if err := globalDB.CreateTaskRun(ctx, humanRun); err != nil {
		t.Fatalf("CreateTaskRun(human) error = %v", err)
	}

	agentTask := taskRecordForTest("task-agent-created-claim")
	agentTask.Status = taskpkg.TaskStatusReady
	agentTask.CreatedBy = taskpkg.ActorIdentity{
		Kind: taskpkg.ActorKindAgentSession,
		Ref:  "sess-parent",
	}
	if err := globalDB.CreateTask(ctx, agentTask); err != nil {
		t.Fatalf("CreateTask(agent) error = %v", err)
	}
	agentRun := taskRunForTest("run-agent-created-claim", agentTask.ID)
	agentRun.QueuedAt = now.Add(time.Second)
	if err := globalDB.CreateTaskRun(ctx, agentRun); err != nil {
		t.Fatalf("CreateTaskRun(agent) error = %v", err)
	}

	first, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-worker-1",
		LeaseDuration:    time.Minute,
		Now:              now.Add(10 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(first) error = %v", err)
	}
	if got, want := first.Task.CreatedBy.Kind, taskpkg.ActorKindHuman; got != want {
		t.Fatalf("first.Task.CreatedBy.Kind = %q, want %q", got, want)
	}

	second, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-worker-2",
		LeaseDuration:    time.Minute,
		Now:              now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(second) error = %v", err)
	}
	if got, want := second.Task.CreatedBy.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("second.Task.CreatedBy.Kind = %q, want %q", got, want)
	}
}

func TestGlobalDBClaimNextRunPersistsSoulProvenanceMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should merge pre-resolved soul provenance without reading SOUL.md", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if globalDB == nil {
				return
			}
			if err := globalDB.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		writeInvalidSoulFixture(t, workspaceRoot)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "claim-soul", workspaceRoot)
		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

		taskRecord := taskRecordForTest("task-soul-claim")
		taskRecord.Scope = taskpkg.ScopeWorkspace
		taskRecord.WorkspaceID = workspaceID
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-soul-claim", taskRecord.ID)
		run.Metadata = json.RawMessage(`{"workflow_id":"wf-soul"}`)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		capturedAt := now.Add(-time.Minute)
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-soul",
			AgentName:        "coder",
			Soul: &taskpkg.SoulClaimProvenance{
				SnapshotID: "soul-snapshot-1",
				Digest:     "sha256:resolved",
				AgentName:  "coder",
				CapturedAt: capturedAt,
			},
			LeaseDuration: time.Minute,
			Now:           now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}

		assertRunSoulMetadata(
			t,
			claim.Run.Metadata,
			"wf-soul",
			"soul-snapshot-1",
			"sha256:resolved",
			"coder",
			capturedAt,
		)
		stored, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		assertRunSoulMetadata(
			t,
			stored.Metadata,
			"wf-soul",
			"soul-snapshot-1",
			"sha256:resolved",
			"coder",
			capturedAt,
		)

		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		globalDB = nil
		reopened, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		globalDB = reopened
		reopenedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(reopen) error = %v", err)
		}
		assertRunSoulMetadata(
			t,
			reopenedRun.Metadata,
			"wf-soul",
			"soul-snapshot-1",
			"sha256:resolved",
			"coder",
			capturedAt,
		)
	})
}

func TestGlobalDBClaimLeaseLifecycleFencing(t *testing.T) {
	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	taskRecord := taskRecordForTest("task-lease-lifecycle")
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	firstRun := taskRunForTest("run-lease-lifecycle-first", taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, firstRun); err != nil {
		t.Fatalf("CreateTaskRun(first) error = %v", err)
	}

	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-lease",
		LeaseDuration:    time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	var storedRaw sql.NullString
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_token FROM task_runs WHERE id = ?`, claim.Run.ID).
		Scan(&storedRaw); err != nil {
		t.Fatalf("query claim_token error = %v", err)
	}
	if !storedRaw.Valid || storedRaw.String != claim.ClaimToken {
		t.Fatalf("stored raw claim_token = %#v, want internal active lease token", storedRaw)
	}
	lifecycleRun, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(before lifecycle update) error = %v", err)
	}
	lifecycleRun.Status = taskpkg.TaskRunStatusStarting
	if err := globalDB.UpdateTaskRun(ctx, lifecycleRun); err != nil {
		t.Fatalf("UpdateTaskRun(starting) error = %v", err)
	}
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_token FROM task_runs WHERE id = ?`, claim.Run.ID).
		Scan(&storedRaw); err != nil {
		t.Fatalf("query lifecycle claim_token error = %v", err)
	}
	if !storedRaw.Valid || storedRaw.String != claim.ClaimToken {
		t.Fatalf(
			"lifecycle stored raw claim_token = %#v, want unchanged internal active lease token",
			storedRaw,
		)
	}

	secondRun := taskRunForTest("run-lease-lifecycle-second", taskRecord.ID)
	secondRun.QueuedAt = firstRun.QueuedAt.Add(time.Second)
	if err := globalDB.CreateTaskRun(ctx, secondRun); err != nil {
		t.Fatalf("CreateTaskRun(second) error = %v", err)
	}
	if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-lease",
		LeaseDuration:    time.Minute,
		Now:              now.Add(5 * time.Second),
	}); !errors.Is(err, taskpkg.ErrActiveRunLease) {
		t.Fatalf(
			"ClaimNextRun(second active same session) error = %v, want %v",
			err,
			taskpkg.ErrActiveRunLease,
		)
	}

	if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         claim.Run.ID,
		ClaimToken:    "stale-token",
		LeaseDuration: time.Minute,
		Now:           now.Add(10 * time.Second),
	}); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
		t.Fatalf(
			"HeartbeatRunLease(stale token) error = %v, want %v",
			err,
			taskpkg.ErrInvalidClaimToken,
		)
	}
	if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         claim.Run.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: time.Minute,
		Now:           claim.LeaseUntil,
	}); !errors.Is(err, taskpkg.ErrLeaseExpired) {
		t.Fatalf(
			"HeartbeatRunLease(expired token) error = %v, want %v",
			err,
			taskpkg.ErrLeaseExpired,
		)
	}
	heartbeat, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         claim.Run.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: 2 * time.Minute,
		Now:           now.Add(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("HeartbeatRunLease(current token) error = %v", err)
	}
	if got, want := heartbeat.LeaseUntil, now.Add(150*time.Second); !got.Equal(want) {
		t.Fatalf("heartbeat.LeaseUntil = %v, want %v", got, want)
	}
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_token FROM task_runs WHERE id = ?`, claim.Run.ID).
		Scan(&storedRaw); err != nil {
		t.Fatalf("query heartbeat claim_token error = %v", err)
	}
	if !storedRaw.Valid || storedRaw.String != claim.ClaimToken {
		t.Fatalf(
			"heartbeat stored raw claim_token = %#v, want retained internal active lease token",
			storedRaw,
		)
	}

	if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		Actor:      coordinatorActorContextForTest(),
		RunID:      claim.Run.ID,
		ClaimToken: "stale-token",
		Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":false}`)},
		Now:        now.Add(35 * time.Second),
	}); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
		t.Fatalf(
			"CompleteRunLease(stale token) error = %v, want %v",
			err,
			taskpkg.ErrInvalidClaimToken,
		)
	}
	completed, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		Actor:      coordinatorActorContextForTest(),
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
		Now:        now.Add(40 * time.Second),
	})
	if err != nil {
		t.Fatalf("CompleteRunLease(current token) error = %v", err)
	}
	if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("completed.Status = %q, want %q", got, want)
	}
	if completed.LeaseUntil.IsZero() == false || completed.HeartbeatAt.IsZero() == false {
		t.Fatalf("completed lease fields = lease_until %v heartbeat_at %v, want zero",
			completed.LeaseUntil,
			completed.HeartbeatAt,
		)
	}
	if completed.ClaimTokenHash == "" {
		t.Fatal("completed.ClaimTokenHash = empty, want retained hash")
	}
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_token FROM task_runs WHERE id = ?`, claim.Run.ID).
		Scan(&storedRaw); err != nil {
		t.Fatalf("query completed claim_token error = %v", err)
	}
	if storedRaw.Valid {
		t.Fatalf("completed stored raw claim_token = %q, want NULL", storedRaw.String)
	}

	releaseClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-lease",
		LeaseDuration:    time.Minute,
		Now:              now.Add(45 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(after completion) error = %v", err)
	}
	released, err := globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
		RunID:      releaseClaim.Run.ID,
		ClaimToken: releaseClaim.ClaimToken,
		Reason:     "handoff",
		Now:        now.Add(50 * time.Second),
	})
	if err != nil {
		t.Fatalf("ReleaseRunLease() error = %v", err)
	}
	if got, want := released.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("released.Status = %q, want %q", got, want)
	}
	if released.ClaimTokenHash != "" || released.SessionID != "" || released.ClaimedBy != nil {
		t.Fatalf("released ownership fields = hash %q session %q claimed_by %#v, want cleared",
			released.ClaimTokenHash,
			released.SessionID,
			released.ClaimedBy,
		)
	}

	failClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-lease-fail",
		LeaseDuration:    time.Minute,
		Now:              now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(for failure) error = %v", err)
	}
	failed, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
		Actor:      coordinatorActorContextForTest(),
		RunID:      failClaim.Run.ID,
		ClaimToken: failClaim.ClaimToken,
		Failure:    taskpkg.RunFailure{Error: "worker failed"},
		Now:        now.Add(70 * time.Second),
	})
	if err != nil {
		t.Fatalf("FailRunLease() error = %v", err)
	}
	if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("failed.Status = %q, want %q", got, want)
	}
	if got, want := failed.Error, "worker failed"; got != want {
		t.Fatalf("failed.Error = %q, want %q", got, want)
	}
}

func TestGlobalDBCompleteRunLeaseRejectsHallucinatedCreatedTaskIDsBeforeTerminalWrite(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should reject hallucinated created task ids before terminal write", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-hallucination-gate")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-hallucination-gate", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-gate",
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		before, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before) error = %v", err)
		}

		_, err = globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:          coordinatorActorContextForTest(),
			RunID:          claim.Run.ID,
			ClaimToken:     claim.ClaimToken,
			Result:         taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			CreatedTaskIDs: []string{"task-phantom-globaldb"},
			Now:            now.Add(30 * time.Second),
		})
		if !errors.Is(err, taskpkg.ErrHallucinatedTaskRefs) {
			t.Fatalf("CompleteRunLease() error = %v, want %v", err, taskpkg.ErrHallucinatedTaskRefs)
		}
		var typed *taskpkg.HallucinatedTaskRefsError
		if !errors.As(err, &typed) {
			t.Fatalf("CompleteRunLease() error type = %T, want *HallucinatedTaskRefsError", err)
		}
		if got, want := typed.InvalidTaskIDs, []string{"task-phantom-globaldb"}; len(
			got,
		) != len(
			want,
		) ||
			got[0] != want[0] {
			t.Fatalf("InvalidTaskIDs = %#v, want %#v", got, want)
		}

		after, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after) error = %v", err)
		}
		assertRunLeaseUnchangedAfterHallucinationRejection(t, after, before)
		assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, claim.Run.ID)

		var storedRaw sql.NullString
		if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_token FROM task_runs WHERE id = ?`, claim.Run.ID).
			Scan(&storedRaw); err != nil {
			t.Fatalf("query claim_token error = %v", err)
		}
		if !storedRaw.Valid || storedRaw.String != claim.ClaimToken {
			t.Fatalf("stored raw claim_token = %#v, want internal active lease token", storedRaw)
		}
	})
}

func TestGlobalDBCompleteRunLeaseVerifiesCreatedTaskIDOwnership(t *testing.T) {
	t.Parallel()

	const completingSessionID = "sess-gate-owner"

	tests := []struct {
		name              string
		suffix            string
		childSessionID    string
		childWorkspaceKey string
		wantErr           bool
	}{
		{
			name:              "Should accept existing created task ids from the completing session and workspace",
			suffix:            "valid-owner",
			childSessionID:    completingSessionID,
			childWorkspaceKey: "same",
		},
		{
			name:              "Should reject existing created task ids from another session",
			suffix:            "other-session",
			childSessionID:    "sess-foreign-owner",
			childWorkspaceKey: "same",
			wantErr:           true,
		},
		{
			name:              "Should reject existing created task ids from another workspace",
			suffix:            "other-workspace",
			childSessionID:    completingSessionID,
			childWorkspaceKey: "other",
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC)
			workspaceID := registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"completion-gate-"+tt.suffix,
				filepath.Join(t.TempDir(), "workspace"),
			)
			otherWorkspaceID := registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"completion-gate-other-"+tt.suffix,
				filepath.Join(t.TempDir(), "other-workspace"),
			)

			taskRecord := workspaceTaskRecordForTest("task-gate-owner-"+tt.suffix, workspaceID)
			taskRecord.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask(parent) error = %v", err)
			}
			run := taskRunForTest("run-gate-owner-"+tt.suffix, taskRecord.ID)
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(parent) error = %v", err)
			}

			childWorkspaceID := workspaceID
			if tt.childWorkspaceKey == "other" {
				childWorkspaceID = otherWorkspaceID
			}
			childTask := workspaceTaskRecordForTest("task-gate-child-"+tt.suffix, childWorkspaceID)
			childTask.CreatedBy = taskpkg.ActorIdentity{
				Kind: taskpkg.ActorKindAgentSession,
				Ref:  tt.childSessionID,
			}
			if err := globalDB.CreateTask(ctx, childTask); err != nil {
				t.Fatalf("CreateTask(child) error = %v", err)
			}

			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeWorkspace,
				WorkspaceID:      workspaceID,
				ClaimerSessionID: completingSessionID,
				LeaseDuration:    time.Minute,
				Now:              now,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			before, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(before) error = %v", err)
			}

			completed, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
				Actor:          coordinatorActorContextForTest(),
				RunID:          claim.Run.ID,
				ClaimToken:     claim.ClaimToken,
				Result:         taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
				CreatedTaskIDs: []string{childTask.ID},
				Now:            now.Add(30 * time.Second),
			})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("CompleteRunLease() error = %v", err)
				}
				if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
					t.Fatalf("completed.Status = %q, want %q", got, want)
				}
				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, "")
				return
			}

			if !errors.Is(err, taskpkg.ErrHallucinatedTaskRefs) {
				t.Fatalf(
					"CompleteRunLease() error = %v, want %v",
					err,
					taskpkg.ErrHallucinatedTaskRefs,
				)
			}
			typed, ok := errors.AsType[*taskpkg.HallucinatedTaskRefsError](err)
			if !ok {
				t.Fatalf("CompleteRunLease() error type = %T, want *HallucinatedTaskRefsError", err)
			}
			if got, want := typed.InvalidTaskIDs, []string{childTask.ID}; !slices.Equal(got, want) {
				t.Fatalf("InvalidTaskIDs = %#v, want %#v", got, want)
			}
			after, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(after) error = %v", err)
			}
			assertRunLeaseUnchangedAfterHallucinationRejection(t, after, before)
			assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, claim.Run.ID)
		})
	}
}

func TestGlobalDBRecoverExpiredRunLeasesThenClaim(t *testing.T) {
	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	taskRecord := taskRecordForTest("task-expired-lease-recovery")
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	expiredRun := leasedRunForGlobalTest(
		t,
		"run-expired-lease-recovery",
		taskRecord.ID,
		"sess-expired",
		"expired-token",
		now.Add(-time.Minute),
	)
	if err := globalDB.CreateTaskRun(ctx, expiredRun); err != nil {
		t.Fatalf("CreateTaskRun(expired) error = %v", err)
	}
	unexpiredRun := leasedRunForGlobalTest(
		t,
		"run-unexpired-lease-recovery",
		taskRecord.ID,
		"sess-active",
		"active-token",
		now.Add(time.Minute),
	)
	if err := globalDB.CreateTaskRun(ctx, unexpiredRun); err != nil {
		t.Fatalf("CreateTaskRun(unexpired) error = %v", err)
	}

	if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-before-recovery",
		LeaseDuration:    time.Minute,
		Now:              now,
	}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
		t.Fatalf(
			"ClaimNextRun(before recovery) error = %v, want %v",
			err,
			taskpkg.ErrNoClaimableRun,
		)
	}

	recovered, err := globalDB.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
		Now:    now,
		Reason: "orphaned_on_boot",
	})
	if err != nil {
		t.Fatalf("RecoverExpiredRunLeases() error = %v", err)
	}
	if got, want := len(recovered), 1; got != want {
		t.Fatalf("len(RecoverExpiredRunLeases()) = %d, want %d", got, want)
	}
	if got, want := recovered[0].Run.ID, expiredRun.ID; got != want {
		t.Fatalf("recovered run id = %q, want %q", got, want)
	}
	if got, want := recovered[0].PreviousSessionID, "sess-expired"; got != want {
		t.Fatalf("PreviousSessionID = %q, want %q", got, want)
	}
	if recovered[0].PreviousClaimTokenHash == "" {
		t.Fatal("PreviousClaimTokenHash = empty, want expired hash")
	}
	if got, want := recovered[0].Run.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("recovered status = %q, want %q", got, want)
	}
	if recovered[0].Run.ClaimTokenHash != "" || recovered[0].Run.SessionID != "" {
		t.Fatalf("recovered ownership = hash %q session %q, want cleared",
			recovered[0].Run.ClaimTokenHash,
			recovered[0].Run.SessionID,
		)
	}

	if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         expiredRun.ID,
		ClaimToken:    "expired-token",
		LeaseDuration: time.Minute,
		Now:           now.Add(time.Second),
	}); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
		t.Fatalf(
			"HeartbeatRunLease(stale recovered lease) error = %v, want %v",
			err,
			taskpkg.ErrInvalidClaimToken,
		)
	}

	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-after-recovery",
		LeaseDuration:    time.Minute,
		Now:              now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(after recovery) error = %v", err)
	}
	if got, want := claim.Run.ID, expiredRun.ID; got != want {
		t.Fatalf("ClaimNextRun(after recovery) run id = %q, want %q", got, want)
	}

	active, err := globalDB.GetTaskRun(ctx, unexpiredRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(unexpired) error = %v", err)
	}
	if got, want := active.Status, taskpkg.TaskRunStatusClaimed; got != want {
		t.Fatalf("unexpired status = %q, want %q", got, want)
	}
	if got, want := active.SessionID, "sess-active"; got != want {
		t.Fatalf("unexpired session id = %q, want %q", got, want)
	}

	t.Run("Should exhaust the shared attempt budget after bounded same-row recoveries", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-expired-lease-budget")
		taskRecord.Status = taskpkg.TaskStatusReady
		taskRecord.MaxAttempts = 3
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		run := leasedRunForGlobalTest(
			t,
			"run-expired-lease-budget",
			taskRecord.ID,
			"sess-recovery-0",
			"recovery-token-0",
			now.Add(-time.Second),
		)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		recoverRun := func(at time.Time, wantRecoveryCount int32, wantExhausted bool) taskpkg.Run {
			t.Helper()
			recovered, err := globalDB.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
				Now:    at,
				Reason: "orphaned_on_boot",
			})
			if err != nil {
				t.Fatalf("RecoverExpiredRunLeases() error = %v", err)
			}
			if got, want := len(recovered), 1; got != want {
				t.Fatalf("len(RecoverExpiredRunLeases()) = %d, want %d", got, want)
			}
			if got := recovered[0].Run.RecoveryCount; got != wantRecoveryCount {
				t.Fatalf("RecoveryCount = %d, want %d", got, wantRecoveryCount)
			}
			if got := recovered[0].Exhausted; got != wantExhausted {
				t.Fatalf("Exhausted = %t, want %t", got, wantExhausted)
			}
			return recovered[0].Run
		}

		claimRun := func(at time.Time, sessionID string) taskpkg.ClaimResult {
			t.Helper()
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID:            run.ID,
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: sessionID,
				LeaseDuration:    time.Minute,
				Now:              at,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			return claim
		}

		firstRecovery := recoverRun(now, 1, false)
		if firstRecovery.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("first recovery status = %q, want queued", firstRecovery.Status)
		}
		firstClaim := claimRun(now.Add(time.Second), "sess-recovery-1")
		secondRecovery := recoverRun(firstClaim.LeaseUntil.Add(time.Second), 2, false)
		if secondRecovery.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("second recovery status = %q, want queued", secondRecovery.Status)
		}
		secondClaim := claimRun(firstClaim.LeaseUntil.Add(2*time.Second), "sess-recovery-2")
		exhausted := recoverRun(secondClaim.LeaseUntil.Add(time.Second), 2, true)
		if got, want := exhausted.Status, taskpkg.TaskRunStatusNeedsAttention; got != want {
			t.Fatalf("exhausted status = %q, want %q", got, want)
		}
		if got, want := exhausted.Error, taskpkg.LeaseRecoveryExhaustedReason; got != want {
			t.Fatalf("exhausted error = %q, want %q", got, want)
		}
		if exhausted.SessionID != "" || exhausted.ClaimTokenHash != "" || !exhausted.LeaseUntil.IsZero() {
			t.Fatalf("exhausted ownership = %#v, want cleared", exhausted)
		}

		escalated, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if escalated.NeedsAttention == nil ||
			escalated.NeedsAttention.Reason != taskpkg.LeaseRecoveryExhaustedReason {
			t.Fatalf("NeedsAttention = %#v, want lease recovery exhaustion", escalated.NeedsAttention)
		}
	})
}

func TestGlobalDBTaskCurrentRunProjection(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should set projection when claiming the next run",
			run: func(t *testing.T) {
				globalDB, ctx, taskRecord, run, now := setupCurrentRunProjectionTest(t, "claim")

				claim := claimProjectionRunForTest(ctx, t, globalDB, "sess-projection-claim", now)
				if got, want := claim.Run.ID, run.ID; got != want {
					t.Fatalf("ClaimNextRun() run id = %q, want %q", got, want)
				}
				if got, want := claim.Task.CurrentRunID, run.ID; got != want {
					t.Fatalf("claim.Task.CurrentRunID = %q, want %q", got, want)
				}

				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, run.ID)
				summaries, err := globalDB.ListTasks(ctx, taskpkg.Query{Scope: taskpkg.ScopeGlobal})
				if err != nil {
					t.Fatalf("ListTasks() error = %v", err)
				}
				if got, want := summaries[0].CurrentRunID, run.ID; got != want {
					t.Fatalf("summary.CurrentRunID = %q, want %q", got, want)
				}
			},
		},
		{
			name: "Should clear projection when completing a lease",
			run: func(t *testing.T) {
				globalDB, ctx, taskRecord, _, now := setupCurrentRunProjectionTest(t, "complete")
				claim := claimProjectionRunForTest(
					ctx,
					t,
					globalDB,
					"sess-projection-complete",
					now,
				)

				if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
					Now:        now.Add(10 * time.Second),
				}); err != nil {
					t.Fatalf("CompleteRunLease() error = %v", err)
				}

				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, "")
			},
		},
		{
			name: "Should clear projection when failing a lease",
			run: func(t *testing.T) {
				globalDB, ctx, taskRecord, _, now := setupCurrentRunProjectionTest(t, "fail")
				claim := claimProjectionRunForTest(ctx, t, globalDB, "sess-projection-fail", now)

				if _, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Failure:    taskpkg.RunFailure{Error: "worker failed"},
					Now:        now.Add(10 * time.Second),
				}); err != nil {
					t.Fatalf("FailRunLease() error = %v", err)
				}

				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, "")
			},
		},
		{
			name: "Should clear projection when releasing a lease",
			run: func(t *testing.T) {
				globalDB, ctx, taskRecord, _, now := setupCurrentRunProjectionTest(t, "release")
				claim := claimProjectionRunForTest(ctx, t, globalDB, "sess-projection-release", now)

				if _, err := globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Reason:     "handoff",
					Now:        now.Add(10 * time.Second),
				}); err != nil {
					t.Fatalf("ReleaseRunLease() error = %v", err)
				}

				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, "")
			},
		},
		{
			name: "Should clear projection when recovering an expired lease",
			run: func(t *testing.T) {
				globalDB := openTestGlobalDB(t)
				ctx := testutil.Context(t)
				now := time.Date(2026, 4, 26, 15, 0, 0, 0, time.UTC)
				taskRecord := taskRecordForTest("task-current-projection-recovery")
				taskRecord.Status = taskpkg.TaskStatusReady
				if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}
				run := leasedRunForGlobalTest(
					t,
					"run-current-projection-recovery",
					taskRecord.ID,
					"sess-projection-recovery",
					"expired-projection-token",
					now.Add(-time.Minute),
				)
				if err := globalDB.CreateTaskRun(ctx, run); err != nil {
					t.Fatalf("CreateTaskRun() error = %v", err)
				}
				if _, err := globalDB.db.ExecContext(
					ctx,
					`UPDATE tasks SET current_run_id = ? WHERE id = ?`,
					run.ID,
					taskRecord.ID,
				); err != nil {
					t.Fatalf("seed current_run_id error = %v", err)
				}

				recovered, err := globalDB.RecoverExpiredRunLeases(
					ctx,
					taskpkg.ExpiredLeaseRecovery{
						Now:    now,
						Reason: "orphaned_on_boot",
					},
				)
				if err != nil {
					t.Fatalf("RecoverExpiredRunLeases() error = %v", err)
				}
				if got, want := len(recovered), 1; got != want {
					t.Fatalf("len(RecoverExpiredRunLeases()) = %d, want %d", got, want)
				}

				assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestSetTaskCurrentRunProjectionDetectsConcurrentOverwrite(t *testing.T) {
	t.Parallel()

	globalDB, ctx, taskRecord, run, _ := setupCurrentRunProjectionTest(t, "set-race")
	otherRun := taskRunForTest("run-current-projection-set-race-other", taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, otherRun); err != nil {
		t.Fatalf("CreateTaskRun(other) error = %v", err)
	}

	injected := false
	exec := projectionRaceExecutor{
		taskSQLExecutor: globalDB.db,
		beforeExec: func(ctx context.Context) error {
			if injected {
				return nil
			}
			injected = true
			_, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE tasks SET current_run_id = ? WHERE id = ?`,
				otherRun.ID,
				taskRecord.ID,
			)
			return err
		},
	}

	err := setTaskCurrentRunProjection(ctx, exec, taskRecord.ID, run.ID)
	if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
		t.Fatalf(
			"setTaskCurrentRunProjection() error = %v, want %v",
			err,
			taskpkg.ErrInvalidStatusTransition,
		)
	}
	assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, otherRun.ID)
}

func TestClearTaskCurrentRunProjectionDetectsConcurrentProjectionChange(t *testing.T) {
	t.Parallel()

	globalDB, ctx, taskRecord, run, _ := setupCurrentRunProjectionTest(t, "clear-race")
	otherRun := taskRunForTest("run-current-projection-clear-race-other", taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, otherRun); err != nil {
		t.Fatalf("CreateTaskRun(other) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE tasks SET current_run_id = ? WHERE id = ?`,
		run.ID,
		taskRecord.ID,
	); err != nil {
		t.Fatalf("seed current_run_id error = %v", err)
	}

	injected := false
	exec := projectionRaceExecutor{
		taskSQLExecutor: globalDB.db,
		beforeExec: func(ctx context.Context) error {
			if injected {
				return nil
			}
			injected = true
			_, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE tasks SET current_run_id = ? WHERE id = ?`,
				otherRun.ID,
				taskRecord.ID,
			)
			return err
		},
	}

	err := clearTaskCurrentRunProjection(ctx, exec, taskRecord.ID, run.ID)
	if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
		t.Fatalf(
			"clearTaskCurrentRunProjection() error = %v, want %v",
			err,
			taskpkg.ErrInvalidStatusTransition,
		)
	}
	assertTaskCurrentRunProjection(ctx, t, globalDB, taskRecord.ID, otherRun.ID)
}

func TestGlobalDBClaimNextRunSkipsBlockedTasks(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude blocked tasks and re-admit them when ready", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-blocked-claim")
		taskRecord.Status = taskpkg.TaskStatusBlocked
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-blocked-claim", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-blocked",
			LeaseDuration:    time.Minute,
			Now:              time.Date(2026, 4, 26, 13, 5, 0, 0, time.UTC),
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(blocked) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}

		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.UpdateTask(ctx, taskRecord, coordinatorActorContextForTest()); err != nil {
			t.Fatalf("UpdateTask(ready) error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-unblocked",
			LeaseDuration:    time.Minute,
			Now:              time.Date(2026, 4, 26, 13, 6, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(after ready) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun(after ready) run = %q, want %q", got, want)
		}
	})

	t.Run("Should exclude open block rows before status reconciliation", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 16, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"claim-open-block",
			filepath.Join(t.TempDir(), "workspace"),
		)
		taskRecord := workspaceTaskRecordForTest("task-open-block-claim", workspaceID)
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-open-block-claim", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		blockResult, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskpkg.TaskBlock{
				ID:     "block-open-claim",
				TaskID: taskRecord.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: "creator input required",
				CreatedBy: taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindAgentSession,
					Ref:  "sess-open-block",
				},
				CreatedAt: now,
			},
			RecurrenceLimit: 2,
		})
		if err != nil {
			t.Fatalf("CreateTaskBlock() error = %v", err)
		}
		block := blockResult.Block

		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-open-block-claim",
			LeaseDuration:    time.Minute,
			Now:              now.Add(time.Second),
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(open block) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}

		if _, err := globalDB.ClearTaskBlock(ctx, taskpkg.ClearTaskBlockMutation{
			TaskID:    taskRecord.ID,
			BlockID:   block.ID,
			ClearedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "operator"},
			ClearedAt: now.Add(2 * time.Second),
			Actor:     operatorActorContextForTest("operator"),
		}); err != nil {
			t.Fatalf("ClearTaskBlock() error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-open-block-cleared",
			LeaseDuration:    time.Minute,
			Now:              now.Add(3 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(cleared block) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun(cleared block) run = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBTaskPauseControlsClaimEligibilityAndBacklog(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should skip direct and inherited paused tasks in claim and backlog scans",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			pausedAt := time.Date(2026, 5, 21, 9, 30, 0, 0, time.UTC)

			root := taskRecordForTest("task-pause-root")
			root.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, root); err != nil {
				t.Fatalf("CreateTask(root) error = %v", err)
			}
			child := taskRecordForTest("task-pause-child")
			child.ParentTaskID = root.ID
			child.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, child); err != nil {
				t.Fatalf("CreateTask(child) error = %v", err)
			}
			peer := taskRecordForTest("task-pause-peer")
			peer.Status = taskpkg.TaskStatusReady
			peer.Priority = taskpkg.PriorityHigh
			if err := globalDB.CreateTask(ctx, peer); err != nil {
				t.Fatalf("CreateTask(peer) error = %v", err)
			}
			for _, run := range []taskpkg.Run{
				taskRunForTest("run-pause-child", child.ID),
				taskRunForTest("run-pause-peer", peer.ID),
			} {
				if err := globalDB.CreateTaskRun(ctx, run); err != nil {
					t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
				}
			}
			if _, err := globalDB.PauseTask(ctx, taskpkg.PauseMutation{
				TaskID:   root.ID,
				Actor:    "human:operator",
				Reason:   "provider incident",
				PausedAt: pausedAt,
			}); err != nil {
				t.Fatalf("PauseTask(root) error = %v", err)
			}

			effectivePaused, pausedByTaskID, err := globalDB.IsTaskEffectivelyPaused(ctx, child.ID)
			if err != nil {
				t.Fatalf("IsTaskEffectivelyPaused(child) error = %v", err)
			}
			if !effectivePaused || pausedByTaskID != root.ID {
				t.Fatalf(
					"child effective pause = %v by %q, want true by root",
					effectivePaused,
					pausedByTaskID,
				)
			}
			visibleCount, err := globalDB.CountQueuedTaskRuns(ctx, false)
			if err != nil {
				t.Fatalf("CountQueuedTaskRuns(false) error = %v", err)
			}
			allCount, err := globalDB.CountQueuedTaskRuns(ctx, true)
			if err != nil {
				t.Fatalf("CountQueuedTaskRuns(true) error = %v", err)
			}
			if visibleCount != 1 || allCount != 2 {
				t.Fatalf("queued counts = visible %d all %d, want 1 and 2", visibleCount, allCount)
			}

			backlog, err := globalDB.SchedulerBacklog(
				ctx,
				taskpkg.SchedulerBacklogQuery{IncludePaused: true},
			)
			if err != nil {
				t.Fatalf("SchedulerBacklog(include paused) error = %v", err)
			}
			pausedByRun := make(map[string]taskpkg.SchedulerBacklogRun, len(backlog.Runs))
			for _, item := range backlog.Runs {
				pausedByRun[item.Run.ID] = item
			}
			if item := pausedByRun["run-pause-child"]; !item.EffectivePaused ||
				item.PausedByTaskID != root.ID {
				t.Fatalf("child backlog item = %#v, want inherited pause from root", item)
			}
			if item := pausedByRun["run-pause-peer"]; item.EffectivePaused {
				t.Fatalf("peer backlog item = %#v, want unpaused", item)
			}

			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-pause-peer",
				LeaseDuration:    time.Minute,
				Now:              pausedAt.Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(peer) error = %v", err)
			}
			if got, want := claim.Run.ID, "run-pause-peer"; got != want {
				t.Fatalf("ClaimNextRun(peer) run = %q, want %q", got, want)
			}
			if _, err := globalDB.ResumeTask(ctx, taskpkg.ResumeMutation{
				TaskID:    root.ID,
				ResumedAt: pausedAt.Add(2 * time.Minute),
			}); err != nil {
				t.Fatalf("ResumeTask(root) error = %v", err)
			}
			childClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-pause-child",
				LeaseDuration:    time.Minute,
				Now:              pausedAt.Add(3 * time.Minute),
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(child after resume) error = %v", err)
			}
			if got, want := childClaim.Run.ID, "run-pause-child"; got != want {
				t.Fatalf("ClaimNextRun(child after resume) run = %q, want %q", got, want)
			}
		},
	)

	t.Run("Should filter scheduler backlog by task scope and workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
			ID:      "ws-alpha",
			RootDir: filepath.Join(t.TempDir(), "alpha"),
			Name:    "Alpha",
		})
		insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
			ID:      "ws-beta",
			RootDir: filepath.Join(t.TempDir(), "beta"),
			Name:    "Beta",
		})

		globalTask := taskRecordForTest("task-backlog-global")
		globalTask.Status = taskpkg.TaskStatusReady

		workspaceTask := taskRecordForTest("task-backlog-alpha")
		workspaceTask.Status = taskpkg.TaskStatusReady
		workspaceTask.Scope = taskpkg.ScopeWorkspace
		workspaceTask.WorkspaceID = "ws-alpha"

		otherWorkspaceTask := taskRecordForTest("task-backlog-beta")
		otherWorkspaceTask.Status = taskpkg.TaskStatusReady
		otherWorkspaceTask.Scope = taskpkg.ScopeWorkspace
		otherWorkspaceTask.WorkspaceID = "ws-beta"

		for _, record := range []taskpkg.Task{globalTask, workspaceTask, otherWorkspaceTask} {
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}
		for _, run := range []taskpkg.Run{
			taskRunForTest("run-backlog-global", globalTask.ID),
			taskRunForTest("run-backlog-alpha", workspaceTask.ID),
			taskRunForTest("run-backlog-beta", otherWorkspaceTask.ID),
		} {
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
			}
		}

		globalBacklog, err := globalDB.SchedulerBacklog(ctx, taskpkg.SchedulerBacklogQuery{
			Scope: taskpkg.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("SchedulerBacklog(global) error = %v", err)
		}
		if got, want := schedulerBacklogRunIDs(globalBacklog), []string{"run-backlog-global"}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("SchedulerBacklog(global) runs = %v, want %v", got, want)
		}

		workspaceBacklog, err := globalDB.SchedulerBacklog(ctx, taskpkg.SchedulerBacklogQuery{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-alpha",
		})
		if err != nil {
			t.Fatalf("SchedulerBacklog(workspace) error = %v", err)
		}
		if got, want := schedulerBacklogRunIDs(workspaceBacklog), []string{"run-backlog-alpha"}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("SchedulerBacklog(workspace) runs = %v, want %v", got, want)
		}
	})
}

func schedulerBacklogRunIDs(backlog taskpkg.SchedulerBacklog) []string {
	ids := make([]string, 0, len(backlog.Runs))
	for idx := range backlog.Runs {
		item := &backlog.Runs[idx]
		ids = append(ids, item.Run.ID)
	}
	return ids
}

func setupCurrentRunProjectionTest(
	t *testing.T,
	suffix string,
) (*GlobalDB, context.Context, taskpkg.Task, taskpkg.Run, time.Time) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
	taskRecord := taskRecordForTest("task-current-projection-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-current-projection-"+suffix, taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	return globalDB, ctx, taskRecord, run, now
}

func claimProjectionRunForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	now time.Time,
) taskpkg.ClaimResult {
	t.Helper()

	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: sessionID,
		LeaseDuration:    time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	return claim
}

type projectionRaceExecutor struct {
	taskSQLExecutor
	beforeExec func(ctx context.Context) error
}

func (e projectionRaceExecutor) ExecContext(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	if e.beforeExec != nil {
		if err := e.beforeExec(ctx); err != nil {
			return nil, err
		}
	}
	return e.taskSQLExecutor.ExecContext(ctx, query, args...)
}

func assertTaskCurrentRunProjection(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	want string,
) {
	t.Helper()

	taskRecord, err := globalDB.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", taskID, err)
	}
	if got := taskRecord.CurrentRunID; got != want {
		t.Fatalf("task.CurrentRunID = %q, want %q", got, want)
	}
}

func assertRunLeaseUnchangedAfterHallucinationRejection(
	t *testing.T,
	got taskpkg.Run,
	want taskpkg.Run,
) {
	t.Helper()

	if got.Status != want.Status ||
		got.Attempt != want.Attempt ||
		got.SessionID != want.SessionID ||
		got.ClaimTokenHash != want.ClaimTokenHash ||
		!got.LeaseUntil.Equal(want.LeaseUntil) ||
		!got.HeartbeatAt.Equal(want.HeartbeatAt) ||
		!got.EndedAt.Equal(want.EndedAt) ||
		string(got.Result) != string(want.Result) {
		t.Fatalf("run after rejection = %#v, want lease/state unchanged from %#v", got, want)
	}
}

func leasedRunForGlobalTest(
	t *testing.T,
	id string,
	taskID string,
	sessionID string,
	rawToken string,
	leaseUntil time.Time,
) taskpkg.Run {
	t.Helper()

	hash, err := taskpkg.ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash(%q) error = %v", rawToken, err)
	}
	run := taskRunForTest(id, taskID)
	run.Status = taskpkg.TaskRunStatusClaimed
	run.ClaimedBy = actorForTest(taskpkg.ActorKindAgentSession, sessionID)
	run.SessionID = sessionID
	run.ClaimTokenHash = hash
	run.ClaimedAt = leaseUntil.Add(-time.Minute)
	run.HeartbeatAt = leaseUntil.Add(-30 * time.Second)
	run.LeaseUntil = leaseUntil
	return run
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldRollbackWhenFinalizerFails(t *testing.T) {
	t.Parallel()

	t.Run("Should rollback when finalizer fails", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldRollbackWhenFinalizerFails(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldRollbackWhenFinalizerFails(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-rollback", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-rollback",
		now,
	)

	_, err = globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor: taskpkg.ActorContext{
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.loop"},
			Authority: taskpkg.Authority{Read: true, Write: true, CreateGlobal: true},
		},
		Plan: taskpkg.CoordinatorCompletionPlan{
			Snapshot: taskpkg.GenerationSnapshot{LoopRunID: string(loopRun.ID), Generation: 1},
			Terminal: &taskpkg.CoordinatorTerminal{
				Status: string(looppkg.StatusNoOp),
				Cause:  string(looppkg.TransitionCauseContract),
			},
		},
		Now: now.Add(time.Second),
	}, failingLoopFinalizer{})
	if !errors.Is(err, errForcedFinalizer) {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v, want %v", err, errForcedFinalizer)
	}
	storedRun, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if storedRun.Status != taskpkg.TaskRunStatusClaimed {
		t.Fatalf(
			"stored run status = %q, want %q after rollback",
			storedRun.Status,
			taskpkg.TaskRunStatusClaimed,
		)
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.Status != looppkg.StatusRunning {
		t.Fatalf(
			"loop status = %q, want %q after rollback",
			storedLoop.Status,
			looppkg.StatusRunning,
		)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldPauseAtBoundaryWithoutEnqueue(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should pause at boundary without enqueue", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldPauseAtBoundaryWithoutEnqueue(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldPauseAtBoundaryWithoutEnqueue(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 15, 30, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-pause", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if err := globalDB.SetLoopRunPauseRequested(
		ctx,
		loopRun.WorkspaceID,
		loopRun.ID,
		true,
		coordinatorActorContextForTest(),
	); err != nil {
		t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-pause",
		now,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			Snapshot: taskpkg.GenerationSnapshot{LoopRunID: string(loopRun.ID), Generation: 1},
			NextCoordinator: &taskpkg.EnqueueSpec{
				TaskID:         claim.Run.TaskID,
				RunID:          "run-coordinator-pause-next",
				RunKind:        taskpkg.RunKindCoordinator,
				LoopRunID:      string(loopRun.ID),
				IdempotencyKey: "coordinator-pause-next",
			},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if !result.Paused {
		t.Fatal("result.Paused = false, want true")
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusPaused); got != want {
		t.Fatalf("result loop status = %q, want %q", got, want)
	}
	if len(result.EnqueuedRuns) != 0 {
		t.Fatalf("enqueued runs = %d, want 0 at pause boundary", len(result.EnqueuedRuns))
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.Status != looppkg.StatusPaused {
		t.Fatalf("loop status = %q, want %q", storedLoop.Status, looppkg.StatusPaused)
	}
	if storedLoop.PauseRequested {
		t.Fatal("loop pause_requested = true, want cleared after boundary pause")
	}
	if storedLoop.Generation != 1 {
		t.Fatalf("loop generation = %d, want 1", storedLoop.Generation)
	}
	if _, err := globalDB.GetTaskRun(ctx, "run-coordinator-pause-next"); !errors.Is(
		err,
		taskpkg.ErrTaskRunNotFound,
	) {
		t.Fatalf(
			"GetTaskRun(next coordinator) error = %v, want %v",
			err,
			taskpkg.ErrTaskRunNotFound,
		)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldKeepNodeOutputsPendingWhenBoundarySkipsReservation(
	t *testing.T,
) {
	t.Parallel()

	cases := []struct {
		name       string
		mutateRun  func(*looppkg.Run)
		beforePlan func(context.Context, *testing.T, *GlobalDB, looppkg.Run)
		now        func(time.Time) time.Time
		wantStatus looppkg.Status
	}{
		{
			name: "pause",
			beforePlan: func(ctx context.Context, t *testing.T, db *GlobalDB, run looppkg.Run) {
				t.Helper()
				if err := db.SetLoopRunPauseRequested(
					ctx,
					run.WorkspaceID,
					run.ID,
					true,
					coordinatorActorContextForTest(),
				); err != nil {
					t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
				}
			},
			now:        func(now time.Time) time.Time { return now.Add(time.Second) },
			wantStatus: looppkg.StatusPaused,
		},
		{
			name: "budget",
			mutateRun: func(run *looppkg.Run) {
				run.BudgetWallSec = 1
			},
			beforePlan: func(context.Context, *testing.T, *GlobalDB, looppkg.Run) {},
			now:        func(now time.Time) time.Time { return now.Add(2 * time.Second) },
			wantStatus: looppkg.StatusExhausted,
		},
	}

	for _, tc := range cases {
		t.Run("Should keep pending outputs on "+tc.name+" boundary", func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 4, 15, 35, 0, 0, time.UTC)
			seed := testLoopRun("looprun-coordinator-skip-"+tc.name, now, looppkg.StatusRunning)
			if tc.mutateRun != nil {
				tc.mutateRun(&seed)
			}
			loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			tc.beforePlan(ctx, t, globalDB, loopRun)
			claim := claimCoordinatorRunForTest(
				ctx,
				t,
				globalDB,
				loopRun.ID,
				"run-coordinator-skip-"+tc.name,
				now,
			)
			nodeTaskID := "loop." + string(loopRun.ID) + ".g1.node.load.0"
			nodeRunID := "run.loop." + string(loopRun.ID) + ".g1.node.load.0"

			result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Actor:      coordinatorActorContextForTest(),
				Plan: taskpkg.CoordinatorCompletionPlan{
					NodeTasks: []taskpkg.CoordinatorTaskSpec{{
						TaskID:   nodeTaskID,
						Title:    "Loop delivery node load",
						Metadata: json.RawMessage(`{"node_id":"load"}`),
					}},
					NodeRuns: []taskpkg.EnqueueSpec{{
						TaskID:         nodeTaskID,
						RunID:          nodeRunID,
						RunKind:        taskpkg.RunKindWorker,
						LoopRunID:      string(loopRun.ID),
						IdempotencyKey: "coordinator-skip-node-" + tc.name,
					}},
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  string(loopRun.ID),
						Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
							NodeID: "load",
							Status: "pending",
						}}},
					},
					PostReserveSnapshot: &taskpkg.GenerationSnapshot{
						LoopRunID:  string(loopRun.ID),
						Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
							NodeID:    "load",
							Status:    "enqueued",
							TaskRunID: nodeRunID,
						}}},
					},
				},
				Now: tc.now(now),
			}, looppkg.NewStoreFinalizer())
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
			}
			if got, want := coordinatorResultStatus(t, &result), string(tc.wantStatus); got != want {
				t.Fatalf("loop status = %q, want %q", got, want)
			}
			if len(result.EnqueuedRuns) != 0 {
				t.Fatalf("enqueued runs = %d, want 0", len(result.EnqueuedRuns))
			}
			if _, err := globalDB.GetTaskRun(ctx, nodeRunID); !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
				t.Fatalf("GetTaskRun(node) error = %v, want %v", err, taskpkg.ErrTaskRunNotFound)
			}
			var status string
			var taskRunID sql.NullString
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT status, task_run_id FROM loop_generation_outputs
				  WHERE loop_run_id = ? AND generation = 1 AND node_id = ? AND item_index = 0`,
				string(loopRun.ID),
				"load",
			).Scan(&status, &taskRunID); err != nil {
				t.Fatalf("query generation output error = %v", err)
			}
			if status != "pending" || taskRunID.Valid {
				t.Fatalf("generation output status/task_run_id = %q/%#v, want pending/null", status, taskRunID)
			}
		})
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldDeferBoundaryWhileGenerationInFlight(
	t *testing.T,
) {
	t.Parallel()

	cases := []struct {
		name      string
		mutateRun func(*looppkg.Run)
		before    func(context.Context, *testing.T, *GlobalDB, looppkg.Run)
		now       func(time.Time) time.Time
	}{
		{
			name: "pause",
			before: func(ctx context.Context, t *testing.T, db *GlobalDB, run looppkg.Run) {
				t.Helper()
				if err := db.SetLoopRunPauseRequested(
					ctx,
					run.WorkspaceID,
					run.ID,
					true,
					coordinatorActorContextForTest(),
				); err != nil {
					t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
				}
			},
			now: func(now time.Time) time.Time { return now.Add(time.Second) },
		},
		{
			name: "budget",
			mutateRun: func(run *looppkg.Run) {
				run.BudgetWallSec = 1
			},
			before: func(context.Context, *testing.T, *GlobalDB, looppkg.Run) {},
			now:    func(now time.Time) time.Time { return now.Add(2 * time.Second) },
		},
	}

	for _, tc := range cases {
		t.Run("Should defer "+tc.name+" boundary while generation is in flight", func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 4, 15, 37, 0, 0, time.UTC)
			seed := testLoopRun("looprun-coordinator-inflight-"+tc.name, now, looppkg.StatusRunning)
			if tc.mutateRun != nil {
				tc.mutateRun(&seed)
			}
			loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			tc.before(ctx, t, globalDB, loopRun)
			claim := claimCoordinatorRunForTest(
				ctx,
				t,
				globalDB,
				loopRun.ID,
				"run-coordinator-inflight-"+tc.name,
				now,
			)
			nodeTaskID := "loop." + string(loopRun.ID) + ".g1.node.ready.0"
			nodeRunID := "run.loop." + string(loopRun.ID) + ".g1.node.ready.0"

			result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Actor:      coordinatorActorContextForTest(),
				Plan: taskpkg.CoordinatorCompletionPlan{
					NodeTasks: []taskpkg.CoordinatorTaskSpec{{
						TaskID:   nodeTaskID,
						Title:    "Loop delivery node ready",
						Metadata: json.RawMessage(`{"node_id":"ready"}`),
					}},
					NodeRuns: []taskpkg.EnqueueSpec{{
						TaskID:         nodeTaskID,
						RunID:          nodeRunID,
						RunKind:        taskpkg.RunKindWorker,
						LoopRunID:      string(loopRun.ID),
						IdempotencyKey: "coordinator-inflight-node-" + tc.name,
					}},
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  string(loopRun.ID),
						Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
							{NodeID: "slow", Status: "running", TaskRunID: "run-slow"},
							{NodeID: "ready", Status: "pending"},
						}},
					},
					PostReserveSnapshot: &taskpkg.GenerationSnapshot{
						LoopRunID:  string(loopRun.ID),
						Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
							{NodeID: "slow", Status: "running", TaskRunID: "run-slow"},
							{NodeID: "ready", Status: "enqueued", TaskRunID: nodeRunID},
						}},
					},
					GenerationInFlight: true,
				},
				Now: tc.now(now),
			}, looppkg.NewStoreFinalizer())
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
			}
			if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusRunning); got != want {
				t.Fatalf("loop status = %q, want %q", got, want)
			}
			if len(result.EnqueuedRuns) != 0 {
				t.Fatalf("enqueued runs = %d, want 0 while boundary is deferred", len(result.EnqueuedRuns))
			}
			storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
			if err != nil {
				t.Fatalf("GetLoopRunByID() error = %v", err)
			}
			if storedLoop.Status != looppkg.StatusRunning {
				t.Fatalf("loop status = %q, want %q", storedLoop.Status, looppkg.StatusRunning)
			}
			if tc.name == "pause" && !storedLoop.PauseRequested {
				t.Fatal("pause_requested = false, want it preserved until quiesced boundary")
			}
			if _, err := globalDB.GetTaskRun(ctx, nodeRunID); !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
				t.Fatalf("GetTaskRun(node) error = %v, want %v", err, taskpkg.ErrTaskRunNotFound)
			}
			var status string
			var taskRunID sql.NullString
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT status, task_run_id FROM loop_generation_outputs
				  WHERE loop_run_id = ? AND generation = 1 AND node_id = ? AND item_index = 0`,
				string(loopRun.ID),
				"ready",
			).Scan(&status, &taskRunID); err != nil {
				t.Fatalf("query generation output error = %v", err)
			}
			if status != "pending" || taskRunID.Valid {
				t.Fatalf("generation output status/task_run_id = %q/%#v, want pending/null", status, taskRunID)
			}
		})
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotDispatchWhenLoopIsSuspended(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should not dispatch when loop is suspended", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotDispatchWhenLoopIsSuspended(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotDispatchWhenLoopIsSuspended(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 15, 39, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-suspended", now, looppkg.StatusPaused),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if err := globalDB.CompareAndSwapLoopRunStatus(
		ctx,
		loopRun.ID,
		looppkg.StatusRunning,
		looppkg.StatusPaused,
		looppkg.TransitionCausePauseBoundary,
		now.Add(time.Millisecond),
	); err != nil {
		t.Fatalf("CompareAndSwapLoopRunStatus(paused) error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-suspended",
		now,
	)
	nodeTaskID := "loop." + string(loopRun.ID) + ".g1.node.resume.0"
	nodeRunID := "run.loop." + string(loopRun.ID) + ".g1.node.resume.0"

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			NodeTasks: []taskpkg.CoordinatorTaskSpec{{
				TaskID:   nodeTaskID,
				Title:    "Loop delivery node resume",
				Metadata: json.RawMessage(`{"node_id":"resume"}`),
			}},
			NodeRuns: []taskpkg.EnqueueSpec{{
				TaskID:         nodeTaskID,
				RunID:          nodeRunID,
				RunKind:        taskpkg.RunKindWorker,
				LoopRunID:      string(loopRun.ID),
				IdempotencyKey: "coordinator-suspended-node",
			}},
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					NodeID: "resume",
					Status: "pending",
				}}},
			},
			PostReserveSnapshot: &taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					NodeID:    "resume",
					Status:    "enqueued",
					TaskRunID: nodeRunID,
				}}},
			},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusPaused); got != want {
		t.Fatalf("loop status = %q, want %q", got, want)
	}
	if len(result.EnqueuedRuns) != 0 {
		t.Fatalf("enqueued runs = %d, want 0 for suspended loop", len(result.EnqueuedRuns))
	}
	if _, err := globalDB.GetTaskRun(ctx, nodeRunID); !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		t.Fatalf("GetTaskRun(node) error = %v, want %v", err, taskpkg.ErrTaskRunNotFound)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldResumeWatchingLoopForReadyPoll(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should resume watching loop for ready poll", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldResumeWatchingLoopForReadyPoll(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldResumeWatchingLoopForReadyPoll(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 5, 15, 39, 0, 0, time.UTC)
	liveSpec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-1",
		ChannelStrategy: participation.StrategyLoopRun,
		ChannelID:       "looprun-coordinator-watch-ready",
		Source:          participation.SourceLoopDefinition,
		Bounds: participation.Bounds{
			MaxWakes: 2, MaxWakeWallTime: "1s", MaxTotalWallTime: "2s",
			MaxInputTokens: 100, MaxOutputTokens: 100, MaxWakeDepth: 2, CoalesceWindow: "100ms",
		},
	}
	loopSeed := testLoopRun("looprun-coordinator-watch-ready", now, looppkg.StatusWatching)
	loopSeed.SetNetworkSpec(liveSpec)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		loopSeed,
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if err := globalDB.CompareAndSwapLoopRunStatus(
		ctx,
		loopRun.ID,
		looppkg.StatusRunning,
		looppkg.StatusWatching,
		looppkg.TransitionCauseWatchPoll,
		now.Add(time.Millisecond),
	); err != nil {
		t.Fatalf("CompareAndSwapLoopRunStatus(watching) error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-watch-ready",
		now,
	)
	if got := claim.Run.NetworkSpecSnapshot(); got != liveSpec {
		t.Fatalf("coordinator participation = %#v, want %#v", got, liveSpec)
	}
	nodeTaskID := "loop." + string(loopRun.ID) + ".g1.node.fix_review.0"
	nodeRunID := "run.loop." + string(loopRun.ID) + ".g1.node.fix_review.0"

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			NodeTasks: []taskpkg.CoordinatorTaskSpec{{
				TaskID:   nodeTaskID,
				Title:    "Loop delivery node fix review",
				Metadata: json.RawMessage(`{"node_id":"fix_review"}`),
			}},
			NodeRuns: []taskpkg.EnqueueSpec{{
				TaskID:         nodeTaskID,
				RunID:          nodeRunID,
				RunKind:        taskpkg.RunKindWorker,
				LoopRunID:      string(loopRun.ID),
				IdempotencyKey: "coordinator-watch-ready-node",
			}},
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
					{NodeID: "watch_reviews", Status: "succeeded"},
					{NodeID: "fix_review", Status: "pending"},
				}},
			},
			PostReserveSnapshot: &taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
					{NodeID: "watch_reviews", Status: "succeeded"},
					{NodeID: "fix_review", Status: "enqueued", TaskRunID: nodeRunID},
				}},
			},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusRunning); got != want {
		t.Fatalf("loop status = %q, want %q", got, want)
	}
	if len(result.EnqueuedRuns) != 1 {
		t.Fatalf("enqueued runs = %d, want 1", len(result.EnqueuedRuns))
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.Status != looppkg.StatusRunning {
		t.Fatalf("stored loop status = %q, want running", storedLoop.Status)
	}
	storedNodeRun, err := globalDB.GetTaskRun(ctx, nodeRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(node) error = %v", err)
	}
	if got := storedNodeRun.NetworkSpecSnapshot(); got != liveSpec {
		t.Fatalf("node participation = %#v, want inherited %#v", got, liveSpec)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotPauseWhileYielding(t *testing.T) {
	t.Parallel()

	t.Run("Should not pause while yielding", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotPauseWhileYielding(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldNotPauseWhileYielding(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 15, 45, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-yield-pause", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if err := globalDB.SetLoopRunPauseRequested(
		ctx,
		loopRun.WorkspaceID,
		loopRun.ID,
		true,
		coordinatorActorContextForTest(),
	); err != nil {
		t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-yield-pause",
		now,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			Yield:    true,
			Snapshot: taskpkg.GenerationSnapshot{LoopRunID: string(loopRun.ID), Generation: 1},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if result.Paused {
		t.Fatal("result.Paused = true, want false while yielding")
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusRunning); got != want {
		t.Fatalf("result loop status = %q, want %q", got, want)
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.Status != looppkg.StatusRunning {
		t.Fatalf("loop status = %q, want %q", storedLoop.Status, looppkg.StatusRunning)
	}
	if !storedLoop.PauseRequested {
		t.Fatal("loop pause_requested = false, want intent to remain pending until a real boundary")
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldEnqueuePostCommitWakes(t *testing.T) {
	t.Parallel()

	t.Run("Should enqueue coordinator wake after boundary commit", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldEnqueuePostCommitWakes(t)
	})

	t.Run("Should preserve the committed result when a post-commit wake fails", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldPreserveResultOnWakeFailure(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldEnqueuePostCommitWakes(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 8, 16, 30, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-post-commit-wake", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-post-commit-wake",
		now,
	)
	actor := coordinatorActorContextForTest()
	wakeKey := "loop.coordinator.watch_events." + string(loopRun.ID) + ".watch_task_status"

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      actor,
		Plan: taskpkg.CoordinatorCompletionPlan{
			Yield: true,
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
			},
			PostCommitWakes: []taskpkg.CoordinatorWakeSpec{{
				LoopRunID:      string(loopRun.ID),
				IdempotencyKey: wakeKey,
			}},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusRunning); got != want {
		t.Fatalf("result loop status = %q, want %q", got, want)
	}
	wakeRun, err := globalDB.GetTaskRunByIdempotencyKey(ctx, wakeKey, actor.Origin)
	if err != nil {
		t.Fatalf("GetTaskRunByIdempotencyKey(post-commit wake) error = %v", err)
	}
	if wakeRun.ID == claim.Run.ID {
		t.Fatalf("wake run id = completed coordinator run %q, want new coordinator run", wakeRun.ID)
	}
	if got, want := wakeRun.RunKind, taskpkg.RunKindCoordinator; got != want {
		t.Fatalf("wake run kind = %q, want %q", got, want)
	}
	if got, want := wakeRun.LoopRunID, string(loopRun.ID); got != want {
		t.Fatalf("wake loop_run_id = %q, want %q", got, want)
	}
	if got, want := wakeRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("wake status = %q, want %q", got, want)
	}
	if got, want := wakeRun.IdempotencyKey, wakeKey; got != want {
		t.Fatalf("wake idempotency_key = %q, want %q", got, want)
	}
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldPreserveResultOnWakeFailure(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 8, 16, 35, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-failed-post-commit-wake", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-failed-post-commit-wake",
		now,
	)
	actor := coordinatorActorContextForTest()
	wakeKey := "loop.coordinator.watch_events." + string(loopRun.ID) + ".failed_wake"
	if _, err := globalDB.db.ExecContext(ctx, `
		CREATE TRIGGER fail_coordinator_post_commit_wake
		BEFORE INSERT ON task_runs
		WHEN NEW.idempotency_key = 'loop.coordinator.watch_events.looprun-coordinator-failed-post-commit-wake.failed_wake'
		BEGIN
			SELECT RAISE(ABORT, 'forced coordinator post-commit wake failure');
		END;
	`); err != nil {
		t.Fatalf("create post-commit wake failure trigger error = %v", err)
	}

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      actor,
		Plan: taskpkg.CoordinatorCompletionPlan{
			Yield: true,
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
			},
			PostCommitWakes: []taskpkg.CoordinatorWakeSpec{{
				LoopRunID:      string(loopRun.ID),
				IdempotencyKey: wakeKey,
			}},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err == nil || !strings.Contains(err.Error(), "forced coordinator post-commit wake failure") {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v, want forced wake failure", err)
	}
	if got, want := result.Run.ID, claim.Run.ID; got != want {
		t.Fatalf("result.Run.ID = %q, want committed run %q", got, want)
	}
	if got, want := result.Run.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("result.Run.Status = %q, want %q", got, want)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldLetVerdictPreemptPause(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status looppkg.Status
		cause  looppkg.TransitionCause
		gateID string
	}{
		{name: "terminal", status: looppkg.StatusDone, cause: looppkg.TransitionCauseContract},
		{
			name:   "needs approval",
			status: looppkg.StatusNeedsApproval,
			cause:  looppkg.TransitionCauseBudget,
			gateID: string(looppkg.BudgetGateID),
		},
	}
	for _, tc := range cases {
		t.Run("Should apply "+tc.name+" instead of pending pause", func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 4, 15, 50, 0, 0, time.UTC)
			loopRun, err := globalDB.CreateLoopRunForStart(
				ctx,
				testLoopRun(
					"looprun-coordinator-preempt-pause-"+strings.ReplaceAll(tc.name, " ", "-"),
					now,
					looppkg.StatusRunning,
				),
				dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			if err := globalDB.SetLoopRunPauseRequested(
				ctx,
				loopRun.WorkspaceID,
				loopRun.ID,
				true,
				coordinatorActorContextForTest(),
			); err != nil {
				t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
			}
			claim := claimCoordinatorRunForTest(
				ctx,
				t,
				globalDB,
				loopRun.ID,
				"run-coordinator-preempt-pause-"+strings.ReplaceAll(tc.name, " ", "-"),
				now,
			)

			result, err := globalDB.CompleteCoordinatorAndEnqueueNext(
				ctx,
				taskpkg.CoordinatorCompletion{
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Actor:      coordinatorActorContextForTest(),
					Plan: taskpkg.CoordinatorCompletionPlan{
						Snapshot: taskpkg.GenerationSnapshot{
							LoopRunID:  string(loopRun.ID),
							Generation: 1,
						},
						Terminal: &taskpkg.CoordinatorTerminal{
							Status: string(tc.status),
							Cause:  string(tc.cause),
							GateID: tc.gateID,
						},
					},
					Now: now.Add(time.Second),
				},
				looppkg.NewStoreFinalizer(),
			)
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
			}
			if result.Paused {
				t.Fatal("result.Paused = true, want false when verdict preempts pause")
			}
			if got, want := coordinatorResultStatus(t, &result), string(tc.status); got != want {
				t.Fatalf("result loop status = %q, want %q", got, want)
			}
			storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
			if err != nil {
				t.Fatalf("GetLoopRunByID() error = %v", err)
			}
			if storedLoop.Status != tc.status {
				t.Fatalf("loop status = %q, want %q", storedLoop.Status, tc.status)
			}
			if storedLoop.PauseRequested {
				t.Fatal("loop pause_requested = true, want cleared by truthful verdict")
			}
		})
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldRefreshTokensAndApplyBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		exceeded   dsl.BudgetExceeded
		wantStatus looppkg.Status
	}{
		{name: "halt", exceeded: dsl.BudgetExceededHalt, wantStatus: looppkg.StatusExhausted},
		{
			name:       "escalate",
			exceeded:   dsl.BudgetExceededEscalate,
			wantStatus: looppkg.StatusNeedsApproval,
		},
	}
	for _, tc := range cases {
		t.Run("Should apply "+tc.name+" after refreshing task-run token sum", func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 4, 16, 0, 0, 0, time.UTC)
			seed := testLoopRun("looprun-coordinator-budget-"+tc.name, now, looppkg.StatusRunning)
			seed.BudgetTokens = 5
			seed.BudgetOnExceeded = tc.exceeded
			loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			createCompletedLoopWorkerRunForTest(
				ctx,
				t,
				globalDB,
				loopRun.ID,
				"worker-budget-"+tc.name,
				5,
				now,
			)
			claim := claimCoordinatorRunForTest(
				ctx,
				t,
				globalDB,
				loopRun.ID,
				"run-coordinator-budget-"+tc.name,
				now,
			)

			result, err := globalDB.CompleteCoordinatorAndEnqueueNext(
				ctx,
				taskpkg.CoordinatorCompletion{
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Actor:      coordinatorActorContextForTest(),
					Plan: taskpkg.CoordinatorCompletionPlan{
						Snapshot: taskpkg.GenerationSnapshot{
							LoopRunID:  string(loopRun.ID),
							Generation: 1,
						},
						NextCoordinator: &taskpkg.EnqueueSpec{
							TaskID:         claim.Run.TaskID,
							RunID:          "run-coordinator-budget-next-" + tc.name,
							RunKind:        taskpkg.RunKindCoordinator,
							LoopRunID:      string(loopRun.ID),
							IdempotencyKey: "coordinator-budget-next-" + tc.name,
						},
					},
					Now: now.Add(time.Second),
				},
				looppkg.NewStoreFinalizer(),
			)
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
			}
			if got, want := coordinatorResultStatus(t, &result), string(tc.wantStatus); got != want {
				t.Fatalf("result loop status = %q, want %q", got, want)
			}
			if !result.Terminal {
				t.Fatal("result.Terminal = false, want true for budget boundary")
			}
			if result.TokensUsed != 5 {
				t.Fatalf("result.TokensUsed = %d, want 5", result.TokensUsed)
			}
			if len(result.EnqueuedRuns) != 0 {
				t.Fatalf(
					"enqueued runs = %d, want 0 when budget is exceeded",
					len(result.EnqueuedRuns),
				)
			}
			storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
			if err != nil {
				t.Fatalf("GetLoopRunByID() error = %v", err)
			}
			if storedLoop.Status != tc.wantStatus {
				t.Fatalf("loop status = %q, want %q", storedLoop.Status, tc.wantStatus)
			}
			if storedLoop.TokensUsed != 5 {
				t.Fatalf("loop tokens_used = %d, want refreshed sum 5", storedLoop.TokensUsed)
			}
			if tc.wantStatus == looppkg.StatusNeedsApproval {
				if storedLoop.ActiveGateID != looppkg.BudgetGateID {
					t.Fatalf("loop active_gate_id = %q, want %q", storedLoop.ActiveGateID, looppkg.BudgetGateID)
				}
				if storedLoop.BudgetApprovalSeq != 1 {
					t.Fatalf("loop budget_approval_seq = %d, want 1", storedLoop.BudgetApprovalSeq)
				}
				events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
					WorkspaceID: loopRun.WorkspaceID,
					RunID:       loopRun.ID,
				})
				if err != nil {
					t.Fatalf("ListLoopRunEvents() error = %v", err)
				}
				payload := loopEventPayloadForKind(t, events, loopRunEventNeedsApproval)
				if got, want := payload["gate_id"], string(looppkg.BudgetGateID); got != want {
					t.Fatalf("needs_approval.gate_id = %#v, want %q", got, want)
				}
			}
		})
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyWallClockBudget(t *testing.T) {
	t.Parallel()

	t.Run("Should apply wall-clock budget", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyWallClockBudget(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyWallClockBudget(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 16, 20, 0, 0, time.UTC)
	seed := testLoopRun("looprun-coordinator-wall-budget", now, looppkg.StatusRunning)
	seed.BudgetWallSec = 10
	seed.BudgetOnExceeded = dsl.BudgetExceededHalt
	loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-wall-budget",
		now,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Actor:      coordinatorActorContextForTest(),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID:  string(loopRun.ID),
					Generation: 1,
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         claim.Run.TaskID,
					RunID:          "run-coordinator-wall-budget-next",
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(loopRun.ID),
					IdempotencyKey: "coordinator-wall-budget-next",
				},
			},
			Now: now.Add(11 * time.Second),
		},
		looppkg.NewStoreFinalizer(),
	)
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusExhausted); got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !result.Terminal {
		t.Fatal("Terminal = false, want true for wall-clock budget")
	}
	if len(result.EnqueuedRuns) != 0 {
		t.Fatalf("enqueued runs = %d, want 0 after wall-clock budget", len(result.EnqueuedRuns))
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldUseStartedAtForWallClockBudget(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	startedAt := time.Date(2026, 7, 4, 16, 35, 0, 0, time.UTC)
	seed := testLoopRun("looprun-coordinator-wall-started-at", startedAt, looppkg.StatusRunning)
	seed.CreatedAt = startedAt.Add(-time.Hour)
	seed.StartedAt = startedAt
	seed.BudgetWallSec = 10
	seed.BudgetOnExceeded = dsl.BudgetExceededHalt
	loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-wall-started-at",
		startedAt,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Actor:      coordinatorActorContextForTest(),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID:  string(loopRun.ID),
					Generation: 1,
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         claim.Run.TaskID,
					RunID:          "run-coordinator-wall-started-at-next",
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(loopRun.ID),
					IdempotencyKey: "coordinator-wall-started-at-next",
				},
			},
			Now: startedAt.Add(9 * time.Second),
		},
		looppkg.NewStoreFinalizer(),
	)
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if result.Terminal {
		t.Fatal("Terminal = true, want continue before started_at wall-clock budget expires")
	}
	if len(result.EnqueuedRuns) != 1 {
		t.Fatalf("enqueued runs = %d, want 1 before started_at budget expires", len(result.EnqueuedRuns))
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.Status != looppkg.StatusRunning {
		t.Fatalf("loop status = %q, want running", storedLoop.Status)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldConsumeBudgetApprovalOnce(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 16, 45, 0, 0, time.UTC)
	seed := testLoopRun("looprun-coordinator-budget-approval-once", now, looppkg.StatusRunning)
	seed.BudgetTokens = 5
	seed.BudgetOnExceeded = dsl.BudgetExceededHalt
	seed.BudgetApprovalSeq = 1
	loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	createCompletedLoopWorkerRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"worker-budget-approval-once",
		5,
		now,
	)
	firstClaim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-budget-approval-once",
		now,
	)

	first, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID:      firstClaim.Run.ID,
			ClaimToken: firstClaim.ClaimToken,
			Actor:      coordinatorActorContextForTest(),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID:  string(loopRun.ID),
					Generation: 1,
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         firstClaim.Run.TaskID,
					RunID:          "run-coordinator-budget-approval-next",
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(loopRun.ID),
					IdempotencyKey: "coordinator-budget-approval-next",
				},
			},
			Now: now.Add(time.Second),
		},
		looppkg.NewStoreFinalizer(),
	)
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(first) error = %v", err)
	}
	if first.Terminal {
		t.Fatal("first.Terminal = true, want one approved continuation")
	}
	if len(first.EnqueuedRuns) != 1 {
		t.Fatalf("first enqueued runs = %d, want 1", len(first.EnqueuedRuns))
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(first) error = %v", err)
	}
	if storedLoop.BudgetApprovalSeq != 0 {
		t.Fatalf("budget_approval_seq after first boundary = %d, want consumed 0", storedLoop.BudgetApprovalSeq)
	}

	secondClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            first.EnqueuedRuns[0].ID,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      string(loopRun.WorkspaceID),
		RunKind:          taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-budget-approval-second",
		ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    time.Minute,
		Now:              now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(second) error = %v", err)
	}
	second, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID:      secondClaim.Run.ID,
			ClaimToken: secondClaim.ClaimToken,
			Actor:      coordinatorActorContextForTest(),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID:  string(loopRun.ID),
					Generation: 1,
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         secondClaim.Run.TaskID,
					RunID:          "run-coordinator-budget-approval-third",
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(loopRun.ID),
					IdempotencyKey: "coordinator-budget-approval-third",
				},
			},
			Now: now.Add(3 * time.Second),
		},
		looppkg.NewStoreFinalizer(),
	)
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(second) error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &second), string(looppkg.StatusExhausted); got != want {
		t.Fatalf("second loop status = %q, want %q", got, want)
	}
	if !second.Terminal {
		t.Fatal("second.Terminal = false, want exhausted after approval is consumed")
	}
	if len(second.EnqueuedRuns) != 0 {
		t.Fatalf("second enqueued runs = %d, want 0", len(second.EnqueuedRuns))
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyRunStops(t *testing.T) {
	t.Parallel()

	t.Run("Should apply run stops", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyRunStops(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldApplyRunStops(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 16, 25, 0, 0, time.UTC)
	parentRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-stop-parent", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(parent) error = %v", err)
	}
	childSeed := testLoopRun("looprun-coordinator-stop-child", now, looppkg.StatusRunning)
	childSeed.ParentLoopRunID = parentRun.ID
	childRun, err := globalDB.CreateLoopRunForStart(ctx, childSeed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(child) error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		parentRun.ID,
		"run-coordinator-stop-parent",
		now,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			RunStops: []taskpkg.CoordinatorStopSpec{{
				LoopRunID:  string(childRun.ID),
				ReasonCode: "child_loop_timeout",
			}},
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(parentRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					NodeID:         "load",
					Status:         "failed",
					OutputRef:      "child_loop_timeout",
					ChildLoopRunID: string(childRun.ID),
				}}},
			},
			Terminal: &taskpkg.CoordinatorTerminal{
				Status:     string(looppkg.StatusFailed),
				Cause:      string(looppkg.TransitionCauseContract),
				ReasonCode: "child_loop_timeout",
			},
		},
		Now: now.Add(2 * time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusFailed); got != want {
		t.Fatalf("parent status = %q, want %q", got, want)
	}
	storedChild, err := globalDB.GetLoopRunByID(ctx, childRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(child) error = %v", err)
	}
	if got, want := storedChild.Status, looppkg.StatusFailed; got != want {
		t.Fatalf("child status = %q, want %q", got, want)
	}
}

func TestGlobalDBReconcileLoopCoordinatorsOnBootShouldPromoteOldestQueuedRun(t *testing.T) {
	t.Parallel()

	t.Run("Should promote oldest queued run", func(t *testing.T) {
		t.Parallel()
		testGlobalDBReconcileLoopCoordinatorsOnBootShouldPromoteOldestQueuedRun(t)
	})
}

func testGlobalDBReconcileLoopCoordinatorsOnBootShouldPromoteOldestQueuedRun(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 16, 28, 0, 0, time.UTC)
	activeRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-queue-active", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(active) error = %v", err)
	}
	coordinatorTask := taskRecordForTest("task-coordinator-queue-promote")
	coordinatorTask.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, coordinatorTask); err != nil {
		t.Fatalf("CreateTask(coordinator) error = %v", err)
	}
	coordinatorRun := taskRunForTest("run-coordinator-queue-promote-seed", coordinatorTask.ID)
	coordinatorRun.RunKind = taskpkg.RunKindCoordinator
	coordinatorRun.LoopRunID = string(activeRun.ID)
	coordinatorRun.Status = taskpkg.TaskRunStatusCompleted
	coordinatorRun.EndedAt = now.Add(time.Second)
	if err := globalDB.CreateTaskRun(ctx, coordinatorRun); err != nil {
		t.Fatalf("CreateTaskRun(coordinator seed) error = %v", err)
	}
	queuedOne, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-queue-one", now.Add(time.Second), looppkg.StatusRunning),
		dsl.ConcurrencyQueue,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(queued one) error = %v", err)
	}
	queuedTwo, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-queue-two", now.Add(2*time.Second), looppkg.StatusRunning),
		dsl.ConcurrencyQueue,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(queued two) error = %v", err)
	}
	if queuedOne.Status != looppkg.StatusQueued || queuedTwo.Status != looppkg.StatusQueued {
		t.Fatalf("queued statuses = %q/%q, want queued/queued", queuedOne.Status, queuedTwo.Status)
	}
	if err := globalDB.CompareAndSwapLoopRunStatus(
		ctx,
		activeRun.ID,
		looppkg.StatusRunning,
		looppkg.StatusDone,
		looppkg.TransitionCauseContract,
		now.Add(3*time.Second),
	); err != nil {
		t.Fatalf("CompareAndSwapLoopRunStatus(active done) error = %v", err)
	}

	enqueued, err := globalDB.ReconcileLoopCoordinatorsOnBoot(
		ctx,
		taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "boot-test"},
		now.Add(4*time.Second),
	)
	if err != nil {
		t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
	}
	if got, want := len(enqueued), 1; got != want {
		t.Fatalf("enqueued coordinators = %d, want %d", got, want)
	}
	if got, want := enqueued[0].LoopRunID, string(queuedOne.ID); got != want {
		t.Fatalf("enqueued LoopRunID = %q, want %q", got, want)
	}
	storedOne, err := globalDB.GetLoopRunByID(ctx, queuedOne.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(queued one) error = %v", err)
	}
	if got, want := storedOne.Status, looppkg.StatusRunning; got != want {
		t.Fatalf("queued one status = %q, want %q", got, want)
	}
	storedTwo, err := globalDB.GetLoopRunByID(ctx, queuedTwo.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(queued two) error = %v", err)
	}
	if got, want := storedTwo.Status, looppkg.StatusQueued; got != want {
		t.Fatalf("queued two status = %q, want %q", got, want)
	}
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldCreateNodeTasksDependenciesAndRuns(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should create node tasks dependencies and runs", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldCreateNodeTasksDependenciesAndRuns(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldCreateNodeTasksDependenciesAndRuns(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 16, 30, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-materialize", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-coordinator-materialize",
		now,
	)
	rootTaskID := "loop.looprun-coordinator-materialize.g1.node.load.0"
	childTaskID := "loop.looprun-coordinator-materialize.g1.node.agent.0"
	rootRunID := "run.loop.looprun-coordinator-materialize.g1.node.load.0"

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			NodeTasks: []taskpkg.CoordinatorTaskSpec{
				{
					TaskID:   rootTaskID,
					Title:    "Loop delivery node load",
					Metadata: json.RawMessage(`{"node_id":"load"}`),
				},
				{
					TaskID:   childTaskID,
					Title:    "Loop delivery node agent",
					Metadata: json.RawMessage(`{"node_id":"agent"}`),
				},
			},
			Dependencies: []taskpkg.CoordinatorDependencySpec{
				{
					TaskID:          childTaskID,
					DependsOnTaskID: rootTaskID,
					Kind:            taskpkg.DependencyKindBlocks,
				},
			},
			NodeRuns: []taskpkg.EnqueueSpec{
				{
					TaskID:         rootTaskID,
					RunID:          rootRunID,
					RunKind:        taskpkg.RunKindWorker,
					LoopRunID:      string(loopRun.ID),
					IdempotencyKey: "coordinator-materialize-root",
				},
			},
			Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
					{NodeID: "load", Status: "pending"},
					{NodeID: "agent", Status: "pending"},
				}},
			},
			PostReserveSnapshot: &taskpkg.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
					{NodeID: "load", Status: "enqueued", TaskRunID: rootRunID},
					{NodeID: "agent", Status: "pending"},
				}},
			},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := len(result.EnqueuedRuns), 1; got != want {
		t.Fatalf("enqueued runs = %d, want %d", got, want)
	}
	rootTask, err := globalDB.GetTask(ctx, rootTaskID)
	if err != nil {
		t.Fatalf("GetTask(root) error = %v", err)
	}
	if rootTask.AutoEnqueueOnReady {
		t.Fatal("root task AutoEnqueueOnReady = true, want coordinator-controlled enqueue")
	}
	childTask, err := globalDB.GetTask(ctx, childTaskID)
	if err != nil {
		t.Fatalf("GetTask(child) error = %v", err)
	}
	if childTask.ParentTaskID != claim.Run.TaskID {
		t.Fatalf("child ParentTaskID = %q, want %q", childTask.ParentTaskID, claim.Run.TaskID)
	}
	dependencies, err := globalDB.ListDependencies(ctx, childTaskID)
	if err != nil {
		t.Fatalf("ListDependencies(child) error = %v", err)
	}
	if len(dependencies) != 1 || dependencies[0].DependsOnTaskID != rootTaskID {
		t.Fatalf("dependencies = %#v, want one dependency on %q", dependencies, rootTaskID)
	}
	rootRun, err := globalDB.GetTaskRun(ctx, rootRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(root) error = %v", err)
	}
	if rootRun.Status != taskpkg.TaskRunStatusQueued || rootRun.LoopRunID != string(loopRun.ID) {
		t.Fatalf("root run = %#v, want queued worker for loop run", rootRun)
	}
	var status string
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT status FROM loop_generation_outputs
		  WHERE loop_run_id = ? AND generation = 1 AND node_id = ? AND item_index = 0`,
		string(loopRun.ID),
		"load",
	).Scan(&status); err != nil {
		t.Fatalf("query generation output status error = %v", err)
	}
	if status != "enqueued" {
		t.Fatalf("generation output status = %q, want enqueued", status)
	}
}

func TestGlobalDBRunLeaseTerminalShouldRecordLoopNodeProgress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		complete         bool
		result           json.RawMessage
		failureMetadata  json.RawMessage
		tokensUsed       int64
		wantOutputStatus string
		wantOutputRef    string
		wantFailureCode  string
		wantFailureCause string
		wantRecovery     string
		wantEvents       []string
	}{
		{
			name:             "success records terminal output",
			complete:         true,
			result:           json.RawMessage(`{"message":"approved agh_claim_SECRET123"}`),
			tokensUsed:       3,
			wantOutputStatus: "succeeded",
			wantOutputRef:    `{"message":"approved agh_claim_SECRET123"}`,
			wantEvents: []string{
				loopRunEventStatusChanged,
				loopRunEventNodeRunning,
				loopRunEventNodeSucceeded,
				loopRunEventChannelMsg,
				loopRunEventTokenTick,
			},
		},
		{
			name:             "failure records terminal output",
			complete:         false,
			tokensUsed:       5,
			wantOutputStatus: "failed",
			wantOutputRef:    "credential_missing",
			wantEvents: []string{
				loopRunEventStatusChanged,
				loopRunEventNodeRunning,
				loopRunEventNodeFailed,
				loopRunEventTokenTick,
			},
		},
		{
			name:     "action failure preserves operator detail",
			complete: false,
			failureMetadata: json.RawMessage(
				`{"reason_code":"loop_action_failed","failure":{"kind":"action_failure","code":"tool_invalid_input","cause":"No task set matched .compozy/tasks/helix-v1-launch/task_*.md.","recovery":"Create the matching task set or correct the Loop input, then retry the run."}}`,
			),
			tokensUsed:       5,
			wantOutputStatus: "failed",
			wantFailureCode:  "tool_invalid_input",
			wantFailureCause: "No task set matched .compozy/tasks/helix-v1-launch/task_*.md.",
			wantRecovery:     "Create the matching task set or correct the Loop input, then retry the run.",
			wantEvents: []string{
				loopRunEventStatusChanged,
				loopRunEventNodeRunning,
				loopRunEventNodeFailed,
				loopRunEventTokenTick,
			},
		},
	}
	for _, tc := range cases {
		t.Run("Should record loop node terminal progress on "+tc.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 4, 17, 0, 0, 0, time.UTC)
			terminalAt := now.Add(2 * time.Second)
			seed := testLoopRun(
				"looprun-node-terminal-"+strings.ReplaceAll(tc.name, " ", "-"),
				now,
				looppkg.StatusRunning,
			)
			loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			taskRecord := taskRecordForTest(
				"task-node-terminal-" + strings.ReplaceAll(tc.name, " ", "-"),
			)
			taskRecord.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			runID := "run-node-terminal-" + strings.ReplaceAll(tc.name, " ", "-")
			metadata := json.RawMessage(`{"generation":1,"node_id":"load","item_index":0}`)
			reservation := queuedRunReservationForTest(
				taskRecord.ID,
				runID,
				"node-terminal-"+strings.ReplaceAll(tc.name, " ", "-"),
				taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
				metadata,
				now,
			)
			reservation.RunKind = taskpkg.RunKindWorker
			reservation.LoopRunID = string(loopRun.ID)
			if _, _, _, err := globalDB.ReserveQueuedRun(ctx, reservation); err != nil {
				t.Fatalf("ReserveQueuedRun(worker) error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_generation_outputs (
					loop_run_id, generation, node_id, item_index, status, task_run_id
				) VALUES (?, 1, 'load', 0, 'enqueued', ?)`,
				string(loopRun.ID),
				runID,
			); err != nil {
				t.Fatalf("insert generation output error = %v", err)
			}
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "worker-" + strings.ReplaceAll(tc.name, " ", "-"),
				ClaimedBy: &taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindDaemon,
					Ref:  "worker",
				},
				LeaseDuration: time.Minute,
				Now:           now,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			if tc.complete {
				if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Result:     taskpkg.RunResult{Value: tc.result},
					TokensUsed: tc.tokensUsed,
					Now:        terminalAt,
				}); err != nil {
					t.Fatalf("CompleteRunLease() error = %v", err)
				}
			} else {
				metadata := tc.failureMetadata
				if len(metadata) == 0 {
					metadata = json.RawMessage(`{"reason_code":"credential_missing"}`)
				}
				if _, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: claim.ClaimToken,
					Failure: taskpkg.RunFailure{
						Error:    "missing credential",
						Metadata: metadata,
					},
					TokensUsed: tc.tokensUsed,
					Now:        terminalAt,
				}); err != nil {
					t.Fatalf("FailRunLease() error = %v", err)
				}
			}
			var status string
			var outputRef sql.NullString
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT status, output_ref
				 FROM loop_generation_outputs
				 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'load' AND item_index = 0`,
				string(loopRun.ID),
			).Scan(&status, &outputRef); err != nil {
				t.Fatalf("query generation output error = %v", err)
			}
			if status != tc.wantOutputStatus {
				t.Fatalf("output status = %q, want %q", status, tc.wantOutputStatus)
			}
			if tc.wantFailureCode != "" {
				var failure struct {
					Kind     string `json:"kind"`
					Code     string `json:"code"`
					Cause    string `json:"cause"`
					Recovery string `json:"recovery"`
				}
				if err := json.Unmarshal([]byte(outputRef.String), &failure); err != nil {
					t.Fatalf("decode action failure output_ref error = %v; output_ref=%q", err, outputRef.String)
				}
				if failure.Kind != "action_failure" || failure.Code != tc.wantFailureCode ||
					failure.Cause != tc.wantFailureCause || failure.Recovery != tc.wantRecovery {
					t.Fatalf(
						"action failure output_ref = %#v, want code/cause/recovery %q/%q/%q",
						failure,
						tc.wantFailureCode,
						tc.wantFailureCause,
						tc.wantRecovery,
					)
				}
			} else if outputRef.String != tc.wantOutputRef {
				t.Fatalf("output_ref = %q, want %q", outputRef.String, tc.wantOutputRef)
			}
			storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
			if err != nil {
				t.Fatalf("GetLoopRunByID() error = %v", err)
			}
			if !storedLoop.LastProgressAt.Equal(terminalAt) {
				t.Fatalf("last_progress_at = %s, want %s", storedLoop.LastProgressAt, terminalAt)
			}
			if storedLoop.TokensUsed != tc.tokensUsed {
				t.Fatalf("loop tokens_used = %d, want %d", storedLoop.TokensUsed, tc.tokensUsed)
			}
			events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
				WorkspaceID: loopRun.WorkspaceID,
				RunID:       loopRun.ID,
			})
			if err != nil {
				t.Fatalf("ListLoopRunEvents() error = %v", err)
			}
			assertLoopEventKinds(t, events, tc.wantEvents)
			tick := loopEventPayloadForKind(t, events, loopRunEventTokenTick)
			if got := int64(tick["tokens_used"].(float64)); got != tc.tokensUsed {
				t.Fatalf("token_tick.tokens_used = %d, want %d", got, tc.tokensUsed)
			}
			if tc.complete {
				channel := loopEventPayloadForKind(t, events, loopRunEventChannelMsg)
				text := channel["text"].(string)
				if strings.Contains(text, "agh_claim_SECRET123") {
					t.Fatalf("channel_msg.text leaked raw claim token: %q", text)
				}
				if !strings.Contains(text, "agh_claim_[REDACTED]") {
					t.Fatalf("channel_msg.text = %q, want redacted claim token marker", text)
				}
			}
		})
	}
}

func TestGlobalDBCompleteRunLeaseShouldCommitCoordinatorControlWithoutNodeSuccess(t *testing.T) {
	t.Run("Should preserve task completion while leaving the Goal output control pending", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
		definition := dsl.Definition{
			APIVersion: dsl.APIVersion,
			Kind:       dsl.KindLoop,
			Meta:       dsl.Meta{Name: "goal-control-pending", Version: 1},
			Contract: dsl.Contract{
				Goal:             "Settle Goal control truthfully",
				DefinitionOfDone: "The Goal control boundary is durable",
				IterationCap:     3,
				NoProgress:       dsl.NoProgress{Window: 1},
			},
			Graph: dsl.Graph{Nodes: []dsl.Node{{
				ID:    "converge",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionGoal),
				Params: dsl.NodeParams{
					"agent":     "worker",
					"objective": "Reach the Goal control boundary",
					"judge": []any{
						map[string]any{"id": "done", "type": "agent-judge", "rubric": "Approve when done"},
					},
					"max_turns":    3,
					"on_exhausted": dsl.GoalOnExhaustedHalt,
					"output_schema": map[string]any{
						"status": map[string]any{
							"type": "string",
							"enum": []any{"complete", "blocked"},
						},
					},
				},
			}}},
		}
		resolvedDefinition, err := looppkg.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(Goal definition) error = %v", err)
		}
		effective, err := looppkg.ResolveEffectiveConfig(
			resolvedDefinition,
			looppkg.DefaultLoopDefaults(),
			nil,
			looppkg.LoopConfig{},
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(
			resolvedDefinition,
			effective,
		)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		seed := testLoopRun("looprun-goal-control-pending", now, looppkg.StatusRunning)
		seed.Generation = 1
		seed.DefinitionVersion = resolvedDefinition.DefinitionVersion
		seed.DefinitionDigest = digest
		seed.DefinitionSnapshot = snapshot
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			seed,
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		taskRecord := taskRecordForTest("task-goal-control-pending")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		const runID = "run-goal-control-pending"
		reservation := queuedRunReservationForTest(
			taskRecord.ID,
			runID,
			"goal-control-pending",
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
			json.RawMessage(`{"generation":1,"node_id":"converge","item_index":0}`),
			now,
		)
		reservation.RunKind = taskpkg.RunKindWorker
		reservation.LoopRunID = string(loopRun.ID)
		if _, _, _, err := globalDB.ReserveQueuedRun(ctx, reservation); err != nil {
			t.Fatalf("ReserveQueuedRun(worker) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id
			) VALUES (?, 1, 'converge', 0, 'enqueued', ?)`,
			string(loopRun.ID),
			runID,
		); err != nil {
			t.Fatalf("insert generation output error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "worker-goal-control-pending",
			ClaimedBy: &taskpkg.ActorIdentity{
				Kind: taskpkg.ActorKindDaemon,
				Ref:  "worker",
			},
			LeaseDuration: time.Minute,
			Now:           now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		control := looppkg.ActionControl{
			Disposition:  looppkg.ActionDispositionPaused,
			GoalStatus:   "paused",
			Cause:        "operator_pause",
			CheckpointID: "checkpoint-goal-control-pending",
		}
		controlPayload, err := json.Marshal(control)
		if err != nil {
			t.Fatalf("json.Marshal(control) error = %v", err)
		}
		completed, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result: taskpkg.RunResult{CoordinatorControl: &taskpkg.CoordinatorControlResult{
				Kind:    "loop_action",
				Payload: controlPayload,
			}},
			TokensUsed: 7,
			Now:        now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		var storedControl taskpkg.CoordinatorControlResult
		if err := json.Unmarshal(completed.Result, &storedControl); err != nil {
			t.Fatalf("json.Unmarshal(completed.Result) error = %v", err)
		}
		if got, want := storedControl.Kind, "loop_action"; got != want {
			t.Fatalf("stored control kind = %q, want %q", got, want)
		}
		var outputStatus string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT status FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'converge' AND item_index = 0`,
			string(loopRun.ID),
		).Scan(&outputStatus); err != nil {
			t.Fatalf("query generation output error = %v", err)
		}
		if got, want := outputStatus, "control_pending"; got != want {
			t.Fatalf("generation output status = %q, want %q", got, want)
		}
		loopEvents, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: loopRun.WorkspaceID,
			RunID:       loopRun.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		for _, event := range loopEvents {
			if event.Kind == loopRunEventNodeSucceeded || event.Kind == loopRunEventNodeFailed {
				t.Fatalf("unexpected intermediate node terminal event: %#v", event)
			}
		}
		taskEvents, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		completedEvents := 0
		for _, event := range taskEvents {
			if event.EventType == string(hookspkg.HookTaskRunCompleted) {
				completedEvents++
			}
		}
		if got, want := completedEvents, 1; got != want {
			t.Fatalf("task.run.completed count = %d, want %d", got, want)
		}

		recovered, err := globalDB.ReconcileLoopCoordinatorsOnBoot(
			ctx,
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "boot-goal-control"},
			now.Add(3*time.Second),
		)
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
		}
		if len(recovered) > 1 {
			t.Fatalf("recovered coordinators = %#v, want at most one Loop coordinator", recovered)
		}
		if len(recovered) == 1 && (recovered[0].RunKind != taskpkg.RunKindCoordinator ||
			recovered[0].LoopRunID != string(loopRun.ID)) {
			t.Fatalf("recovered coordinator = %#v, want this Loop Run", recovered[0])
		}
		coordinatorClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      string(loopRun.WorkspaceID),
			RunKind:          taskpkg.RunKindCoordinator,
			ClaimerSessionID: "coordinator-goal-control",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    time.Minute,
			Now:              now.Add(4 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(recovered coordinator) error = %v", err)
		}
		runner, err := looppkg.NewCoordinatorRunner(
			globalDB,
			globalDB,
			globalDB,
			slog.Default(),
		)
		if err != nil {
			t.Fatalf("NewCoordinatorRunner() error = %v", err)
		}
		plan, err := runner.Run(ctx, taskpkg.RunID(coordinatorClaim.Run.ID))
		if err != nil {
			t.Fatalf("Run(recovered coordinator) error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(looppkg.StatusPaused) {
			t.Fatalf("recovered coordinator terminal = %#v, want paused", plan.Terminal)
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
			RunID:      coordinatorClaim.Run.ID,
			ClaimToken: coordinatorClaim.ClaimToken,
			Actor:      coordinatorActorContextForTest(),
			Plan:       plan,
			Now:        now.Add(5 * time.Second),
		}, looppkg.NewStoreFinalizer()); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(recovered) error = %v", err)
		}
		settledRun, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID(settled) error = %v", err)
		}
		if settledRun.Status != looppkg.StatusPaused {
			t.Fatalf("settled Run status = %q, want paused", settledRun.Status)
		}
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT status FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'converge' AND item_index = 0`,
			string(loopRun.ID),
		).Scan(&outputStatus); err != nil {
			t.Fatalf("query settled Goal output error = %v", err)
		}
		if outputStatus != "awaiting_goal" {
			t.Fatalf("settled Goal output = %q, want awaiting_goal", outputStatus)
		}
		replayed, err := globalDB.ReconcileLoopCoordinatorsOnBoot(
			ctx,
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "boot-goal-control"},
			now.Add(6*time.Second),
		)
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot(replay) error = %v", err)
		}
		if len(replayed) != 0 {
			t.Fatalf("replayed coordinators = %#v, want none after settlement", replayed)
		}
	})
}

func TestGlobalDBHeartbeatRunLeaseShouldPersistCoalescedLoopTokenTicks(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 17, 30, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-heartbeat-token-ticks", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	taskRecord := taskRecordForTest("task-heartbeat-token-ticks")
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	runID := "run-heartbeat-token-ticks"
	reservation := queuedRunReservationForTest(
		taskRecord.ID,
		runID,
		"heartbeat-token-ticks",
		taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
		json.RawMessage(`{"generation":1,"node_id":"agent","item_index":0}`),
		now,
	)
	reservation.RunKind = taskpkg.RunKindWorker
	reservation.LoopRunID = string(loopRun.ID)
	if _, _, _, err := globalDB.ReserveQueuedRun(ctx, reservation); err != nil {
		t.Fatalf("ReserveQueuedRun(worker) error = %v", err)
	}
	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "worker-heartbeat-token-ticks",
		ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  "worker",
		},
		LeaseDuration: time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	heartbeats := []struct {
		at     time.Time
		tokens int64
	}{
		{at: now.Add(6 * time.Second), tokens: 1200},
		{at: now.Add(7 * time.Second), tokens: 3200},
		{at: now.Add(12 * time.Second), tokens: 4300},
	}
	for _, heartbeat := range heartbeats {
		if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			RunID:         claim.Run.ID,
			ClaimToken:    claim.ClaimToken,
			LeaseDuration: time.Minute,
			Now:           heartbeat.at,
			TokensUsed:    heartbeat.tokens,
		}); err != nil {
			t.Fatalf("HeartbeatRunLease(tokens=%d) error = %v", heartbeat.tokens, err)
		}
	}
	var storedTaskTokens int64
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT tokens_used FROM task_runs WHERE id = ?`,
		runID,
	).Scan(&storedTaskTokens); err != nil {
		t.Fatalf("query task run tokens_used error = %v", err)
	}
	if storedTaskTokens != 4300 {
		t.Fatalf("task run tokens_used = %d, want 4300", storedTaskTokens)
	}
	storedLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID() error = %v", err)
	}
	if storedLoop.TokensUsed != 4300 {
		t.Fatalf("loop tokens_used = %d, want 4300", storedLoop.TokensUsed)
	}
	events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: loopRun.WorkspaceID,
		RunID:       loopRun.ID,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	ticks := make([]map[string]any, 0, 2)
	for _, event := range events {
		if event.Kind != loopRunEventTokenTick {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal(token_tick) error = %v", err)
		}
		ticks = append(ticks, payload)
	}
	if len(ticks) != 2 {
		t.Fatalf("token_tick count = %d, want 2; events=%#v", len(ticks), events)
	}
	if got := int64(ticks[0]["tokens_used"].(float64)); got != 1200 {
		t.Fatalf("first token_tick.tokens_used = %d, want 1200", got)
	}
	if got := int64(ticks[1]["tokens_used"].(float64)); got != 4300 {
		t.Fatalf("second token_tick.tokens_used = %d, want 4300", got)
	}
	if terminal, ok := ticks[0]["terminal"].(bool); !ok || terminal {
		t.Fatalf("first token_tick.terminal = %#v, want false", ticks[0]["terminal"])
	}
}

func TestLoopGateVerdictEventPayloadShouldSurfaceCriterionConfidence(t *testing.T) {
	t.Parallel()

	t.Run("Should keep confidence on each criterion and omit ambiguous summary", func(t *testing.T) {
		t.Parallel()

		firstConfidence := 0.91
		secondConfidence := 0.42
		raw := mustJSON(t, gate.Verdict{
			Outcome: gate.VerdictOutcomeRejected,
			Criteria: []gate.CriterionResult{
				{
					ID:         "all_handled",
					Type:       dsl.CriterionAgentJudge,
					Outcome:    gate.VerdictOutcomeRejected,
					Confidence: &firstConfidence,
				},
				{
					ID:         "docs_current",
					Type:       dsl.CriterionAgentJudge,
					Outcome:    gate.VerdictOutcomeApproved,
					Passed:     true,
					Confidence: &secondConfidence,
				},
			},
			Route: gate.RouteDecision{
				Action:     gate.RouteRevise,
				ReasonCode: "blocking_issues",
			},
		})

		payload := loopGateVerdictEventPayload(testutil.Context(t), 2, taskpkg.CoordinatorTerminal{
			GateID:     "definition_of_done",
			Status:     string(looppkg.StatusRunning),
			ReasonCode: "blocking_issues",
			Details:    raw,
		})

		if _, ok := payload[loopRunEventPayloadKeyConfidence]; ok {
			t.Fatalf("payload confidence = %#v, want omitted for multiple confidence-bearing criteria", payload)
		}
		criteria, ok := payload["criteria"].([]map[string]any)
		if !ok {
			t.Fatalf("payload criteria = %#v, want []map[string]any", payload["criteria"])
		}
		if got, want := len(criteria), 2; got != want {
			t.Fatalf("criteria count = %d, want %d", got, want)
		}
		if got := criteria[0][loopRunEventPayloadKeyConfidence]; got != firstConfidence {
			t.Fatalf("criteria[0].confidence = %#v, want %.2f", got, firstConfidence)
		}
		if got := criteria[1][loopRunEventPayloadKeyConfidence]; got != secondConfidence {
			t.Fatalf("criteria[1].confidence = %#v, want %.2f", got, secondConfidence)
		}
	})

	t.Run("Should keep summary confidence when only one criterion has confidence", func(t *testing.T) {
		t.Parallel()

		confidence := 0.88
		raw := mustJSON(t, gate.Verdict{
			Outcome: gate.VerdictOutcomeApproved,
			Criteria: []gate.CriterionResult{
				{ID: "compile", Type: dsl.CriterionCommand, Outcome: gate.VerdictOutcomeApproved, Passed: true},
				{
					ID:         "review",
					Type:       dsl.CriterionAgentJudge,
					Outcome:    gate.VerdictOutcomeApproved,
					Passed:     true,
					Confidence: &confidence,
				},
			},
			Route: gate.RouteDecision{Action: gate.RouteContinue},
		})

		payload := loopGateVerdictEventPayload(testutil.Context(t), 3, taskpkg.CoordinatorTerminal{
			GateID:  "definition_of_done",
			Status:  string(looppkg.StatusRunning),
			Details: raw,
		})

		if got := payload[loopRunEventPayloadKeyConfidence]; got != confidence {
			t.Fatalf("payload confidence = %#v, want %.2f", got, confidence)
		}
	})

	t.Run("Should degrade malformed verdict details to revise payload", func(t *testing.T) {
		t.Parallel()

		payload := loopGateVerdictEventPayload(testutil.Context(t), 4, taskpkg.CoordinatorTerminal{
			GateID:     "definition_of_done",
			Status:     string(looppkg.StatusRunning),
			ReasonCode: "malformed_verdict",
			Details:    json.RawMessage(`{"outcome":`),
		})

		if got, want := payload["verdict"], loopRunEventVerdictRevise; got != want {
			t.Fatalf("payload verdict = %#v, want %q", got, want)
		}
		if got, want := payload[loopRunEventPayloadKeyReason], "malformed_verdict"; got != want {
			t.Fatalf("payload reason = %#v, want %q", got, want)
		}
	})
}

func TestLoopChannelMessageTextShouldSuppressMalformedPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return empty text for malformed JSON", func(t *testing.T) {
		t.Parallel()

		text := channelMessageText(testutil.Context(t), "run-invalid-payload", json.RawMessage(`{"message":`))
		if text != "" {
			t.Fatalf("channelMessageText() = %q, want empty text for malformed payload", text)
		}
	})
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func TestGlobalDBCompleteRunLeaseShouldStoreLargeLoopOutputByRef(t *testing.T) {
	t.Run("Should externalize large loop node result and carry ref on generation output", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 17, 10, 0, 0, time.UTC)
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-large-output-ref", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		taskRecord := taskRecordForTest("task-large-output-ref")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		runID := "run-large-output-ref"
		metadata := json.RawMessage(`{"generation":1,"node_id":"summarize","item_index":0}`)
		reservation := queuedRunReservationForTest(
			taskRecord.ID,
			runID,
			"large-output-ref",
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
			metadata,
			now,
		)
		reservation.RunKind = taskpkg.RunKindWorker
		reservation.LoopRunID = string(loopRun.ID)
		if _, _, _, err := globalDB.ReserveQueuedRun(ctx, reservation); err != nil {
			t.Fatalf("ReserveQueuedRun(worker) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id
			) VALUES (?, 1, 'summarize', 0, 'enqueued', ?)`,
			string(loopRun.ID),
			runID,
		); err != nil {
			t.Fatalf("insert generation output error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "worker-large-output-ref",
			ClaimedBy: &taskpkg.ActorIdentity{
				Kind: taskpkg.ActorKindDaemon,
				Ref:  "worker",
			},
			LeaseDuration: time.Minute,
			Now:           now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		resultPayload, err := json.Marshal(map[string]string{
			"body": strings.Repeat("x", looppkg.LoopOutputInlineLimitBytes+1),
		})
		if err != nil {
			t.Fatalf("marshal result payload error = %v", err)
		}
		wantRef := looppkg.OutputRefForPayload(resultPayload)

		updated, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result:     taskpkg.RunResult{Value: resultPayload},
			TokensUsed: 7,
			Now:        now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if len(updated.Result) != 0 {
			t.Fatalf("updated Result = %s, want empty inline result", updated.Result)
		}

		var resultJSON sql.NullString
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT result_json FROM task_runs WHERE id = ?`,
			claim.Run.ID,
		).Scan(&resultJSON); err != nil {
			t.Fatalf("query task run result_json error = %v", err)
		}
		if resultJSON.Valid {
			t.Fatalf("task_runs.result_json = %q, want NULL", resultJSON.String)
		}
		var status string
		var outputRef sql.NullString
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT status, output_ref
			 FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'summarize' AND item_index = 0`,
			string(loopRun.ID),
		).Scan(&status, &outputRef); err != nil {
			t.Fatalf("query generation output error = %v", err)
		}
		if got, want := status, "succeeded"; got != want {
			t.Fatalf("output status = %q, want %q", got, want)
		}
		if !outputRef.Valid || outputRef.String != wantRef {
			t.Fatalf("output_ref = %#v, want %q", outputRef, wantRef)
		}
		loaded, err := getLoopOutputByRefWithExecutor(ctx, globalDB.db, wantRef)
		if err != nil {
			t.Fatalf("getLoopOutputByRefWithExecutor() error = %v", err)
		}
		if string(loaded) != string(resultPayload) {
			t.Fatalf("loaded payload mismatch: got %d bytes want %d bytes", len(loaded), len(resultPayload))
		}
	})
}

func TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldSweepOrphanedLoopOutputBlobsAtTerminalBoundary(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should sweep orphaned loop output blobs at terminal boundary", func(t *testing.T) {
		t.Parallel()
		testGlobalDBCompleteCoordinatorAndEnqueueNextShouldSweepOrphanedLoopOutputBlobsAtTerminalBoundary(t)
	})
}

func testGlobalDBCompleteCoordinatorAndEnqueueNextShouldSweepOrphanedLoopOutputBlobsAtTerminalBoundary(t *testing.T) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 4, 17, 20, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-output-retention", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	keepPayload := json.RawMessage(`{"body":"keep"}`)
	keepRef := looppkg.OutputRefForPayload(keepPayload)
	reportPayload := json.RawMessage(`"goal report evidence"`)
	reportRef := looppkg.OutputRefForPayload(reportPayload)
	orphanPayload := json.RawMessage(`{"body":"orphan"}`)
	orphanRef := looppkg.OutputRefForPayload(orphanPayload)
	err = globalDB.withTaskImmediateTransaction(ctx, "seed loop output retention", func(exec taskSQLExecutor) error {
		if err := upsertLoopOutputBlobWithExecutor(ctx, exec, keepRef, keepPayload, now); err != nil {
			return err
		}
		if err := upsertLoopOutputBlobWithExecutor(ctx, exec, orphanRef, orphanPayload, now); err != nil {
			return err
		}
		if err := upsertLoopOutputBlobWithExecutor(ctx, exec, reportRef, reportPayload, now); err != nil {
			return err
		}
		_, err := exec.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, output_ref
			) VALUES (?, 1, 'load', 0, 'succeeded', ?)`,
			string(loopRun.ID),
			keepRef,
		)
		if err != nil {
			return err
		}
		_, err = exec.ExecContext(
			ctx,
			`INSERT INTO loop_goal_checkpoints (
				loop_run_id, generation, node_id, item_index, control_epoch, phase, goal_status,
				turn_limit, context_nudge_ratio, report_prompt_id, report_status,
				report_evidence_ref, report_binding_epoch, report_actor_kind, report_actor_id,
				report_recorded_at, updated_at
			) VALUES (?, 1, 'goal', 0, 1, 'prompting', 'active', 3, 0.8,
				'prompt-report', 'blocked', ?, 1, 'agent_session', 'session-report', ?, ?)`,
			string(loopRun.ID),
			reportRef,
			store.FormatTimestamp(now),
			store.FormatTimestamp(now),
		)
		return err
	})
	if err != nil {
		t.Fatalf("seed loop output retention error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		loopRun.ID,
		"run-output-retention",
		now,
	)

	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan: taskpkg.CoordinatorCompletionPlan{
			Snapshot: taskpkg.GenerationSnapshot{LoopRunID: string(loopRun.ID), Generation: 1},
			Terminal: &taskpkg.CoordinatorTerminal{
				Status: string(looppkg.StatusDone),
				Cause:  string(looppkg.TransitionCauseContract),
			},
		},
		Now: now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusDone); got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	loaded, err := getLoopOutputByRefWithExecutor(ctx, globalDB.db, keepRef)
	if err != nil {
		t.Fatalf("getLoopOutputByRefWithExecutor(keep) error = %v", err)
	}
	if string(loaded) != string(keepPayload) {
		t.Fatalf("kept payload = %s, want %s", loaded, keepPayload)
	}
	loadedReport, err := getLoopOutputByRefWithExecutor(ctx, globalDB.db, reportRef)
	if err != nil {
		t.Fatalf("getLoopOutputByRefWithExecutor(report) error = %v", err)
	}
	if string(loadedReport) != string(reportPayload) {
		t.Fatalf("kept report payload = %s, want %s", loadedReport, reportPayload)
	}
	if _, err := getLoopOutputByRefWithExecutor(ctx, globalDB.db, orphanRef); !errors.Is(
		err,
		looppkg.ErrOutputRefNotFound,
	) {
		t.Fatalf("getLoopOutputByRefWithExecutor(orphan) error = %v, want %v", err, looppkg.ErrOutputRefNotFound)
	}
}

var errForcedFinalizer = errors.New("forced finalizer failure")

type failingLoopFinalizer struct{}

func (failingLoopFinalizer) WriteGenerationSnapshot(
	context.Context,
	taskpkg.Tx,
	taskpkg.GenerationSnapshot,
) error {
	return errForcedFinalizer
}

func coordinatorActorContextForTest() taskpkg.ActorContext {
	return taskpkg.ActorContext{
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.loop"},
		Authority: taskpkg.Authority{Read: true, Write: true, CreateGlobal: true},
	}
}

func operatorActorContextForTest(ref string) taskpkg.ActorContext {
	return taskpkg.ActorContext{
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: ref},
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "test"},
		Scope:     taskpkg.CallerScope{Operator: true},
		Authority: taskpkg.Authority{Read: true, Write: true},
	}
}

func claimCoordinatorRunForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	loopRunID looppkg.RunID,
	runID string,
	now time.Time,
) taskpkg.ClaimResult {
	t.Helper()

	loopRun, err := globalDB.GetLoopRunByID(ctx, loopRunID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(%s) error = %v", loopRunID, err)
	}
	seededRunID := loopCoordinatorRunID(loopRun.ID, loopRun.Generation+1)
	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            seededRunID,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      string(loopRun.WorkspaceID),
		RunKind:          taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-" + runID,
		ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(%s) error = %v", runID, err)
	}
	return claim
}

func createCompletedLoopWorkerRunForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	loopRunID looppkg.RunID,
	suffix string,
	tokensUsed int64,
	now time.Time,
) {
	t.Helper()

	taskRecord := taskRecordForTest("task-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusCompleted
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask(worker %s) error = %v", suffix, err)
	}
	run := taskRunForTest("run-"+suffix, taskRecord.ID)
	run.Status = taskpkg.TaskRunStatusCompleted
	run.RunKind = taskpkg.RunKindWorker
	run.LoopRunID = string(loopRunID)
	run.TokensUsed = tokensUsed
	run.EndedAt = now
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(worker %s) error = %v", suffix, err)
	}
}

func writeInvalidSoulFixture(t *testing.T, workspaceRoot string) {
	t.Helper()

	soulDir := filepath.Join(workspaceRoot, ".agh", "agents", "coder")
	if err := os.MkdirAll(soulDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", soulDir, err)
	}
	soulPath := filepath.Join(soulDir, "SOUL.md")
	content := []byte(
		"---\nprovider: claude\n---\nThis invalid file must not be read during claim.\n",
	)
	if err := os.WriteFile(soulPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", soulPath, err)
	}
}

func assertRunSoulMetadata(
	t *testing.T,
	raw json.RawMessage,
	workflowID string,
	snapshotID string,
	digest string,
	agentName string,
	capturedAt time.Time,
) {
	t.Helper()

	var decoded struct {
		WorkflowID string `json:"workflow_id"`
		Soul       struct {
			SnapshotID string    `json:"snapshot_id"`
			Digest     string    `json:"digest"`
			AgentName  string    `json:"agent_name"`
			CapturedAt time.Time `json:"captured_at"`
		} `json:"soul"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(run.Metadata) error = %v; raw=%s", err, raw)
	}
	if decoded.WorkflowID != workflowID {
		t.Fatalf("metadata.workflow_id = %q, want %q", decoded.WorkflowID, workflowID)
	}
	if decoded.Soul.SnapshotID != snapshotID ||
		decoded.Soul.Digest != digest ||
		decoded.Soul.AgentName != agentName ||
		!decoded.Soul.CapturedAt.Equal(capturedAt) {
		t.Fatalf(
			"metadata.soul = %#v, want snapshot=%q digest=%q agent=%q captured_at=%s",
			decoded.Soul,
			snapshotID,
			digest,
			agentName,
			capturedAt,
		)
	}
}

func coordinatorResultStatus(
	t *testing.T,
	result *taskpkg.CoordinatorCompletionResult,
) string {
	t.Helper()

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result.Context, &payload); err != nil {
		t.Fatalf("json.Unmarshal(CoordinatorCompletionResult.Context) error = %v", err)
	}
	return payload.Status
}

func assertLoopEventKinds(t *testing.T, events []looppkg.RunEvent, want []string) {
	t.Helper()

	got := make([]string, 0, len(events))
	for _, event := range events {
		got = append(got, event.Kind)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("loop event kinds = %#v, want %#v", got, want)
	}
}

func loopEventPayloadForKind(
	t *testing.T,
	events []looppkg.RunEvent,
	kind string,
) map[string]any {
	t.Helper()

	for _, event := range events {
		if event.Kind != kind {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal(%s payload) error = %v; payload=%s", kind, err, event.Payload)
		}
		return payload
	}
	t.Fatalf("loop event kind %q not found in %#v", kind, events)
	return nil
}
