//go:build integration

package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/workspace"
)

func TestTaskRoleRuntimeRecoveredSessionOwnerIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should recover a parked run under the same persisted session owner", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		manager, err := task.NewManager(task.WithStore(db))
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		actor := coordinatorTaskActor()
		owner := &session.Info{
			ID: "sess-owner", ProfileID: storepkg.DefaultProfileID, AgentName: "zeta-agent",
			Type: session.SessionTypeUser, State: session.StateStopped, StopReason: storepkg.StopAgentCrashed,
		}
		maxAttempts := 2
		record, err := manager.CreateTask(ctx, task.CreateTask{
			ProfileID: owner.ProfileID, Scope: task.ScopeGlobal, Title: "Recover session-owned task",
			Owner: &task.Ownership{Kind: task.OwnerKindAgentSession, Ref: owner.ID}, MaxAttempts: &maxAttempts,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, task.EnqueueRun{TaskID: record.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(ctx, run.ID, "unclaimed", actor); err != nil {
			t.Fatalf("MarkRunNeedsAttention() error = %v", err)
		}
		recovered, err := manager.RecoverRun(ctx, run.ID, task.RecoverRunRequest{Reason: "operator recovery"}, actor)
		if err != nil {
			t.Fatalf("RecoverRun() error = %v", err)
		}
		if recovered.Run.PreviousRunID != run.ID || recovered.Run.Status != task.TaskRunStatusQueued {
			t.Fatalf("RecoverRun() continuation = %#v, want queued run linked to %q", recovered.Run, run.ID)
		}
		stored, err := db.GetTask(ctx, recovered.Run.TaskID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.Owner == nil || *stored.Owner != *record.Owner {
			t.Fatalf("GetTask().Owner = %#v, want %#v", stored.Owner, record.Owner)
		}

		sessions := &taskRoleRuntimeSessions{infos: []*session.Info{owner}}
		runtime := newTaskRoleRuntimeForTest(t, db, sessions)
		if err := runtime.activateForStarvation(ctx, stored, recovered.Run, starvationSpawner{}); err != nil {
			t.Fatalf("activateForStarvation() error = %v", err)
		}
		resumed, err := sessions.Status(ctx, owner.ID)
		if err != nil || resumed == nil || resumed.State != session.StateActive {
			t.Fatalf("owner status = %#v, error = %v, want active", resumed, err)
		}

		for _, sessionID := range []string{"sess-same-agent", resumed.ID} {
			claimant := actor
			claimant.Actor = task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: sessionID}
			claimant.Scope = task.CallerScope{SessionID: sessionID}
			result, claimErr := manager.ClaimNextRun(ctx, task.ClaimCriteria{
				RunID: recovered.Run.ID, AgentName: resumed.AgentName,
			}, claimant)
			if sessionID != resumed.ID {
				if !errors.Is(claimErr, task.ErrNoClaimableRun) {
					t.Fatalf("ClaimNextRun(same agent, different session) error = %v, want no claimable run", claimErr)
				}
				continue
			}
			if claimErr != nil || result == nil || result.Run.ID != recovered.Run.ID ||
				result.Run.SessionID != resumed.ID {
				t.Fatalf("ClaimNextRun(owner) error = %v or continuation is not bound to the owner session", claimErr)
			}
		}
	})
}

func TestTaskRoleRuntimeWorktreeReuseIntegrationIT030(t *testing.T) {
	t.Run("Should reuse only the same referenced worktree and never reuse per-run sessions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(ctx, workspace.Workspace{
			ID:        "ws-task-role-worktrees",
			Name:      "Task role worktrees",
			RootDir:   t.TempDir(),
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}

		manager, err := task.NewManager(
			task.WithStore(db),
			task.WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		sessions := &taskRoleRuntimeSessions{}
		runtime, err := newTaskRoleRuntime(db, sessions, discardLogger(), func() time.Time { return now })
		if err != nil {
			t.Fatalf("newTaskRoleRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.shutdown(context.Background()); err != nil {
				t.Errorf("task role runtime shutdown error = %v", err)
			}
		})

		actor := coordinatorTaskActor()
		enqueue := func(id string, policy task.WorktreePolicy) task.Run {
			t.Helper()
			record, createErr := manager.CreateTask(ctx, task.CreateTask{
				ProfileID:   storepkg.DefaultProfileID,
				ID:          id,
				Scope:       task.ScopeWorkspace,
				WorkspaceID: "ws-task-role-worktrees",
				Title:       "Role worktree " + id,
				Owner:       &task.Ownership{Kind: task.OwnerKindPool, Ref: "worker-agent"},
			}, actor)
			if createErr != nil {
				t.Fatalf("CreateTask(%s) error = %v", id, createErr)
			}
			profile := task.DefaultExecutionProfile(record.ID)
			profile.Worktree = policy
			if _, upsertErr := db.UpsertExecutionProfile(ctx, &profile); upsertErr != nil {
				t.Fatalf("UpsertExecutionProfile(%s) error = %v", id, upsertErr)
			}
			run, enqueueErr := manager.EnqueueRun(ctx, task.EnqueueRun{TaskID: record.ID}, actor)
			if enqueueErr != nil {
				t.Fatalf("EnqueueRun(%s) error = %v", id, enqueueErr)
			}
			return *run
		}
		activate := func(run task.Run) {
			t.Helper()
			runtime.OnTaskRunEnqueued(ctx, hooks.TaskRunEnqueuedPayload{
				TaskID: run.TaskID, RunID: run.ID,
			})
			runtime.wg.Wait()
		}

		refA := enqueue("task-ref-a", task.WorktreePolicy{
			Mode: task.WorktreeModeRef, WorktreeRef: "wt-docs",
		})
		activate(refA)
		activate(refA)
		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("same-ref create count = %d, want %d", got, want)
		}
		if got, want := sessions.createCall(0).Worktree, "wt-docs"; got != want {
			t.Fatalf("same-ref CreateOpts.Worktree = %q, want %q", got, want)
		}

		refB := enqueue("task-ref-b", task.WorktreePolicy{
			Mode: task.WorktreeModeRef, WorktreeRef: "wt-api",
		})
		activate(refB)
		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("changed-ref create count = %d, want %d", got, want)
		}
		if got, want := sessions.createCall(1).Worktree, "wt-api"; got != want {
			t.Fatalf("changed-ref CreateOpts.Worktree = %q, want %q", got, want)
		}
		if first, second := sessions.createCall(0).Name, sessions.createCall(1).Name; first == second {
			t.Fatalf("role session names = %q, want distinct worktree fingerprints", first)
		}

		perRun := enqueue("task-per-run", task.WorktreePolicy{Mode: task.WorktreeModePerRun})
		activate(perRun)
		activate(perRun)
		if got, want := sessions.createCount(), 4; got != want {
			t.Fatalf("per-run create count = %d, want %d without reuse", got, want)
		}
	})
}
