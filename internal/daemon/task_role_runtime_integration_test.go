//go:build integration

package daemon

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/hooks"
	schedulerpkg "github.com/compozy/compozy/internal/scheduler"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/jonboulle/clockwork"
)

func TestTaskRoleRuntimeRecoveredSessionOwnerIntegration(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		cause       session.StopCause
		disabled    bool
		wantResumed bool
	}{
		{
			name:  "Should recover a parked run under the same owner after daemon shutdown",
			cause: session.CauseShutdown, wantResumed: true,
		},
		{name: "Should leave an explicitly stopped owner stopped", cause: session.CauseUserRequested},
		{name: "Should leave a process-exited owner stopped", cause: session.CauseProcessExited},
		{name: "Should honor disabled automatic restart continuation", cause: session.CauseShutdown, disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testRecoveredSessionOwner(t, tc.cause, tc.disabled, tc.wantResumed)
		})
	}
}

func testRecoveredSessionOwner(t *testing.T, cause session.StopCause, disabled, wantResumed bool) {
	t.Helper()
	ctx := testutil.Context(t)
	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Session.Restart.AutoContinue = !disabled
	resolved := newHarnessIntegrationWorkspace(t, homePaths, cfg, t.TempDir())
	if err := db.InsertWorkspace(ctx, workspace.Workspace{
		ID: resolved.ID, Name: resolved.Name, RootDir: resolved.RootDir, CreatedAt: taskRoleRuntimeClock,
		UpdatedAt: taskRoleRuntimeClock,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	driver := newHarnessIntegrationDriver()
	sessions := newHarnessIntegrationManager(
		t,
		homePaths,
		SessionManagerDeps{},
		resolved,
		driver,
		session.WithSessionCatalog(db),
		session.WithSessionInputQueueStore(db),
		session.WithSessionPromptAdmissionStore(db),
		session.WithEventLedger(db),
	)
	t.Cleanup(func() {
		if err := sessions.Shutdown(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	owner, err := sessions.Create(ctx, new(session.CreateOpts{
		ProfileID: storepkg.DefaultProfileID, Workspace: resolved.ID, AgentName: resolved.Agents[0].Name,
	}))
	if err != nil {
		t.Fatalf("Create() owner error = %v", err)
	}
	events, err := sessions.Prompt(ctx, owner.ID, "Record a settled turn before the stop.")
	if err != nil {
		t.Fatalf("Prompt() owner error = %v", err)
	}
	drainHarnessIntegrationEvents(events)
	if err := sessions.StopWithCause(ctx, owner.ID, cause, "test stop"); err != nil {
		t.Fatalf("StopWithCause() owner error = %v", err)
	}

	manager, err := task.NewManager(task.WithStore(db))
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor := coordinatorTaskActor()
	record, err := manager.CreateTask(ctx, task.CreateTask{
		ProfileID: storepkg.DefaultProfileID, Scope: task.ScopeWorkspace, WorkspaceID: resolved.ID,
		Title: "Recover session-owned task", MaxAttempts: new(2),
		Owner: &task.Ownership{Kind: task.OwnerKindAgentSession, Ref: owner.ID},
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

	tasks := &taskRuntime{manager: manager, store: db}
	tasks.roles.Store(newTaskRoleRuntimeForTest(t, db, sessions))
	scheduler := newRecoveredRunTestScheduler(t, tasks, sessions, recovered.Run.QueuedAt)
	activation, err := scheduler.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() starvation error = %v", err)
	}
	info, err := sessions.StoredStatus(ctx, owner.ID)
	if err != nil || info == nil {
		t.Fatalf("StoredStatus() owner error = %v", err)
	}
	driver.mu.Lock()
	starts := len(driver.startCalls)
	driver.mu.Unlock()
	if !wantResumed {
		if activation.SpawnRequested != 0 || info.State != session.StateStopped || starts != 1 {
			t.Fatalf(
				"owner state = %s, starts = %d, spawn requests = %d, want stopped and no eligible replacement",
				info.State,
				starts,
				activation.SpawnRequested,
			)
		}
		return
	}
	if !slices.Equal(activation.SpawnRequestedRunIDs, []string{recovered.Run.ID}) ||
		info.State != session.StateActive || info.AgentName != owner.AgentName || starts != 2 {
		t.Fatalf(
			"owner state = %s, agent = %q, starts = %d, spawn requests = %v",
			info.State,
			info.AgentName,
			starts,
			activation.SpawnRequestedRunIDs,
		)
	}
	wake, err := scheduler.RunOnce(ctx)
	if err != nil || wake.WakeSucceeded != 1 || !slices.Equal(wake.SelectedRunIDs, []string{recovered.Run.ID}) {
		t.Fatalf("RunOnce() after resume = %+v, error = %v, want a wake for the continuation", wake, err)
	}
	assertRecoveredRunClaim(t, manager, recovered.Run, info, actor)
}

func newRecoveredRunTestScheduler(
	t *testing.T, tasks *taskRuntime, sessions *session.Manager, queuedAt time.Time,
) *schedulerpkg.Scheduler {
	t.Helper()
	ctx := testutil.Context(t)
	waker, err := newSchedulerSessionWaker(ctx, sessions, discardLogger())
	if err != nil {
		t.Fatalf("newSchedulerSessionWaker() error = %v", err)
	}
	t.Cleanup(func() {
		if err := waker.shutdown(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("scheduler waker shutdown error = %v", err)
		}
	})
	runtime, err := newSchedulerRuntime(ctx, tasks, sessions, nil, waker, starvationSpawner{},
		schedulerpkg.StarvationThresholds{
			FanOutAfter: 1, SpawnAfter: 1, EventAfter: 3, NeedsAttentionAfter: 4, MinQueuedAge: time.Second,
		}, discardLogger(), schedulerpkg.WithClock(clockwork.NewFakeClockAt(queuedAt.Add(time.Minute))),
	)
	if err != nil {
		t.Fatalf("newSchedulerRuntime() error = %v", err)
	}
	return runtime.scheduler
}

func assertRecoveredRunClaim(
	t *testing.T, manager *task.Service, run task.Run, owner *session.Info, actor task.ActorContext,
) {
	t.Helper()
	claim := func(sessionID string) (*task.ClaimResult, error) {
		claimant := actor
		claimant.Actor = task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: sessionID}
		claimant.Scope = task.CallerScope{SessionID: sessionID, WorkspaceID: owner.WorkspaceID}
		return manager.ClaimNextRun(testutil.Context(t), task.ClaimCriteria{
			RunID: run.ID, AgentName: owner.AgentName, WorkspaceID: owner.WorkspaceID,
		}, claimant)
	}
	if _, err := claim("sess-same-agent"); !errors.Is(err, task.ErrNoClaimableRun) {
		t.Fatalf("ClaimNextRun(same agent, different session) error = %v, want no claimable run", err)
	}
	result, err := claim(owner.ID)
	if err != nil || result == nil || result.Run.ID != run.ID || result.Run.SessionID != owner.ID {
		t.Fatalf("ClaimNextRun(owner) error = %v or continuation is not bound to the owner session", err)
	}
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
