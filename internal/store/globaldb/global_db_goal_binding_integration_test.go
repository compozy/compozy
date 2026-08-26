//go:build integration

package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGoalSessionCreationIdentityIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should create once match exact identity and reject changed or absent identity", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-identity", "ws-goal-foreign")
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		identity := store.SessionCreationIdentity{
			CreationProfileRef: "profile-v1",
			PolicySpecDigest:   "policy-v1",
			CreationDigest:     "creation-session-1",
		}
		info := goalSessionInfoForTest("session-1", "ws-goal-identity", now)

		created, err := globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), info, identity)
		if err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity(create) error = %v", err)
		}
		if !created.Created || created.SessionID != info.ID || created.Identity != identity {
			t.Fatalf("created registration = %#v, want created exact identity", created)
		}
		matched, err := globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), info, identity)
		if err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity(match) error = %v", err)
		}
		if matched.Created || matched.Identity != identity {
			t.Fatalf("matched registration = %#v, want existing exact identity", matched)
		}

		changed := identity
		changed.CreationDigest = "creation-session-1-changed"
		_, err = globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), info, changed)
		requireSessionCreationIdentityMismatch(t, err)

		foreign := info
		foreign.WorkspaceID = "ws-goal-foreign"
		_, err = globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), foreign, identity)
		requireSessionCreationIdentityMismatch(t, err)

		legacy := goalSessionInfoForTest("session-null-identity", "ws-goal-identity", now)
		if err := globalDB.RegisterSession(testutil.Context(t), legacy); err != nil {
			t.Fatalf("RegisterSession(legacy) error = %v", err)
		}
		_, err = globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), legacy, identity)
		requireSessionCreationIdentityMismatch(t, err)

		provisional := goalSessionInfoForTest("session-starting-identity", "ws-goal-identity", now)
		provisional.State = globalDBSessionStateStarting
		if err := globalDB.RegisterSession(testutil.Context(t), provisional); err != nil {
			t.Fatalf("RegisterSession(provisional) error = %v", err)
		}
		bound, err := globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), provisional, identity)
		if err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity(provisional) error = %v", err)
		}
		if bound.Created || bound.Identity != identity {
			t.Fatalf("provisional registration = %#v, want bound existing starting session", bound)
		}

		loaded, err := globalDB.GetSessionCreationIdentity(testutil.Context(t), info.ID)
		if err != nil {
			t.Fatalf("GetSessionCreationIdentity() error = %v", err)
		}
		if loaded != identity {
			t.Fatalf("loaded identity = %#v, want %#v", loaded, identity)
		}
	})

	t.Run("Should bind a terminal recovery snapshot over an identityless starting projection", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-recovery")
		now := time.Date(2026, 8, 12, 12, 28, 45, 0, time.UTC)
		identity := store.SessionCreationIdentity{
			CreationProfileRef: "profile-recovered",
			PolicySpecDigest:   "policy-recovered",
			CreationDigest:     "creation-recovered",
		}
		provisional := goalSessionInfoForTest("session-recovered", "ws-goal-recovery", now)
		provisional.State = globalDBSessionStateStarting
		if err := globalDB.RegisterSession(testutil.Context(t), provisional); err != nil {
			t.Fatalf("RegisterSession(provisional) error = %v", err)
		}

		recovered := provisional
		recovered.State = globalDBSessionStateStopped
		recovered.UpdatedAt = now.Add(time.Second)
		registration, err := globalDB.RegisterSessionWithCreationIdentity(
			testutil.Context(t),
			recovered,
			identity,
		)
		if err != nil {
			t.Fatalf("RegisterSessionWithCreationIdentity(recovered) error = %v", err)
		}
		if registration.Created || registration.Identity != identity {
			t.Fatalf("recovered registration = %#v, want existing exact identity", registration)
		}

		indexed, err := globalDB.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			ID:        recovered.ID,
		})
		if err != nil {
			t.Fatalf("ListSessions(recovered) error = %v", err)
		}
		if len(indexed) != 1 || indexed[0].State != recovered.State {
			t.Fatalf("ListSessions(recovered) = %#v, want one stopped session", indexed)
		}
		loaded, err := globalDB.GetSessionCreationIdentity(testutil.Context(t), recovered.ID)
		if err != nil {
			t.Fatalf("GetSessionCreationIdentity(recovered) error = %v", err)
		}
		if loaded != identity {
			t.Fatalf("recovered identity = %#v, want %#v", loaded, identity)
		}

		settled := goalSessionInfoForTest("session-settled", "ws-goal-recovery", now)
		settled.State = globalDBSessionStateStopped
		if err := globalDB.RegisterSession(testutil.Context(t), settled); err != nil {
			t.Fatalf("RegisterSession(settled) error = %v", err)
		}
		_, err = globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), settled, identity)
		requireSessionCreationIdentityMismatch(t, err)
	})
}

func TestGoalSessionBindingLifecycleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should atomically allocate the next epoch for a later Goal generation", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-goal-binding-allocate"
			loopRunID   = "run-goal-binding-allocate"
			handle      = "goal:binding-allocate"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 26, 14, 0, 0, 0, time.UTC)
		insertGoalSchemaLoopRun(t, globalDB, loopRunID, workspaceID, "catalog", nil)
		seedActiveGoalBindingForTest(
			t, globalDB, loopRunID, workspaceID, handle, 1, "session-binding-allocate-1", now,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = ? AND handle = ? AND binding_epoch = 1`,
			store.FormatTimestamp(now.Add(time.Second)),
			loopRunID,
			handle,
		); err != nil {
			t.Fatalf("close prior Goal binding error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET generation = 2 WHERE id = ? AND workspace_id = ?`,
			loopRunID,
			workspaceID,
		); err != nil {
			t.Fatalf("advance Goal Run generation error = %v", err)
		}
		checkpointKey := goal.TurnKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Generation:  2,
			NodeID:      "goal-binding-allocate",
		}
		if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
			Key: checkpointKey, ControlEpoch: 1, Phase: "idle", Status: "active", TurnLimit: 3,
			TaskRunID: "task-goal-binding-allocate", ContextState: "unknown", ContextNudgeRatio: 0.8,
			UpdatedAt: now.Add(2 * time.Second),
		}}); err != nil {
			t.Fatalf("CreateCheckpoint() error = %v", err)
		}
		profile := store.SessionCreationProfile{
			Version: store.SessionCreationProfileVersion, AgentName: "codex", Provider: "native",
			ProfileID: store.DefaultProfileID, WorkspaceID: workspaceID, CWD: "/tmp",
			SandboxMode: store.SessionCreationSandboxNone,
		}
		request := goal.AllocateBindingAttemptRequest{
			Key:           goal.BindingKey{WorkspaceID: workspaceID, LoopRunID: loopRunID, Handle: handle},
			CheckpointKey: checkpointKey, ExpectedControlEpoch: 1, ExpectedCheckpointPhase: "idle",
			ExpectedTaskRunID: "task-goal-binding-allocate", IdentityHandle: handle,
			CreationProfile: profile,
			CreationOptions: store.SessionCreationOptions{
				NetworkOwnerKey:      "loop_run:" + loopRunID,
				NetworkParticipation: participation.LocalSpec(),
				SessionType:          "loop-goal",
			},
			CreatedAt: now.Add(3 * time.Second),
		}

		const contenders = 8
		results := make(chan goal.SessionBinding, contenders)
		errorsCh := make(chan error, contenders)
		var wait sync.WaitGroup
		wait.Add(contenders)
		for range contenders {
			go func() {
				defer wait.Done()
				binding, err := globalDB.AllocateSessionBindingAttempt(testutil.Context(t), request)
				if err != nil {
					errorsCh <- err
					return
				}
				results <- binding
			}()
		}
		wait.Wait()
		close(results)
		close(errorsCh)
		for err := range errorsCh {
			t.Errorf("AllocateSessionBindingAttempt() error = %v", err)
		}
		wantAttemptID, wantSessionID := goal.DeriveBindingIdentity(checkpointKey, handle, 2)
		for binding := range results {
			if binding.BindingEpoch != 2 || binding.State != goal.BindingStateCreating ||
				binding.BindingAttemptID != wantAttemptID || binding.SessionID != wantSessionID {
				t.Errorf("allocated binding = %#v, want deterministic creating epoch 2", binding)
			}
		}
		var bindingCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_session_bindings WHERE loop_run_id = ? AND handle = ?`,
			loopRunID,
			handle,
		).Scan(&bindingCount); err != nil {
			t.Fatalf("count Goal bindings error = %v", err)
		}
		if bindingCount != 2 {
			t.Fatalf("binding count = %d, want closed epoch 1 and creating epoch 2", bindingCount)
		}
	})

	t.Run("Should replay cleanup metadata while rejecting a changed immutable identity", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-goal-cleanup-replay"
			loopRunID   = "run-goal-cleanup-replay"
			handle      = "goal:cleanup-replay"
			sessionID   = "session-goal-cleanup-replay"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
		insertGoalSchemaLoopRun(t, globalDB, loopRunID, workspaceID, "catalog", nil)
		seedActiveGoalBindingForTest(t, globalDB, loopRunID, workspaceID, handle, 1, sessionID, now)

		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}, 1)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt() error = %v", err)
		}
		enqueue := func(cause goal.SessionCleanupCause, createdAt time.Time, target goal.SessionBinding) error {
			return globalDB.withTaskImmediateTransaction(
				ctx,
				"enqueue replayed Goal cleanup",
				func(exec taskSQLExecutor) error {
					return enqueueGoalSessionCleanupWithExecutor(ctx, exec, target, cause, createdAt)
				},
			)
		}
		if err := enqueue(goal.SessionCleanupCauseStop, now.Add(time.Minute), binding); err != nil {
			t.Fatalf("enqueue(stop) error = %v", err)
		}
		if err := enqueue(goal.SessionCleanupCauseTerminal, now.Add(2*time.Minute), binding); err != nil {
			t.Fatalf("enqueue(terminal replay) error = %v", err)
		}

		pending, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(pending) != 1 || pending[0].Cause != goal.SessionCleanupCauseStop ||
			!pending[0].CreatedAt.Equal(now.Add(time.Minute)) {
			t.Fatalf("cleanup replay = %#v, want first-writer metadata", pending)
		}

		changed := binding
		changed.SessionID = "session-goal-cleanup-conflict"
		if err := enqueue(
			goal.SessionCleanupCauseTerminal,
			now.Add(3*time.Minute),
			changed,
		); !errors.Is(
			err,
			looppkg.ErrTransitionConflict,
		) {
			t.Fatalf("enqueue(changed identity) error = %v, want transition conflict", err)
		}
	})

	t.Run("Should close a completed run-agent binding and publish durable cleanup", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-run-agent-completion"
			loopRunID   = "run-agent-completion"
			taskID      = "task-run-agent-completion"
			taskRunID   = "taskrun-run-agent-completion"
			handle      = "action:run-agent-completion"
			sessionID   = "session-run-agent-completion"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 19, 52, 11, 0, time.UTC)
		insertGoalSchemaLoopRun(t, globalDB, loopRunID, workspaceID, "catalog", nil)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at)
			 VALUES (?, 1, 0, 'initial', ?)`,
			loopRunID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert run-agent generation error = %v", err)
		}
		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID:  workspaceID,
			LoopRunID:    loopRunID,
			TaskID:       taskID,
			TaskRunID:    taskRunID,
			Handle:       handle,
			Generation:   1,
			Attempt:      1,
			Epoch:        4,
			InsertOutput: true,
		})
		seedActiveGoalBindingForTest(t, globalDB, loopRunID, workspaceID, handle, 5, sessionID, now)

		leaseOwner := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-action-runtime"}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: taskRunID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			RunKind: taskpkg.RunKindWorker, ClaimedBy: &leaseOwner,
			LeaseDuration: time.Minute, Now: now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		actor := taskpkg.ActorContext{
			Actor:     leaseOwner,
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.loop-action"},
			Authority: taskpkg.Authority{Read: true, Write: true},
		}
		if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor: actor, RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Result: taskpkg.RunResult{Value: json.RawMessage(`{"summary":"done"}`)},
			Now:    now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}

		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt() error = %v", err)
		}
		if binding.State != goal.BindingStateClosed {
			t.Fatalf("binding state = %q, want %q", binding.State, goal.BindingStateClosed)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(cleanups) != 1 || cleanups[0].SessionID != sessionID ||
			cleanups[0].Cause != goal.SessionCleanupCauseTerminal {
			t.Fatalf("run-agent cleanups = %#v, want one terminal session cleanup", cleanups)
		}
	})

	t.Run("Should settle a run-agent without closing an origin-borrowed binding", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-run-agent-borrowed"
			loopRunID   = "run-agent-borrowed"
			taskID      = "task-run-agent-borrowed"
			taskRunID   = "taskrun-run-agent-borrowed"
			handle      = "action:run-agent-borrowed"
			sessionID   = "session-run-agent-borrowed"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 21, 0, 15, 0, 0, time.UTC)
		insertGoalSchemaLoopRun(t, globalDB, loopRunID, workspaceID, "catalog", nil)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at)
			 VALUES (?, 1, 0, 'initial', ?)`,
			loopRunID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert run-agent generation error = %v", err)
		}
		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID:  workspaceID,
			LoopRunID:    loopRunID,
			TaskID:       taskID,
			TaskRunID:    taskRunID,
			Handle:       handle,
			Generation:   1,
			Attempt:      1,
			Epoch:        0,
			InsertOutput: true,
		})
		seedActiveGoalBindingWithOwnershipForTest(
			t,
			globalDB,
			loopRunID,
			workspaceID,
			handle,
			1,
			sessionID,
			goal.BindingOwnershipOriginBorrowed,
			now,
		)

		leaseOwner := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-action-runtime"}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: taskRunID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			RunKind: taskpkg.RunKindWorker, ClaimedBy: &leaseOwner,
			LeaseDuration: time.Minute, Now: now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		actor := taskpkg.ActorContext{
			Actor:     leaseOwner,
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.loop-action"},
			Authority: taskpkg.Authority{Read: true, Write: true},
		}
		if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor: actor, RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Result: taskpkg.RunResult{Value: json.RawMessage(`{"summary":"done"}`)},
			Now:    now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}

		settled, err := globalDB.GetTaskRun(ctx, taskRunID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if settled.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("task run status = %q, want %q", settled.Status, taskpkg.TaskRunStatusCompleted)
		}
		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}, 1)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt() error = %v", err)
		}
		if binding.State != goal.BindingStateActive {
			t.Fatalf("borrowed binding state = %q, want %q", binding.State, goal.BindingStateActive)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(cleanups) != 0 {
			t.Fatalf("borrowed binding cleanups = %#v, want none", cleanups)
		}
	})

	t.Run("Should keep a retried run-agent binding and close it after final failure", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-run-agent-failure"
			loopRunID   = "run-agent-failure"
			taskID      = "task-run-agent-failure"
			taskRunID   = "taskrun-run-agent-failure"
			handle      = "action:run-agent-failure"
			sessionID   = "session-run-agent-failure"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 20, 35, 0, 0, time.UTC)
		loopRun := testLoopRun(loopRunID, now, looppkg.StatusRunning)
		loopRun.WorkspaceID = workspaceID
		created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}

		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID:  workspaceID,
			LoopRunID:    loopRunID,
			TaskID:       taskID,
			TaskRunID:    taskRunID,
			Handle:       handle,
			Generation:   1,
			Attempt:      1,
			Epoch:        4,
			InsertOutput: true,
		})
		bindingKey := goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}
		seedActiveGoalBindingForTest(t, globalDB, loopRunID, workspaceID, handle, 5, sessionID, now)

		leaseOwner := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-action-runtime"}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: taskRunID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			RunKind: taskpkg.RunKindWorker, ClaimedBy: &leaseOwner,
			LeaseDuration: time.Minute, Now: now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		actor := taskpkg.ActorContext{
			Actor:     leaseOwner,
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.loop-action"},
			Authority: taskpkg.Authority{Read: true, Write: true},
		}
		failedAt := now.Add(2 * time.Second)
		if _, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
			Actor: actor, RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Failure: taskpkg.RunFailure{Error: "provider transport failed"},
			Now:     failedAt,
		}); err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}

		binding, err := globalDB.GetSessionBindingAttempt(ctx, bindingKey, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(after failure) error = %v", err)
		}
		if binding.State != goal.BindingStateActive {
			t.Fatalf("binding state after unsettled failure = %q, want %q", binding.State, goal.BindingStateActive)
		}

		nextAttemptAt := now.Add(time.Minute)
		expectedEpoch := int64(4)
		failureClass := looppkg.FailureTransport
		coordinatorClaim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"run-agent-retry",
			now.Add(3*time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: coordinatorClaim.Run.ID, ClaimToken: coordinatorClaim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(4 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Yield: true,
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID: loopRunID, Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{
							Outputs: []looppkg.GenerationOutput{{
								Generation: 1, NodeID: "execute", Status: "retrying", TaskRunID: taskRunID,
								Attempt: 1, NextAttemptAt: &nextAttemptAt, Epoch: 5, ExpectedEpoch: &expectedEpoch,
							}},
							Attempts: []looppkg.NodeAttempt{{
								LoopRunID: created.ID, Generation: 1, NodeID: "execute", Attempt: 1,
								FailureClass: &failureClass, FailureCode: "provider_transport",
								Cause: "provider transport failed", Disposition: looppkg.AttemptRetried,
								StartedAt: now.Add(time.Second), EndedAt: &failedAt, NextAttemptAt: &nextAttemptAt,
							}},
						},
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(retry) error = %v", err)
		}
		binding, err = globalDB.GetSessionBindingAttempt(ctx, bindingKey, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(after retry) error = %v", err)
		}
		if binding.State != goal.BindingStateActive {
			t.Fatalf("binding state after retry = %q, want %q", binding.State, goal.BindingStateActive)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup(after retry) error = %v", err)
		}
		if len(cleanups) != 0 {
			t.Fatalf("run-agent retry cleanups = %#v, want none", cleanups)
		}

		finalSnapshot := taskpkg.GenerationSnapshot{
			LoopRunID:  loopRunID,
			Generation: 1,
			Payload: looppkg.GenerationSnapshotPayload{
				Outputs: []looppkg.GenerationOutput{{
					Generation: 1, NodeID: "execute", Status: "failed", TaskRunID: taskRunID,
					Attempt: 1, Epoch: 5,
				}},
				Attempts: []looppkg.NodeAttempt{{
					LoopRunID: created.ID, Generation: 1, NodeID: "execute", Attempt: 1,
					FailureClass: &failureClass, FailureCode: "provider_transport",
					Cause: "provider transport failed", Disposition: looppkg.AttemptEscalated,
					StartedAt: now.Add(time.Second), EndedAt: &failedAt,
				}},
			},
		}
		wake, added, err := globalDB.EnqueueLoopCoordinatorWake(
			ctx,
			string(created.ID),
			"run-agent-final-failure",
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
			now.Add(5*time.Second),
		)
		if err != nil || !added {
			t.Fatalf("EnqueueLoopCoordinatorWake(final failure) added = %t, error = %v", added, err)
		}
		finalCoordinatorClaim := claimExactLoopTaskRunForTest(
			ctx,
			t,
			globalDB,
			created,
			wake.ID,
			taskpkg.RunKindCoordinator,
			now.Add(5*time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID:      finalCoordinatorClaim.Run.ID,
				ClaimToken: finalCoordinatorClaim.ClaimToken,
				Actor:      coordinatorActorContextForTest(),
				Now:        now.Add(6 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Snapshot: finalSnapshot,
					Terminal: &taskpkg.CoordinatorTerminal{
						Status: string(looppkg.StatusFailed),
						Cause:  "provider transport failed",
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(final failure) error = %v", err)
		}
		binding, err = globalDB.GetSessionBindingAttempt(ctx, bindingKey, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(after final failure) error = %v", err)
		}
		if binding.State != goal.BindingStateClosed {
			t.Fatalf("binding state after final failure = %q, want %q", binding.State, goal.BindingStateClosed)
		}
		cleanups, err = globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup(after final failure) error = %v", err)
		}
		if len(cleanups) != 1 || cleanups[0].SessionID != sessionID ||
			cleanups[0].Cause != goal.SessionCleanupCauseTerminal {
			t.Fatalf("run-agent final failure cleanups = %#v, want one terminal session cleanup", cleanups)
		}
	})

	t.Run("Should match a settled run-agent output by snapshot generation", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID      = "ws-run-agent-generation"
			loopRunID        = "run-agent-generation"
			staleTaskID      = "task-run-agent-generation-stale"
			staleTaskRunID   = "taskrun-run-agent-generation-stale"
			staleHandle      = "action:run-agent-generation-stale"
			staleSessionID   = "session-run-agent-generation-stale"
			currentTaskID    = "task-run-agent-generation-current"
			currentTaskRunID = "taskrun-run-agent-generation-current"
			currentHandle    = "action:run-agent-generation-current"
			currentSessionID = "session-run-agent-generation-current"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 21, 0, 30, 0, 0, time.UTC)
		loopRun := testLoopRun(loopRunID, now, looppkg.StatusRunning)
		loopRun.WorkspaceID = workspaceID
		created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			TaskID:      staleTaskID,
			TaskRunID:   staleTaskRunID,
			Handle:      staleHandle,
			Generation:  2,
			Attempt:     1,
			Epoch:       4,
		})
		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID:  workspaceID,
			LoopRunID:    loopRunID,
			TaskID:       currentTaskID,
			TaskRunID:    currentTaskRunID,
			Handle:       currentHandle,
			Generation:   1,
			Attempt:      1,
			Epoch:        4,
			InsertOutput: true,
		})
		seedActiveGoalBindingForTest(
			t, globalDB, loopRunID, workspaceID, staleHandle, 5, staleSessionID, now,
		)
		seedActiveGoalBindingForTest(
			t, globalDB, loopRunID, workspaceID, currentHandle, 5, currentSessionID, now,
		)

		endedAt := now.Add(time.Second)
		coordinatorClaim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"run-agent-generation",
			now.Add(2*time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID:      coordinatorClaim.Run.ID,
				ClaimToken: coordinatorClaim.ClaimToken,
				Actor:      coordinatorActorContextForTest(),
				Now:        now.Add(3 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Yield: true,
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  loopRunID,
						Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{
							Outputs: []looppkg.GenerationOutput{
								{
									Generation: 2, NodeID: "execute", Status: "failed",
									TaskRunID: staleTaskRunID, Attempt: 1, Epoch: 4,
								},
								{
									NodeID: "execute", Status: "failed", TaskRunID: currentTaskRunID,
									Attempt: 1, Epoch: 4,
								},
							},
							Attempts: []looppkg.NodeAttempt{{
								LoopRunID: created.ID, Generation: 1, NodeID: "execute", Attempt: 1,
								FailureCode: "provider_failure", Cause: "provider failed",
								Disposition: looppkg.AttemptEscalated,
								StartedAt:   now, EndedAt: &endedAt,
							}},
						},
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
		}

		staleBinding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      staleHandle,
		}, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(stale) error = %v", err)
		}
		if staleBinding.State != goal.BindingStateActive {
			t.Fatalf("stale generation binding state = %q, want %q", staleBinding.State, goal.BindingStateActive)
		}
		currentBinding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      currentHandle,
		}, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(current) error = %v", err)
		}
		if currentBinding.State != goal.BindingStateClosed {
			t.Fatalf("current generation binding state = %q, want %q", currentBinding.State, goal.BindingStateClosed)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(cleanups) != 1 || cleanups[0].SessionID != currentSessionID {
			t.Fatalf("generation cleanups = %#v, want current session only", cleanups)
		}
	})

	t.Run("Should close a live run-agent binding when its loop reaches a terminal boundary", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-run-agent-loop-terminal"
			loopRunID   = "run-agent-loop-terminal"
			taskID      = "task-run-agent-loop-terminal"
			taskRunID   = "taskrun-run-agent-loop-terminal"
			handle      = "action:run-agent-loop-terminal"
			sessionID   = "session-run-agent-loop-terminal"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 20, 50, 0, 0, time.UTC)
		loopRun := testLoopRun(loopRunID, now, looppkg.StatusRunning)
		loopRun.WorkspaceID = workspaceID
		created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}

		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			TaskID:      taskID,
			TaskRunID:   taskRunID,
			Handle:      handle,
			Generation:  1,
			Attempt:     1,
			Epoch:       4,
		})
		seedActiveGoalBindingForTest(t, globalDB, loopRunID, workspaceID, handle, 5, sessionID, now)

		coordinatorClaim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"run-agent-loop-terminal",
			now.Add(time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: coordinatorClaim.Run.ID, ClaimToken: coordinatorClaim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(2 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Snapshot: taskpkg.GenerationSnapshot{LoopRunID: loopRunID, Generation: 1},
					Terminal: &taskpkg.CoordinatorTerminal{
						Status: string(looppkg.StatusFailed), Cause: "terminal worker failure",
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(terminal) error = %v", err)
		}

		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt() error = %v", err)
		}
		if binding.State != goal.BindingStateClosed {
			t.Fatalf("binding state after Loop terminal boundary = %q, want %q", binding.State, goal.BindingStateClosed)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(cleanups) != 1 || cleanups[0].SessionID != sessionID ||
			cleanups[0].Cause != goal.SessionCleanupCauseTerminal {
			t.Fatalf("run-agent Loop terminal cleanups = %#v, want one terminal session cleanup", cleanups)
		}
	})

	t.Run("Should close the exact run-agent binding when its node lane is canceled", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-run-agent-lane-cancel"
			loopRunID   = "run-agent-lane-cancel"
			taskID      = "task-run-agent-lane-cancel"
			taskRunID   = "taskrun-run-agent-lane-cancel"
			handle      = "action:run-agent-lane-cancel"
			sessionID   = "session-run-agent-lane-cancel"
		)
		globalDB := openLoopTestGlobalDB(t, workspaceID)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 21, 0, 0, 0, time.UTC)
		insertGoalSchemaLoopRun(t, globalDB, loopRunID, workspaceID, "catalog", nil)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at)
			 VALUES (?, 1, 0, 'initial', ?)`,
			loopRunID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert run-agent generation error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET generation = 1 WHERE id = ?`,
			loopRunID,
		); err != nil {
			t.Fatalf("advance run-agent generation error = %v", err)
		}

		seedRunAgentCellForTest(t, globalDB, runAgentCellFixture{
			WorkspaceID:  workspaceID,
			LoopRunID:    loopRunID,
			TaskID:       taskID,
			TaskRunID:    taskRunID,
			Handle:       handle,
			Generation:   1,
			Attempt:      1,
			Epoch:        4,
			InsertOutput: true,
		})
		seedActiveGoalBindingForTest(t, globalDB, loopRunID, workspaceID, handle, 5, sessionID, now)

		itemIndex := 0
		result, err := globalDB.RequestNodeCancellation(ctx, looppkg.CancellationMutation{
			WorkspaceID: workspaceID,
			RunID:       loopRunID,
			NodeID:      "execute",
			ItemIndex:   &itemIndex,
			Kind:        looppkg.RunCancelCancel,
			Reason:      "operator canceled lane",
			Actor:       operatorActorContextForTest("operator:lane-cancel"),
			RequestedAt: now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("RequestNodeCancellation() error = %v", err)
		}
		if !result.Applied {
			t.Fatalf("RequestNodeCancellation() = %#v, want applied", result)
		}

		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loopRunID,
			Handle:      handle,
		}, 5)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt() error = %v", err)
		}
		if binding.State != goal.BindingStateClosed {
			t.Fatalf("binding state after node lane cancellation = %q, want %q", binding.State, goal.BindingStateClosed)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(cleanups) != 1 || cleanups[0].SessionID != sessionID ||
			cleanups[0].Cause != goal.SessionCleanupCauseTerminal {
			t.Fatalf("run-agent lane cancellation cleanups = %#v, want one terminal session cleanup", cleanups)
		}
	})

	t.Run(
		"Should fence ordinary action activation by cell owner and publish cleanup for stale creation",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t, "ws-action-binding")
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
			insertGoalSchemaLoopRun(t, globalDB, "run-action-binding", "ws-action-binding", "catalog", nil)
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at)
			 VALUES (?, 1, 0, 'initial', ?)`,
				"run-action-binding",
				store.FormatTimestamp(now),
			); err != nil {
				t.Fatalf("insert action generation error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id, epoch
			 ) VALUES (?, 1, 'execute', 0, 'enqueued', 'taskrun-action-live', 4)`,
				"run-action-binding",
			); err != nil {
				t.Fatalf("insert action cell error = %v", err)
			}

			prepare := func(handle string, sessionID string) goal.BindingKey {
				t.Helper()
				key := goal.BindingKey{
					WorkspaceID: "ws-action-binding",
					LoopRunID:   "run-action-binding",
					Handle:      handle,
				}
				identity := store.SessionCreationIdentity{
					CreationProfileRef: "profile-" + sessionID,
					PolicySpecDigest:   "policy-" + sessionID,
					CreationDigest:     "creation-" + sessionID,
				}
				if _, err := globalDB.PrepareSessionBindingAttempt(ctx, goal.PrepareBindingAttemptRequest{
					Key: key, BindingEpoch: 1, BindingAttemptID: "attempt-" + sessionID,
					SessionID: sessionID, CreationProfileRef: identity.CreationProfileRef,
					PolicySpecDigest: identity.PolicySpecDigest, CreationDigest: identity.CreationDigest,
					CreatedAt: now,
				}); err != nil {
					t.Fatalf("PrepareSessionBindingAttempt(%s) error = %v", handle, err)
				}
				registerGoalSessionIdentityForTest(
					t,
					globalDB,
					goalSessionInfoForTest(sessionID, "ws-action-binding", now),
					identity,
				)
				return key
			}
			cellKey := goal.TurnKey{
				WorkspaceID: "ws-action-binding",
				LoopRunID:   "run-action-binding",
				Generation:  1,
				NodeID:      "execute",
				ItemIndex:   0,
			}
			liveKey := prepare("main", "session-action-live")
			live, stopped, err := globalDB.FinalizeSessionBindingCreation(ctx, goal.ActivateBindingRequest{
				Key: liveKey, CellFence: &goal.BindingCellFence{
					Key: cellKey, Epoch: 4, TaskRunID: "taskrun-action-live",
				},
				ExpectedBindingEpoch: 1,
				ActivatedAt:          now.Add(time.Second),
			})
			if err != nil || stopped || live.State != goal.BindingStateActive {
				t.Fatalf("FinalizeSessionBindingCreation(live cell) = %#v/%t, %v", live, stopped, err)
			}

			staleKey := prepare("stale", "session-action-stale")
			_, _, err = globalDB.FinalizeSessionBindingCreation(ctx, goal.ActivateBindingRequest{
				Key: staleKey, CellFence: &goal.BindingCellFence{
					Key: cellKey, Epoch: 3, TaskRunID: "taskrun-action-live",
				},
				ExpectedBindingEpoch: 1,
				ActivatedAt:          now.Add(2 * time.Second),
			})
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
			stale, err := globalDB.GetSessionBindingAttempt(ctx, staleKey, 1)
			if err != nil {
				t.Fatalf("GetSessionBindingAttempt(stale) error = %v", err)
			}
			if stale.State != goal.BindingStateFailed ||
				stale.FailureCode != goalBindingFailureControlRevokedInFlight {
				t.Fatalf("stale binding = %#v, want failed control-revoked", stale)
			}
			cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
			if err != nil {
				t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
			}
			if len(cleanups) != 1 || cleanups[0].SessionID != "session-action-stale" ||
				cleanups[0].Cause != goal.SessionCleanupCauseControlRevoked {
				t.Fatalf("stale action cleanups = %#v, want one control-revoked session", cleanups)
			}
		},
	)

	t.Run("Should release an unsettled Stop creation cleanup after database reopen", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-cleanup-reopen")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 8, 15, 0, 0, time.UTC)
		insertGoalSchemaLoopRun(
			t, globalDB, "run-goal-cleanup-reopen", "ws-goal-cleanup-reopen", "catalog", nil,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_session_bindings (
				loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
				creation_profile_ref, policy_spec_digest, creation_digest, ownership, state, created_at
			) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, 'run-owned', 'creating', ?)`,
			"run-goal-cleanup-reopen",
			"goal:cleanup-reopen",
			"attempt-cleanup-reopen",
			"session-cleanup-reopen",
			"ws-goal-cleanup-reopen",
			"profile-cleanup-reopen",
			"policy-cleanup-reopen",
			"creation-cleanup-reopen",
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert creating binding error = %v", err)
		}
		if _, err := globalDB.RequestRunCancellation(ctx, looppkg.CancellationMutation{
			WorkspaceID: "ws-goal-cleanup-reopen",
			RunID:       "run-goal-cleanup-reopen",
			Kind:        looppkg.RunCancelCancel,
			Reason:      "operator canceled run",
			Actor:       operatorActorContextForTest("operator-cancel-reopen"),
			RequestedAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("RequestRunCancellation() error = %v", err)
		}
		path := globalDB.Path()
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		if err := reopened.ReconcileGoalSessionCleanup(ctx); err != nil {
			t.Fatalf("ReconcileGoalSessionCleanup() error = %v", err)
		}
		pending, err := reopened.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
		}
		if len(pending) != 1 || pending[0].SessionID != "session-cleanup-reopen" {
			t.Fatalf("reconciled cleanup = %#v", pending)
		}
	})

	t.Run(
		"Should persist run-owned cleanup exclude borrowed sessions and fail a stopped creating attempt",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t, "ws-goal-cleanup")
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 11, 8, 30, 0, 0, time.UTC)
			insertGoalSchemaLoopRun(t, globalDB, "run-goal-cleanup", "ws-goal-cleanup", "catalog", nil)
			seedActiveGoalBindingForTest(
				t,
				globalDB,
				"run-goal-cleanup",
				"ws-goal-cleanup",
				"goal:cleanup",
				1,
				"session-run-owned-cleanup",
				now,
			)
			if err := globalDB.CloseSessionBinding(ctx, goal.CloseBindingRequest{
				Key: goal.BindingKey{
					WorkspaceID: "ws-goal-cleanup",
					LoopRunID:   "run-goal-cleanup",
					Handle:      "goal:cleanup",
				},
				ExpectedBindingEpoch: 1,
				ClosedAt:             now.Add(time.Minute),
			}); err != nil {
				t.Fatalf("CloseSessionBinding(run-owned) error = %v", err)
			}
			pending, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
			if err != nil {
				t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
			}
			if len(pending) != 1 || pending[0].SessionID != "session-run-owned-cleanup" ||
				pending[0].Cause != goal.SessionCleanupCauseTerminal {
				t.Fatalf("run-owned cleanup = %#v", pending)
			}
			if err := globalDB.AcknowledgeGoalSessionCleanup(
				ctx,
				pending[0].CleanupID,
				now.Add(2*time.Minute),
			); err != nil {
				t.Fatalf("AcknowledgeGoalSessionCleanup() error = %v", err)
			}

			insertGoalSchemaLoopRun(t, globalDB, "run-goal-borrowed-cleanup", "ws-goal-cleanup", "catalog", nil)
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_session_bindings (
				loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
				creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
				created_at, activated_at
			) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, 'origin-borrowed', 'active', ?, ?)`,
				"run-goal-borrowed-cleanup",
				"goal:borrowed-cleanup",
				"attempt-borrowed-cleanup",
				"session-borrowed-cleanup",
				"ws-goal-cleanup",
				"profile-borrowed-cleanup",
				"policy-borrowed-cleanup",
				"creation-borrowed-cleanup",
				store.FormatTimestamp(now),
				store.FormatTimestamp(now),
			); err != nil {
				t.Fatalf("insert borrowed binding error = %v", err)
			}
			if err := globalDB.CloseSessionBinding(ctx, goal.CloseBindingRequest{
				Key: goal.BindingKey{
					WorkspaceID: "ws-goal-cleanup",
					LoopRunID:   "run-goal-borrowed-cleanup",
					Handle:      "goal:borrowed-cleanup",
				},
				ExpectedBindingEpoch: 1,
				ClosedAt:             now.Add(time.Minute),
			}); err != nil {
				t.Fatalf("CloseSessionBinding(borrowed) error = %v", err)
			}
			pending, err = globalDB.ClaimGoalSessionCleanup(ctx, 10)
			if err != nil {
				t.Fatalf("ClaimGoalSessionCleanup(after borrowed) error = %v", err)
			}
			if len(pending) != 0 {
				t.Fatalf("borrowed cleanup = %#v, want none", pending)
			}

			insertGoalSchemaLoopRun(t, globalDB, "run-goal-creating-stop", "ws-goal-cleanup", "catalog", nil)
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_session_bindings (
				loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
				creation_profile_ref, policy_spec_digest, creation_digest, ownership, state, created_at
			) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, 'run-owned', 'creating', ?)`,
				"run-goal-creating-stop",
				"goal:creating-stop",
				"attempt-creating-stop",
				"session-creating-stop",
				"ws-goal-cleanup",
				"profile-creating-stop",
				"policy-creating-stop",
				"creation-creating-stop",
				store.FormatTimestamp(now),
			); err != nil {
				t.Fatalf("insert creating binding error = %v", err)
			}
			if _, err := globalDB.RequestRunCancellation(ctx, looppkg.CancellationMutation{
				WorkspaceID: "ws-goal-cleanup",
				RunID:       "run-goal-creating-stop",
				Kind:        looppkg.RunCancelCancel,
				Reason:      "operator canceled run",
				Actor:       operatorActorContextForTest("operator-cancel-creating"),
				RequestedAt: now.Add(time.Minute),
			}); err != nil {
				t.Fatalf("RequestRunCancellation(creating binding) error = %v", err)
			}
			var state, failureCode string
			var activatedAt *time.Time
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT state, failure_code, activated_at FROM loop_session_bindings
			 WHERE loop_run_id = ? AND handle = ? AND binding_epoch = 1`,
				"run-goal-creating-stop",
				"goal:creating-stop",
			).Scan(&state, &failureCode, &activatedAt); err != nil {
				t.Fatalf("read stopped creating binding error = %v", err)
			}
			if state != string(goal.BindingStateFailed) ||
				failureCode != goalBindingFailureStopCreationUnsettled || activatedAt != nil {
				t.Fatalf("stopped creating binding = state:%q failure:%q activated:%v", state, failureCode, activatedAt)
			}
			pending, err = globalDB.ClaimGoalSessionCleanup(ctx, 10)
			if err != nil {
				t.Fatalf("ClaimGoalSessionCleanup(unsettled create) error = %v", err)
			}
			if len(pending) != 0 {
				t.Fatalf("unsettled creation cleanup = %#v, want hidden", pending)
			}
			finalized, stopped, err := globalDB.FinalizeSessionBindingCreation(ctx, goal.ActivateBindingRequest{
				Key: goal.BindingKey{
					WorkspaceID: "ws-goal-cleanup", LoopRunID: "run-goal-creating-stop", Handle: "goal:creating-stop",
				},
				ExpectedBindingEpoch: 1,
				ActivatedAt:          now.Add(2 * time.Minute),
			})
			if err != nil || !stopped || finalized.State != goal.BindingStateFailed ||
				finalized.FailureCode != goalBindingFailureControlRevokedInFlight {
				t.Fatalf("FinalizeSessionBindingCreation(stopped) = %#v/%t, %v", finalized, stopped, err)
			}
			pending, err = globalDB.ClaimGoalSessionCleanup(ctx, 10)
			if err != nil {
				t.Fatalf("ClaimGoalSessionCleanup(settled create) error = %v", err)
			}
			if len(pending) != 1 || pending[0].SessionID != "session-creating-stop" {
				t.Fatalf("settled creation cleanup = %#v", pending)
			}
		},
	)

	t.Run(
		"Should atomically fail one creation attempt advance the prompt identity and prepare its successor",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t, "ws-goal-binding-retry")
			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)
			insertGoalSchemaLoopRun(t, globalDB, "run-goal-binding-retry", "ws-goal-binding-retry", "catalog", nil)
			checkpointKey := goal.TurnKey{
				WorkspaceID: "ws-goal-binding-retry",
				LoopRunID:   "run-goal-binding-retry",
				Generation:  1,
				NodeID:      "goal",
			}
			if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
				Key: checkpointKey, ControlEpoch: 1, Phase: "idle", Status: "active", TurnLimit: 3,
				TaskRunID: "task-goal-binding-retry", ContextState: "unknown",
				ContextNudgeRatio: 0.8, UpdatedAt: now,
			}}); err != nil {
				t.Fatalf("CreateCheckpoint() error = %v", err)
			}
			key := goal.BindingKey{
				WorkspaceID: checkpointKey.WorkspaceID,
				LoopRunID:   checkpointKey.LoopRunID,
				Handle:      "goal:binding-retry",
			}
			if _, err := globalDB.PrepareSessionBindingAttempt(ctx, goal.PrepareBindingAttemptRequest{
				Key: key, BindingEpoch: 1, BindingAttemptID: "attempt-binding-retry-1",
				SessionID: "session-binding-retry-1", CreationProfileRef: "profile-binding-retry",
				PolicySpecDigest: "policy-binding-retry", CreationDigest: "creation-binding-retry-1",
				CreatedAt: now,
			}); err != nil {
				t.Fatalf("PrepareSessionBindingAttempt() error = %v", err)
			}
			prepared, err := globalDB.GetSessionBindingAttempt(ctx, key, 1)
			if err != nil || prepared.State != goal.BindingStateCreating {
				t.Fatalf("GetSessionBindingAttempt(prepared) = %#v, %v", prepared, err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE loop_goal_checkpoints
			 SET binding_epoch = 1, session_id = ?, binding_handle = ?
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'goal' AND item_index = 0`,
				prepared.SessionID,
				key.Handle,
				string(checkpointKey.LoopRunID),
			); err != nil {
				t.Fatalf("seed retry checkpoint binding owner error = %v", err)
			}
			request := goal.AdvanceBindingCreationFailureRequest{
				Key: key, CheckpointKey: checkpointKey, ExpectedControlEpoch: 1,
				ExpectedCheckpointPhase: "idle", ExpectedTaskRunID: "task-goal-binding-retry",
				ExpectedCheckpointBindingEpoch: 1,
				ExpectedCheckpointSessionID:    prepared.SessionID,
				ExpectedCheckpointHandle:       key.Handle,
				ExpectedPromptAttempt:          0, ExpectedBindingEpoch: 1, FailureCode: "provider-no-create",
				ExpectedBindingAttemptID: prepared.BindingAttemptID,
				ExpectedBindingSessionID: prepared.SessionID, ExpectedBindingProfileRef: prepared.CreationProfileRef,
				ExpectedBindingPolicyDigest:   prepared.PolicySpecDigest,
				ExpectedBindingCreationDigest: prepared.CreationDigest,
				FailedAt:                      now.Add(time.Second), PrepareSuccessor: true, SuccessorBindingEpoch: 2,
				SuccessorBindingAttemptID: "attempt-binding-retry-2",
				SuccessorSessionID:        "session-binding-retry-2",
				CreationProfileRef:        "profile-binding-retry",
				PolicySpecDigest:          "policy-binding-retry",
				SuccessorCreationDigest:   "creation-binding-retry-2",
				SuccessorCreatedAt:        now.Add(time.Second),
			}
			if err := globalDB.AdvanceBindingCreationFailure(ctx, request); err != nil {
				t.Fatalf("AdvanceBindingCreationFailure() error = %v", err)
			}
			path := globalDB.Path()
			if err := globalDB.Close(ctx); err != nil {
				t.Fatalf("Close(before retry witness replay) error = %v", err)
			}
			globalDB, err = OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(retry witness replay) error = %v", err)
			}
			t.Cleanup(func() {
				if err := globalDB.Close(context.Background()); err != nil {
					t.Errorf("Close(retry witness replay) error = %v", err)
				}
			})
			if err := globalDB.AdvanceBindingCreationFailure(ctx, request); err != nil {
				t.Fatalf("AdvanceBindingCreationFailure(idempotent after reopen) error = %v", err)
			}
			conflicting := request
			conflicting.SuccessorSessionID = "session-binding-retry-conflict"
			if err := globalDB.AdvanceBindingCreationFailure(ctx, conflicting); err == nil {
				t.Fatal("AdvanceBindingCreationFailure(conflicting successor) error = nil")
			}
			staleOwner := request
			staleOwner.ExpectedTaskRunID = "task-goal-binding-stale"
			if err := globalDB.AdvanceBindingCreationFailure(ctx, staleOwner); err == nil {
				t.Fatal("AdvanceBindingCreationFailure(stale checkpoint owner) error = nil")
			}
			staleBindingEpoch := request
			staleBindingEpoch.ExpectedCheckpointBindingEpoch++
			if err := globalDB.AdvanceBindingCreationFailure(ctx, staleBindingEpoch); err == nil {
				t.Fatal("AdvanceBindingCreationFailure(stale checkpoint binding epoch) error = nil")
			}
			staleBindingSession := request
			staleBindingSession.ExpectedCheckpointSessionID = "session-binding-retry-stale"
			if err := globalDB.AdvanceBindingCreationFailure(ctx, staleBindingSession); err == nil {
				t.Fatal("AdvanceBindingCreationFailure(stale checkpoint binding session) error = nil")
			}
			staleBindingHandle := request
			staleBindingHandle.ExpectedCheckpointHandle = "goal:binding-retry-stale"
			if err := globalDB.AdvanceBindingCreationFailure(ctx, staleBindingHandle); err == nil {
				t.Fatal("AdvanceBindingCreationFailure(stale checkpoint binding handle) error = nil")
			}
			for _, mutate := range []func(*goal.AdvanceBindingCreationFailureRequest){
				func(candidate *goal.AdvanceBindingCreationFailureRequest) {
					candidate.ExpectedBindingAttemptID = "attempt-binding-retry-stale"
				},
				func(candidate *goal.AdvanceBindingCreationFailureRequest) {
					candidate.ExpectedBindingSessionID = "session-binding-retry-stale"
				},
				func(candidate *goal.AdvanceBindingCreationFailureRequest) {
					candidate.ExpectedBindingProfileRef = "profile-binding-retry-stale"
				},
				func(candidate *goal.AdvanceBindingCreationFailureRequest) {
					candidate.ExpectedBindingPolicyDigest = "policy-binding-retry-stale"
				},
				func(candidate *goal.AdvanceBindingCreationFailureRequest) {
					candidate.ExpectedBindingCreationDigest = "creation-binding-retry-stale"
				},
			} {
				candidate := request
				mutate(&candidate)
				if err := globalDB.AdvanceBindingCreationFailure(ctx, candidate); err == nil {
					t.Fatal("AdvanceBindingCreationFailure(stale failed binding identity) error = nil")
				}
			}
			failed, err := globalDB.GetSessionBindingAttempt(ctx, key, 1)
			if err != nil {
				t.Fatalf("GetSessionBindingAttempt(failed) error = %v", err)
			}
			successor, err := globalDB.GetSessionBindingAttempt(ctx, key, 2)
			if err != nil {
				t.Fatalf("GetSessionBindingAttempt(successor) error = %v", err)
			}
			checkpoint, err := globalDB.LoadCheckpoint(ctx, checkpointKey)
			if err != nil {
				t.Fatalf("LoadCheckpoint() error = %v", err)
			}
			if failed.State != goal.BindingStateFailed || failed.FailureCode != request.FailureCode ||
				successor.State != goal.BindingStateCreating || checkpoint.PromptAttempt != 1 ||
				checkpoint.BindingEpoch != 2 || checkpoint.SessionID != successor.SessionID {
				t.Fatalf("binding retry state = failed:%#v successor:%#v checkpoint:%#v", failed, successor, checkpoint)
			}
			var witnessCount int
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM loop_goal_binding_retry_witnesses
			 WHERE loop_run_id = ? AND handle = ? AND failed_binding_epoch = 1`,
				string(key.LoopRunID),
				key.Handle,
			).Scan(&witnessCount); err != nil {
				t.Fatalf("count binding retry witnesses error = %v", err)
			}
			if witnessCount != 1 {
				t.Fatalf("binding retry witness count = %d, want 1", witnessCount)
			}
		},
	)

	t.Run("Should preserve one active epoch and terminalize failed creation before activation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-binding", "ws-goal-binding-foreign")
		now := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
		originIdentity := store.SessionCreationIdentity{
			CreationProfileRef: "profile-v1",
			PolicySpecDigest:   "policy-v1",
			CreationDigest:     "creation-origin",
		}
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest("origin-session", "ws-goal-binding", now),
			originIdentity,
		)
		originSessionID := "origin-session"
		insertGoalSchemaLoopRun(t, globalDB, "run-binding", "ws-goal-binding", "session", &originSessionID)

		key := goal.BindingKey{
			WorkspaceID: "ws-goal-binding",
			LoopRunID:   "run-binding",
			Handle:      "goal:binding",
		}
		originRequest := seedOriginBindingOwnerForTest(
			t, globalDB, key, 1, "binding-attempt-1", "origin-session", originIdentity, now,
		)
		first, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), originRequest)
		if err != nil {
			t.Fatalf("GetOrCreateSessionBinding(first) error = %v", err)
		}
		second, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), originRequest)
		if err != nil {
			t.Fatalf("GetOrCreateSessionBinding(second) error = %v", err)
		}
		if first.BindingEpoch != 1 || first.State != goal.BindingStateActive || !reflect.DeepEqual(second, first) {
			t.Fatalf("binding repeats = %#v/%#v, want identical active epoch 1", first, second)
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = ? AND handle = ? AND binding_epoch = 1`,
			now.Add(time.Second).Format(time.RFC3339Nano),
			string(key.LoopRunID),
			key.Handle,
		); err != nil {
			t.Fatalf("close borrowed origin binding error = %v", err)
		}
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), originRequest); err == nil {
			t.Fatal("GetOrCreateSessionBinding(same generation closed origin) error = nil")
		}
		reactivationRequest := seedOriginBindingOwnerForTest(
			t, globalDB, key, 2, "binding-attempt-2", "origin-session", originIdentity, now.Add(2*time.Second),
		)
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("terminalize origin binding Run error = %v", err)
		}
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), reactivationRequest); err == nil {
			t.Fatal("GetOrCreateSessionBinding(terminal Run) error = nil")
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_runs SET status = 'running' WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("reactivate origin binding Run error = %v", err)
		}
		staleControl := reactivationRequest
		staleControl.ExpectedControlEpoch++
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), staleControl); err == nil {
			t.Fatal("GetOrCreateSessionBinding(stale control owner) error = nil")
		}
		reactivated, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), reactivationRequest)
		if err != nil {
			t.Fatalf("GetOrCreateSessionBinding(later generation exact origin) error = %v", err)
		}
		if reactivated.State != goal.BindingStateActive || reactivated.ClosedAt != nil ||
			reactivated.BindingEpoch != 1 || reactivated.SessionID != originRequest.SessionID ||
			reactivated.AdoptedGeneration != 2 ||
			reactivated.AdoptionAttemptID != reactivationRequest.BindingAttemptID {
			t.Fatalf("reactivated origin = %#v, want exact active epoch 1", reactivated)
		}
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), reactivationRequest); err != nil {
			t.Fatalf("GetOrCreateSessionBinding(later generation replay) error = %v", err)
		}
		changedAttempt := reactivationRequest
		changedAttempt.BindingAttemptID = "binding-attempt-2-conflict"
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), changedAttempt); err == nil {
			t.Fatal("GetOrCreateSessionBinding(changed same-generation attempt) error = nil")
		}

		drifted := reactivationRequest
		drifted.PolicySpecDigest = "policy-drifted"
		if _, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), drifted); err == nil {
			t.Fatal("GetOrCreateSessionBinding(policy drift) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeContinuousBindingMismatch)
		}
		skippedAttempt := goal.PrepareBindingAttemptRequest{
			Key:                key,
			BindingEpoch:       3,
			BindingAttemptID:   "binding-attempt-skipped",
			SessionID:          "session-skipped",
			CreationProfileRef: "profile-v1",
			PolicySpecDigest:   "policy-v1",
			CreationDigest:     "creation-session-skipped",
			CreatedAt:          now.Add(time.Minute),
		}
		if _, err := globalDB.PrepareSessionBindingAttempt(testutil.Context(t), skippedAttempt); err == nil {
			t.Fatal("PrepareSessionBindingAttempt(skipped epoch) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}

		creating, err := globalDB.PrepareSessionBindingAttempt(
			testutil.Context(t),
			goal.PrepareBindingAttemptRequest{
				Key:                key,
				BindingEpoch:       2,
				BindingAttemptID:   "binding-attempt-2",
				SessionID:          "session-2",
				CreationProfileRef: "profile-v1",
				PolicySpecDigest:   "policy-v1",
				CreationDigest:     "creation-session-2",
				CreatedAt:          now.Add(time.Minute),
			},
		)
		if err != nil {
			t.Fatalf("PrepareSessionBindingAttempt() error = %v", err)
		}
		if creating.State != goal.BindingStateCreating || creating.BindingEpoch != 2 {
			t.Fatalf("creating binding = %#v, want epoch 2 creating", creating)
		}
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest("session-2", "ws-goal-binding", now.Add(3*time.Minute)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-v1",
				PolicySpecDigest:   "policy-v1",
				CreationDigest:     "creation-session-2",
			},
		)
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints
			 SET control_grant_id = 1, control_grant_kind = 'reseed',
			     control_grant_cause = 'goal_reseed_confirmation_required', control_grant_turn = 0,
			     control_grant_scope = 'rotate-binding', control_grant_consumed = 0
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			string(reactivationRequest.CheckpointKey.LoopRunID),
			reactivationRequest.CheckpointKey.Generation,
			string(reactivationRequest.CheckpointKey.NodeID),
			reactivationRequest.CheckpointKey.ItemIndex,
		); err != nil {
			t.Fatalf("seed reseed activation grant error = %v", err)
		}
		activation := goal.ActivateBindingRequest{
			Key:                  key,
			CheckpointKey:        &reactivationRequest.CheckpointKey,
			ExpectedBindingEpoch: 2,
			ExpectedControlEpoch: reactivationRequest.ExpectedControlEpoch,
			GrantID:              1,
			ActivatedAt:          now.Add(4 * time.Minute),
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`CREATE TRIGGER fail_goal_reseed_outbox
			 BEFORE INSERT ON loop_goal_session_outbox
			 BEGIN SELECT RAISE(ABORT, 'forced reseed outbox failure'); END`,
		); err != nil {
			t.Fatalf("create reseed outbox failure trigger error = %v", err)
		}
		if _, err := globalDB.ActivateSessionBinding(ctx, activation); err == nil {
			t.Fatal("ActivateSessionBinding(forced outbox failure) error = nil")
		}
		stillOrigin, err := globalDB.GetActiveSessionBinding(ctx, key)
		if err != nil {
			t.Fatalf("GetActiveSessionBinding(after rollback) error = %v", err)
		}
		if stillOrigin.BindingEpoch != 1 || stillOrigin.State != goal.BindingStateActive {
			t.Fatalf("active binding after reseed rollback = %#v, want origin epoch 1", stillOrigin)
		}
		stillCreating, err := globalDB.GetSessionBindingAttempt(ctx, key, 2)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(after rollback) error = %v", err)
		}
		if stillCreating.State != goal.BindingStateCreating {
			t.Fatalf("successor after reseed rollback = %#v, want creating", stillCreating)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_goal_reseed_outbox`); err != nil {
			t.Fatalf("drop reseed outbox failure trigger error = %v", err)
		}

		activated, err := globalDB.ActivateSessionBinding(ctx, activation)
		if err != nil {
			t.Fatalf("ActivateSessionBinding() error = %v", err)
		}
		if activated.State != goal.BindingStateActive || activated.BindingEpoch != 2 {
			t.Fatalf("activated binding = %#v, want epoch 2 active", activated)
		}
		closedOrigin, err := globalDB.GetSessionBindingAttempt(testutil.Context(t), key, 1)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(origin) error = %v", err)
		}
		if closedOrigin.State != goal.BindingStateClosed || closedOrigin.ClosedAt == nil {
			t.Fatalf("origin after reseed = %#v, want closed borrowed binding", closedOrigin)
		}
		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(reseed) error = %v", err)
		}
		if len(pending) != 1 || pending[0].Cause != goal.SessionOutboxCauseReseed ||
			pending[0].BoundSessionID == nil || *pending[0].BoundSessionID != "session-2" {
			t.Fatalf("reseed outbox = %#v, want one successor projection", pending)
		}

		foreignKey := key
		foreignKey.WorkspaceID = "ws-goal-binding-foreign"
		if _, err := globalDB.GetActiveSessionBinding(testutil.Context(t), foreignKey); err == nil {
			t.Fatal("GetActiveSessionBinding(foreign workspace) error = nil")
		}
	})

	t.Run("Should consume one reseed grant across concurrent activation and reopen", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-reseed-grant")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 13, 30, 0, 0, time.UTC)
		originIdentity := store.SessionCreationIdentity{
			CreationProfileRef: "profile-reseed",
			PolicySpecDigest:   "policy-reseed",
			CreationDigest:     "creation-reseed-origin",
		}
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest("origin-reseed", "ws-goal-reseed-grant", now),
			originIdentity,
		)
		originSessionID := "origin-reseed"
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-reseed-grant",
			"ws-goal-reseed-grant",
			"session",
			&originSessionID,
		)
		bindingKey := goal.BindingKey{
			WorkspaceID: "ws-goal-reseed-grant",
			LoopRunID:   "run-reseed-grant",
			Handle:      "goal:reseed-grant",
		}
		originRequest := seedOriginBindingOwnerForTest(
			t, globalDB, bindingKey, 1, "reseed-origin-attempt", originSessionID, originIdentity, now,
		)
		if _, err := globalDB.GetOrCreateSessionBinding(ctx, originRequest); err != nil {
			t.Fatalf("GetOrCreateSessionBinding(origin) error = %v", err)
		}
		checkpointKey := goal.TurnKey{
			WorkspaceID: "ws-goal-reseed-grant",
			LoopRunID:   "run-reseed-grant",
			Generation:  1,
			NodeID:      "converge",
		}
		usageSequence := int64(10)
		if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
			Key:                        checkpointKey,
			ControlEpoch:               2,
			Phase:                      "idle",
			Status:                     "active",
			TurnsUsed:                  1,
			TurnLimit:                  3,
			TaskRunID:                  "goal-reseed-segment-2",
			SessionID:                  originSessionID,
			BindingHandle:              bindingKey.Handle,
			BindingEpoch:               1,
			ContextState:               "known",
			UsageSequence:              &usageSequence,
			RecoveryStreak:             2,
			CompactionRecoveryRequired: true,
			ControlGrant: &goal.ControlGrant{
				ID:       7,
				Kind:     goal.ControlGrantReseed,
				Cause:    looppkg.ReasonCodeGoalReseedConfirmationRequired,
				Turn:     1,
				Scope:    goal.ControlGrantScopeRotateBinding,
				Consumed: false,
			},
			ContextNudgeRatio: 0.8,
			UpdatedAt:         now,
		}}); err != nil {
			t.Fatalf("CreateCheckpoint(reseed grant) error = %v", err)
		}
		newIdentity := store.SessionCreationIdentity{
			CreationProfileRef: originIdentity.CreationProfileRef,
			PolicySpecDigest:   originIdentity.PolicySpecDigest,
			CreationDigest:     "creation-reseed-successor",
		}
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest("session-reseed-successor", "ws-goal-reseed-grant", now.Add(time.Minute)),
			newIdentity,
		)
		if _, err := globalDB.PrepareSessionBindingAttempt(ctx, goal.PrepareBindingAttemptRequest{
			Key:                bindingKey,
			BindingEpoch:       2,
			BindingAttemptID:   "reseed-successor-attempt",
			SessionID:          "session-reseed-successor",
			CreationProfileRef: newIdentity.CreationProfileRef,
			PolicySpecDigest:   newIdentity.PolicySpecDigest,
			CreationDigest:     newIdentity.CreationDigest,
			CreatedAt:          now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("PrepareSessionBindingAttempt(reseed) error = %v", err)
		}
		activation := goal.ActivateBindingRequest{
			Key:                  bindingKey,
			CheckpointKey:        &checkpointKey,
			ExpectedBindingEpoch: 2,
			ExpectedControlEpoch: 2,
			GrantID:              7,
			ActivatedAt:          now.Add(2 * time.Minute),
		}
		const contenders = 8
		errorsCh := make(chan error, contenders)
		var wait sync.WaitGroup
		wait.Add(contenders)
		for range contenders {
			go func() {
				defer wait.Done()
				binding, err := globalDB.ActivateSessionBinding(ctx, activation)
				if err == nil && (binding.BindingEpoch != 2 || binding.State != goal.BindingStateActive) {
					err = errors.New("activated binding identity changed")
				}
				errorsCh <- err
			}()
		}
		wait.Wait()
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Errorf("ActivateSessionBinding(concurrent) error = %v", err)
			}
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, checkpointKey)
		if err != nil {
			t.Fatalf("LoadCheckpoint(reseed consumed) error = %v", err)
		}
		if checkpoint.ControlGrant == nil || !checkpoint.ControlGrant.Consumed ||
			checkpoint.ControlGrant.ID != activation.GrantID {
			t.Fatalf("reseed grant after activation = %#v", checkpoint.ControlGrant)
		}
		if checkpoint.RecoveryStreak != 0 || checkpoint.CompactionRecoveryRequired ||
			checkpoint.CompactionBaselineUsed != nil {
			t.Fatalf("recovery state after successful reseed = %#v", checkpoint)
		}
		var activeCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_session_bindings
			 WHERE loop_run_id = ? AND handle = ? AND state = 'active'`,
			string(bindingKey.LoopRunID),
			bindingKey.Handle,
		).Scan(&activeCount); err != nil {
			t.Fatalf("count active reseed bindings error = %v", err)
		}
		if activeCount != 1 {
			t.Fatalf("active reseed bindings = %d, want 1", activeCount)
		}
		reopened, err := OpenGlobalDB(ctx, globalDB.path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		binding, err := reopened.ActivateSessionBinding(ctx, activation)
		if err != nil || binding.BindingEpoch != 2 || binding.State != goal.BindingStateActive {
			t.Fatalf("ActivateSessionBinding(reopen) = %#v, %v", binding, err)
		}
	})

	t.Run("Should converge concurrent origin adoption on one active epoch", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-binding-race")
		now := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)
		identity := store.SessionCreationIdentity{
			CreationProfileRef: "profile-race",
			PolicySpecDigest:   "policy-race",
			CreationDigest:     "creation-race",
		}
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest("session-race", "ws-goal-binding-race", now),
			identity,
		)
		originSessionID := "session-race"
		insertGoalSchemaLoopRun(t, globalDB, "run-binding-race", "ws-goal-binding-race", "session", &originSessionID)
		key := goal.BindingKey{
			WorkspaceID: "ws-goal-binding-race",
			LoopRunID:   "run-binding-race",
			Handle:      "goal:race",
		}
		req := seedOriginBindingOwnerForTest(
			t, globalDB, key, 1, "binding-race-1", "session-race", identity, now,
		)

		const contenders = 12
		results := make(chan goal.SessionBinding, contenders)
		errorsCh := make(chan error, contenders)
		var wait sync.WaitGroup
		wait.Add(contenders)
		for range contenders {
			go func() {
				defer wait.Done()
				binding, err := globalDB.GetOrCreateSessionBinding(testutil.Context(t), req)
				if err != nil {
					errorsCh <- err
					return
				}
				results <- binding
			}()
		}
		wait.Wait()
		close(results)
		close(errorsCh)
		for err := range errorsCh {
			t.Errorf("GetOrCreateSessionBinding(race) error = %v", err)
		}
		for binding := range results {
			if binding.BindingEpoch != 1 || binding.State != goal.BindingStateActive ||
				binding.AdoptedGeneration != 1 {
				t.Errorf("race binding = %#v, want active epoch 1", binding)
			}
		}
		var total, active int
		if err := globalDB.db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*), SUM(CASE WHEN state = 'active' THEN 1 ELSE 0 END)
			 FROM loop_session_bindings WHERE loop_run_id = ? AND handle = ?`,
			"run-binding-race",
			"goal:race",
		).Scan(&total, &active); err != nil {
			t.Fatalf("query binding race result error = %v", err)
		}
		if total != 1 || active != 1 {
			t.Fatalf("binding race rows = total:%d active:%d, want 1/1", total, active)
		}
	})
}

func seedOriginBindingOwnerForTest(
	t testing.TB,
	globalDB *GlobalDB,
	key goal.BindingKey,
	generation int,
	attemptID string,
	sessionID string,
	identity store.SessionCreationIdentity,
	at time.Time,
) goal.GetOrCreateBindingRequest {
	t.Helper()

	ctx := testutil.Context(t)
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET status = 'running', generation = ? WHERE id = ? AND workspace_id = ?`,
		generation,
		string(key.LoopRunID),
		string(key.WorkspaceID),
	); err != nil {
		t.Fatalf("advance origin binding Run generation error = %v", err)
	}
	checkpointKey := goal.TurnKey{
		WorkspaceID: key.WorkspaceID,
		LoopRunID:   key.LoopRunID,
		Generation:  generation,
		NodeID:      "origin-adoption",
	}
	taskRunID := "task-origin-adoption-" + string(key.LoopRunID) + "-" + strconv.Itoa(generation)
	if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
		Key: checkpointKey, ControlEpoch: 1, Phase: "idle", Status: "active", TurnLimit: 3,
		TaskRunID: taskRunID, ContextState: "unknown", ContextNudgeRatio: 0.8, UpdatedAt: at,
	}}); err != nil {
		t.Fatalf("CreateCheckpoint(origin adoption) error = %v", err)
	}
	return goal.GetOrCreateBindingRequest{
		Key: key, CheckpointKey: checkpointKey, ExpectedControlEpoch: 1,
		ExpectedCheckpointPhase: "idle", ExpectedTaskRunID: taskRunID,
		BindingAttemptID: attemptID, SessionID: sessionID,
		CreationProfileRef: identity.CreationProfileRef,
		PolicySpecDigest:   identity.PolicySpecDigest, CreationDigest: identity.CreationDigest,
		Ownership: goal.BindingOwnershipOriginBorrowed, CreatedAt: at,
	}
}

func goalSessionInfoForTest(id string, workspaceID string, now time.Time) store.SessionInfo {
	return store.SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            id,
		AgentName:     "codex",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type runAgentCellFixture struct {
	WorkspaceID  string
	LoopRunID    string
	TaskID       string
	TaskRunID    string
	Handle       string
	Generation   int
	Attempt      int
	Epoch        int64
	InsertOutput bool
}

func seedRunAgentCellForTest(
	t *testing.T,
	globalDB *GlobalDB,
	fixture runAgentCellFixture,
) taskpkg.Run {
	t.Helper()

	ctx := testutil.Context(t)
	taskRecord := workspaceTaskRecordForTest(fixture.TaskID, fixture.WorkspaceID)
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"generation":     fixture.Generation,
		"node_id":        "execute",
		"item_index":     0,
		"attempt":        fixture.Attempt,
		"epoch":          fixture.Epoch,
		"node_kind":      string(dsl.ActionRunAgent),
		"session_handle": fixture.Handle,
	})
	if err != nil {
		t.Fatalf("json.Marshal(run-agent metadata) error = %v", err)
	}
	run := taskRunForTest(fixture.TaskRunID, fixture.TaskID)
	run.RunKind = taskpkg.RunKindWorker
	run.LoopRunID = fixture.LoopRunID
	run.Metadata = metadata
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	if !fixture.InsertOutput {
		return run
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, task_run_id, attempt, epoch
		 ) VALUES (?, ?, 'execute', 0, 'enqueued', ?, ?, ?)`,
		fixture.LoopRunID,
		fixture.Generation,
		fixture.TaskRunID,
		fixture.Attempt,
		fixture.Epoch,
	); err != nil {
		t.Fatalf("insert run-agent cell error = %v", err)
	}
	return run
}

func seedActiveGoalBindingForTest(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	workspaceID string,
	handle string,
	bindingEpoch int64,
	sessionID string,
	now time.Time,
) {
	t.Helper()
	seedActiveGoalBindingWithOwnershipForTest(
		t,
		globalDB,
		runID,
		workspaceID,
		handle,
		bindingEpoch,
		sessionID,
		goal.BindingOwnershipRunOwned,
		now,
	)
}

func seedActiveGoalBindingWithOwnershipForTest(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	workspaceID string,
	handle string,
	bindingEpoch int64,
	sessionID string,
	ownership goal.BindingOwnership,
	now time.Time,
) {
	t.Helper()

	profileRef := "profile:" + runID
	policyDigest := "policy:" + runID
	creationDigest := "creation:" + runID
	registerGoalSessionIdentityForTest(
		t,
		globalDB,
		goalSessionInfoForTest(sessionID, workspaceID, now),
		store.SessionCreationIdentity{
			CreationProfileRef: profileRef,
			PolicySpecDigest:   policyDigest,
			CreationDigest:     creationDigest,
		},
	)
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO loop_session_bindings (
			loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
			creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
			created_at, activated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		runID,
		handle,
		bindingEpoch,
		"binding-attempt:"+runID+":"+handle,
		sessionID,
		workspaceID,
		profileRef,
		policyDigest,
		creationDigest,
		ownership,
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("insert active Goal test binding error = %v", err)
	}
}

func registerGoalSessionIdentityForTest(
	t *testing.T,
	globalDB *GlobalDB,
	info store.SessionInfo,
	identity store.SessionCreationIdentity,
) {
	t.Helper()

	if _, err := globalDB.RegisterSessionWithCreationIdentity(testutil.Context(t), info, identity); err != nil {
		t.Fatalf("RegisterSessionWithCreationIdentity(%q) error = %v", info.ID, err)
	}
}

func requireSessionCreationIdentityMismatch(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, store.ErrSessionCreationIdentityMismatch) {
		t.Fatalf("error = %v, want %v", err, store.ErrSessionCreationIdentityMismatch)
	}
	requireGoalReasonCode(t, err, looppkg.ReasonCodeSessionCreationIdentityMismatch)
}

func requireGoalReasonCode(t *testing.T, err error, want looppkg.ReasonCode) {
	t.Helper()
	reason, reasonMatched := errors.AsType[*looppkg.ReasonError](err)
	if !reasonMatched || reason.Code != want {
		t.Fatalf("error = %v, want reason code %q", err, want)
	}
}
