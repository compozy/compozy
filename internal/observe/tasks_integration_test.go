//go:build integration

package observe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

type observeTaskClock struct {
	current time.Time
}

func (c *observeTaskClock) Now() time.Time {
	return c.current
}

func (c *observeTaskClock) Advance(duration time.Duration) {
	c.current = c.current.Add(duration)
}

type observeSessionExecutor struct {
	nextSessionID    string
	startCalls       []taskpkg.StartTaskSession
	attachCalls      []string
	requestStopCalls []taskStopCall
	forceStopCalls   []taskStopCall
}

type taskStopCall struct {
	SessionID string
	Reason    taskpkg.StopReason
}

func seedNonLeasedClaimedObserveRun(
	t *testing.T,
	h *harness,
	runID string,
	actor taskpkg.ActorContext,
	claimedAt time.Time,
) {
	t.Helper()

	ctx := testutil.Context(t)
	run, err := h.registry.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
	}
	run.Status = taskpkg.TaskRunStatusClaimed
	run.ClaimedBy = &taskpkg.ActorIdentity{Kind: actor.Actor.Kind, Ref: actor.Actor.Ref}
	run.ClaimedAt = claimedAt.UTC()
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	if err := h.registry.UpdateTaskRun(ctx, run); err != nil {
		t.Fatalf("UpdateTaskRun(%q) error = %v", runID, err)
	}
}

type forbidCountDependenciesRegistry struct {
	Registry
}

func (r *forbidCountDependenciesRegistry) CountDependencies(_ context.Context, taskID string) (int, error) {
	return 0, fmt.Errorf("CountDependencies(%q) should not be called", taskID)
}

func (e *observeSessionExecutor) StartTaskSession(
	_ context.Context,
	spec *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	if spec == nil {
		return nil, taskpkg.ErrValidation
	}
	e.startCalls = append(e.startCalls, *spec)
	if e.nextSessionID == "" {
		e.nextSessionID = "sess-observe-1"
	}
	return &taskpkg.SessionRef{SessionID: e.nextSessionID, StartedAt: spec.Run.StartedAt}, nil
}

func (e *observeSessionExecutor) AttachTaskSession(
	_ context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	e.attachCalls = append(e.attachCalls, sessionID)
	return &taskpkg.SessionRef{SessionID: sessionID}, nil
}

func (e *observeSessionExecutor) RequestTaskStop(_ context.Context, sessionID string, reason taskpkg.StopReason) error {
	e.requestStopCalls = append(e.requestStopCalls, taskStopCall{SessionID: sessionID, Reason: reason})
	return nil
}

func (e *observeSessionExecutor) ForceTaskStop(_ context.Context, sessionID string, reason taskpkg.StopReason) error {
	e.forceStopCalls = append(e.forceStopCalls, taskStopCall{SessionID: sessionID, Reason: reason})
	return nil
}

func TestObserveTaskLifecycleSummaryAndMetrics(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	clock := &observeTaskClock{current: h.now.Add(30 * time.Minute)}
	executor := &observeSessionExecutor{nextSessionID: "sess-observe-lifecycle"}
	manager := newObserveTaskManager(t, h, executor, clock)

	networkActor, err := taskpkg.DeriveNetworkPeerActorContext("peer-build", "peer:peer-build/channel:engineering")
	if err != nil {
		t.Fatalf("DeriveNetworkPeerActorContext() error = %v", err)
	}
	daemonActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	created, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Implement observe lifecycle coverage",
	}, networkActor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	clock.Advance(2 * time.Minute)
	run, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
		TaskID:               created.ID,
		IdempotencyKey:       "idem-observe-1",
		NetworkParticipation: observeNamedParticipation("engineering"),
	}, networkActor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}

	clock.Advance(2 * time.Minute)
	seedNonLeasedClaimedObserveRun(t, h, run.ID, daemonActor, clock.Now())

	clock.Advance(3 * time.Minute)
	if _, err := manager.StartRun(
		testutil.Context(t),
		run.ID,
		taskpkg.StartRun{IdempotencyKey: "start-observe-1"},
		daemonActor,
	); err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	clock.Advance(4 * time.Minute)
	if _, err := manager.CompleteRun(testutil.Context(t), run.ID, taskpkg.RunResult{}, daemonActor); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}

	summary, err := h.observer.QueryTaskSummary(testutil.Context(t), TaskSummaryQuery{})
	if err != nil {
		t.Fatalf("QueryTaskSummary() error = %v", err)
	}
	if !containsTaskTotal(summary.TaskTotals, taskpkg.ScopeWorkspace, taskpkg.TaskStatusCompleted, "engineering", 1) {
		t.Fatalf("summary.TaskTotals = %#v, want workspace/completed/engineering count 1", summary.TaskTotals)
	}
	if !containsTaskOriginTotal(summary.TaskOrigins, taskpkg.OriginKindNetwork, "engineering", 1) {
		t.Fatalf("summary.TaskOrigins = %#v, want network/engineering count 1", summary.TaskOrigins)
	}
	if !containsRunTotal(
		summary.RunTotals,
		taskpkg.TaskRunStatusCompleted,
		taskpkg.OriginKindNetwork,
		"engineering",
		1,
	) {
		t.Fatalf("summary.RunTotals = %#v, want completed/network/engineering count 1", summary.RunTotals)
	}

	metrics, err := h.observer.QueryTaskMetrics(testutil.Context(t), TaskMetricsQuery{})
	if err != nil {
		t.Fatalf("QueryTaskMetrics() error = %v", err)
	}
	if !containsRunTotal(
		metrics.TaskRunsTotal,
		taskpkg.TaskRunStatusCompleted,
		taskpkg.OriginKindNetwork,
		"engineering",
		1,
	) {
		t.Fatalf("metrics.TaskRunsTotal = %#v, want completed/network/engineering count 1", metrics.TaskRunsTotal)
	}
	if got, want := metrics.TaskClaimLatencyMillis.Samples, 1; got != want {
		t.Fatalf("metrics.TaskClaimLatencyMillis.Samples = %d, want %d", got, want)
	}
	if got, want := metrics.TaskClaimLatencyMillis.AverageMillis, int64((2 * time.Minute).Milliseconds()); got != want {
		t.Fatalf("metrics.TaskClaimLatencyMillis.AverageMillis = %d, want %d", got, want)
	}
	if got, want := metrics.TaskStartLatencyMillis.AverageMillis, int64((3 * time.Minute).Milliseconds()); got != want {
		t.Fatalf("metrics.TaskStartLatencyMillis.AverageMillis = %d, want %d", got, want)
	}
}

func TestObserveHealthReflectsRecoveryAndForcedStopOutcomes(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	clock := &observeTaskClock{current: h.now.Add(45 * time.Minute)}
	executor := &observeSessionExecutor{nextSessionID: "sess-observe-cancel"}
	manager := newObserveTaskManager(t, h, executor, clock)

	humanActor, err := taskpkg.DeriveHumanActorContext("user-ops", taskpkg.OriginKindCLI, "agh task")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	daemonActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	recoveryActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(boot) error = %v", err)
	}

	cancelTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Cancel running work",
	}, humanActor)
	if err != nil {
		t.Fatalf("CreateTask(cancelTask) error = %v", err)
	}
	recoveryTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Recover orphaned run",
		MaxAttempts: intPtr(1),
	}, humanActor)
	if err != nil {
		t.Fatalf("CreateTask(recoveryTask) error = %v", err)
	}

	clock.Advance(time.Minute)
	runningRun, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{TaskID: cancelTask.ID}, humanActor)
	if err != nil {
		t.Fatalf("EnqueueRun(running) error = %v", err)
	}
	clock.Advance(time.Minute)
	seedNonLeasedClaimedObserveRun(t, h, runningRun.ID, daemonActor, clock.Now())
	clock.Advance(time.Minute)
	if _, err := manager.StartRun(
		testutil.Context(t),
		runningRun.ID,
		taskpkg.StartRun{IdempotencyKey: "start-cancel-1"},
		daemonActor,
	); err != nil {
		t.Fatalf("StartRun(running) error = %v", err)
	}
	clock.Advance(time.Minute)
	if _, err := manager.CancelTask(
		testutil.Context(t),
		cancelTask.ID,
		taskpkg.CancelTask{Reason: "shutdown"},
		humanActor,
	); err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}

	executor.nextSessionID = "sess-observe-attach"
	clock.Advance(time.Minute)
	recoveredRun, err := manager.EnqueueRun(
		testutil.Context(t),
		taskpkg.EnqueueRun{TaskID: recoveryTask.ID},
		humanActor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(recovery) error = %v", err)
	}
	clock.Advance(time.Minute)
	seedNonLeasedClaimedObserveRun(t, h, recoveredRun.ID, daemonActor, clock.Now())
	clock.Advance(time.Minute)
	if _, err := manager.AttachRunSession(
		testutil.Context(t),
		recoveredRun.ID,
		"sess-missing-on-boot",
		daemonActor,
	); err != nil {
		t.Fatalf("AttachRunSession() error = %v", err)
	}
	clock.Advance(time.Minute)
	if _, err := manager.RecoverRunOnBoot(testutil.Context(t), recoveredRun.ID, taskpkg.RunBootRecovery{
		Action:       taskpkg.RunBootRecoveryFail,
		Reason:       "orphaned_on_boot",
		SessionState: "missing",
	}, recoveryActor); err != nil {
		t.Fatalf("RecoverRunOnBoot() error = %v", err)
	}

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if got, want := len(executor.forceStopCalls), 2; got != want {
		t.Fatalf("len(forceStopCalls) = %d, want %d", got, want)
	}
	if got, want := executor.forceStopCalls[0], (taskStopCall{
		SessionID: "sess-observe-cancel",
		Reason:    taskpkg.StopReasonCancellation,
	}); got != want {
		t.Fatalf("forceStopCalls[0] = %#v, want %#v", got, want)
	}
	if got, want := executor.forceStopCalls[1], (taskStopCall{
		SessionID: "sess-missing-on-boot",
		Reason:    taskpkg.StopReasonFailed,
	}); got != want {
		t.Fatalf("forceStopCalls[1] = %#v, want %#v", got, want)
	}
	if got, want := health.Tasks.ForcedStopsSinceStart, 1; got != want {
		t.Fatalf("health.Tasks.ForcedStopsSinceStart = %d, want %d", got, want)
	}
	if got, want := health.Tasks.RecoverySinceStart.Failed, 1; got != want {
		t.Fatalf("health.Tasks.RecoverySinceStart.Failed = %d, want %d", got, want)
	}
	if !containsTaskTotal(health.Tasks.TaskTotals, taskpkg.ScopeWorkspace, taskpkg.TaskStatusCanceled, "", 1) {
		t.Fatalf("health.Tasks.TaskTotals = %#v, want cancelled task count 1", health.Tasks.TaskTotals)
	}
	if !containsTaskTotal(health.Tasks.TaskTotals, taskpkg.ScopeWorkspace, taskpkg.TaskStatusFailed, "", 1) {
		t.Fatalf("health.Tasks.TaskTotals = %#v, want failed task count 1", health.Tasks.TaskTotals)
	}
	if !containsRunTotal(health.Tasks.RunTotals, taskpkg.TaskRunStatusCanceled, taskpkg.OriginKindCLI, "", 1) {
		t.Fatalf("health.Tasks.RunTotals = %#v, want cancelled/cli run count 1", health.Tasks.RunTotals)
	}
	if !containsRunTotal(health.Tasks.RunTotals, taskpkg.TaskRunStatusFailed, taskpkg.OriginKindCLI, "", 1) {
		t.Fatalf("health.Tasks.RunTotals = %#v, want failed/cli run count 1", health.Tasks.RunTotals)
	}
}

func TestObserveTaskDashboardRejectsInvalidScopeWorkspaceBinding(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{
		Scope:       taskpkg.ScopeGlobal,
		WorkspaceID: h.workspaceID,
	})
	if !errors.Is(err, taskpkg.ErrInvalidScopeBinding) {
		t.Fatalf("QueryTaskDashboard() error = %v, want %v", err, taskpkg.ErrInvalidScopeBinding)
	}
}

func TestObserveTaskDashboardConfigLayersPartialOverrides(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	WithTaskDashboardConfig(TaskDashboardConfig{
		ActiveRunLimit:   6,
		BacklogWarnAfter: 20 * time.Minute,
	})(h.observer)
	WithTaskDashboardConfig(TaskDashboardConfig{
		StaleAfter: 45 * time.Minute,
	})(h.observer)

	if got, want := h.observer.taskDashboardConfig.activeRunLimit, 6; got != want {
		t.Fatalf("taskDashboardConfig.activeRunLimit = %d, want %d", got, want)
	}
	if got, want := h.observer.taskDashboardConfig.backlogWarnAfter, 20*time.Minute; got != want {
		t.Fatalf("taskDashboardConfig.backlogWarnAfter = %v, want %v", got, want)
	}
	if got, want := h.observer.taskDashboardConfig.staleAfter, 45*time.Minute; got != want {
		t.Fatalf("taskDashboardConfig.staleAfter = %v, want %v", got, want)
	}
}

func TestObserveTaskDashboardLoadsDependencyCountsWithoutPerTaskCalls(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.registry = &forbidCountDependenciesRegistry{Registry: h.registry}

	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-dependent",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Dependency blocked task",
		Status:      taskpkg.TaskStatusBlocked,
		CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
		Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		CreatedAt:   h.now,
		UpdatedAt:   h.now,
	})
	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-prerequisite",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Prerequisite task",
		Status:      taskpkg.TaskStatusReady,
		CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
		Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		CreatedAt:   h.now.Add(time.Minute),
		UpdatedAt:   h.now.Add(time.Minute),
	})
	createObserveDependency(t, h, taskpkg.Dependency{
		TaskID:          "task-dependent",
		DependsOnTaskID: "task-prerequisite",
		Kind:            taskpkg.DependencyKindBlocks,
		CreatedAt:       h.now.Add(2 * time.Minute),
	})

	dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
	if err != nil {
		t.Fatalf("QueryTaskDashboard() error = %v", err)
	}
	if got, want := dashboard.Totals.DependencyBlockedTasks, 1; got != want {
		t.Fatalf("dashboard.Totals.DependencyBlockedTasks = %d, want %d", got, want)
	}
}

func TestObserveTaskDashboardAggregatesPersistedLifecycleState(t *testing.T) {
	t.Parallel()

	t.Run("Should aggregate persisted queued running failed and completed lifecycle state", func(t *testing.T) {

		h := newHarness(t)
		clock := &observeTaskClock{current: h.now.Add(30 * time.Minute)}
		executor := &observeSessionExecutor{nextSessionID: "sess-observe-dashboard"}
		manager := newObserveTaskManager(t, h, executor, clock)
		h.observer.now = clock.Now
		WithTaskDashboardConfig(TaskDashboardConfig{BacklogWarnAfter: 20 * time.Minute})(h.observer)

		humanActor, err := taskpkg.DeriveHumanActorContext("user-ops", taskpkg.OriginKindCLI, "agh task")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		queuedTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Queued task",
		}, humanActor)
		if err != nil {
			t.Fatalf("CreateTask(queuedTask) error = %v", err)
		}
		if _, err = manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Approval gate",
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		}, humanActor); err != nil {
			t.Fatalf("CreateTask(blockedTask) error = %v", err)
		}
		runningTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Running task",
		}, humanActor)
		if err != nil {
			t.Fatalf("CreateTask(runningTask) error = %v", err)
		}
		failedTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Failed task",
			MaxAttempts: intPtr(1),
		}, humanActor)
		if err != nil {
			t.Fatalf("CreateTask(failedTask) error = %v", err)
		}
		completedTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Completed task",
		}, humanActor)
		if err != nil {
			t.Fatalf("CreateTask(completedTask) error = %v", err)
		}

		clock.Advance(time.Minute)
		queuedRun, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
			TaskID:               queuedTask.ID,
			NetworkParticipation: observeNamedParticipation("ops"),
		}, humanActor)
		if err != nil {
			t.Fatalf("EnqueueRun(queuedTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		runningRun, err := manager.EnqueueRun(
			testutil.Context(t),
			taskpkg.EnqueueRun{
				TaskID:               runningTask.ID,
				NetworkParticipation: observeNamedParticipation("eng"),
			},
			humanActor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(runningTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		seedNonLeasedClaimedObserveRun(t, h, runningRun.ID, daemonActor, clock.Now())
		clock.Advance(time.Minute)
		if _, err := manager.StartRun(
			testutil.Context(t),
			runningRun.ID,
			taskpkg.StartRun{IdempotencyKey: "start-running-1"},
			daemonActor,
		); err != nil {
			t.Fatalf("StartRun(runningTask) error = %v", err)
		}

		clock.Advance(time.Minute)
		failedRun, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
			TaskID:               failedTask.ID,
			NetworkParticipation: observeNamedParticipation("ops"),
		}, humanActor)
		if err != nil {
			t.Fatalf("EnqueueRun(failedTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		seedNonLeasedClaimedObserveRun(t, h, failedRun.ID, daemonActor, clock.Now())
		clock.Advance(time.Minute)
		if _, err := manager.StartRun(
			testutil.Context(t),
			failedRun.ID,
			taskpkg.StartRun{IdempotencyKey: "start-failed-1"},
			daemonActor,
		); err != nil {
			t.Fatalf("StartRun(failedTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.FailRun(
			testutil.Context(t),
			failedRun.ID,
			taskpkg.RunFailure{Error: "boom"},
			daemonActor,
		); err != nil {
			t.Fatalf("FailRun(failedTask) error = %v", err)
		}

		clock.Advance(time.Minute)
		completedRun, err := manager.EnqueueRun(
			testutil.Context(t),
			taskpkg.EnqueueRun{
				TaskID:               completedTask.ID,
				NetworkParticipation: observeNamedParticipation("eng"),
			},
			humanActor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(completedTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		seedNonLeasedClaimedObserveRun(t, h, completedRun.ID, daemonActor, clock.Now())
		clock.Advance(time.Minute)
		if _, err := manager.StartRun(
			testutil.Context(t),
			completedRun.ID,
			taskpkg.StartRun{IdempotencyKey: "start-completed-1"},
			daemonActor,
		); err != nil {
			t.Fatalf("StartRun(completedTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.CompleteRun(
			testutil.Context(t),
			completedRun.ID,
			taskpkg.RunResult{},
			daemonActor,
		); err != nil {
			t.Fatalf("CompleteRun(completedTask) error = %v", err)
		}

		dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard() error = %v", err)
		}

		if got, want := dashboard.Totals.TasksTotal, 5; got != want {
			t.Fatalf("dashboard.Totals.TasksTotal = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.BlockedTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.BlockedTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.AwaitingApprovalTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.AwaitingApprovalTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.ReadyTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.ReadyTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.InProgressTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.InProgressTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.FailedTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.FailedTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.CompletedTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.CompletedTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Queue.Total, 1; got != want {
			t.Fatalf("dashboard.Queue.Total = %d, want %d", got, want)
		}
		if got, want := dashboard.ActiveRuns.Total, 2; got != want {
			t.Fatalf("dashboard.ActiveRuns.Total = %d, want %d", got, want)
		}
		if got := activeRunIDs(
			dashboard.ActiveRuns.Items,
		); len(got) < 2 || got[0] != runningRun.ID ||
			got[1] != queuedRun.ID {
			t.Fatalf("dashboard.ActiveRuns.Items ids = %#v, want running then queued", got)
		}
	})
}

func TestObserveTaskDashboardRefreshesAfterPersistedTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh dashboard backlog and freshness after persisted run transitions", func(t *testing.T) {

		h := newHarness(t)
		clock := &observeTaskClock{current: h.now.Add(30 * time.Minute)}
		executor := &observeSessionExecutor{nextSessionID: "sess-observe-refresh"}
		manager := newObserveTaskManager(t, h, executor, clock)
		h.observer.now = clock.Now
		WithTaskDashboardConfig(TaskDashboardConfig{BacklogWarnAfter: 5 * time.Minute})(h.observer)

		humanActor, err := taskpkg.DeriveHumanActorContext("user-ops", taskpkg.OriginKindCLI, "agh task")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Transitioning task",
		}, humanActor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		clock.Advance(time.Minute)
		run, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
			TaskID:               taskRecord.ID,
			NetworkParticipation: observeNamedParticipation("ops"),
		}, humanActor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		clock.Advance(6 * time.Minute)

		queuedDashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard(queued) error = %v", err)
		}
		if got, want := queuedDashboard.Queue.Total, 1; got != want {
			t.Fatalf("queuedDashboard.Queue.Total = %d, want %d", got, want)
		}
		if !queuedDashboard.Queue.BacklogWarning {
			t.Fatal("queuedDashboard.Queue.BacklogWarning = false, want true")
		}
		if got, want := queuedDashboard.Freshness.Status, "stale"; got != want {
			t.Fatalf("queuedDashboard.Freshness.Status = %q, want %q", got, want)
		}

		seedNonLeasedClaimedObserveRun(t, h, run.ID, daemonActor, clock.Now())
		clock.Advance(time.Minute)
		if _, err := manager.StartRun(
			testutil.Context(t),
			run.ID,
			taskpkg.StartRun{IdempotencyKey: "start-transition-1"},
			daemonActor,
		); err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}

		runningDashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard(running) error = %v", err)
		}
		if got, want := runningDashboard.Queue.Total, 0; got != want {
			t.Fatalf("runningDashboard.Queue.Total = %d, want %d", got, want)
		}
		if got, want := runningDashboard.Totals.RunningRuns, 1; got != want {
			t.Fatalf("runningDashboard.Totals.RunningRuns = %d, want %d", got, want)
		}
		if got, want := runningDashboard.Totals.InProgressTasks, 1; got != want {
			t.Fatalf("runningDashboard.Totals.InProgressTasks = %d, want %d", got, want)
		}
		if got, want := runningDashboard.Freshness.Status, "current"; got != want {
			t.Fatalf("runningDashboard.Freshness.Status = %q, want %q", got, want)
		}

		clock.Advance(time.Minute)
		if _, err := manager.CompleteRun(testutil.Context(t), run.ID, taskpkg.RunResult{}, daemonActor); err != nil {
			t.Fatalf("CompleteRun() error = %v", err)
		}

		completedDashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard(completed) error = %v", err)
		}
		if got, want := completedDashboard.ActiveRuns.Total, 0; got != want {
			t.Fatalf("completedDashboard.ActiveRuns.Total = %d, want %d", got, want)
		}
		if got, want := completedDashboard.Totals.CompletedTasks, 1; got != want {
			t.Fatalf("completedDashboard.Totals.CompletedTasks = %d, want %d", got, want)
		}
		if got, want := completedDashboard.Totals.CompletedRuns, 1; got != want {
			t.Fatalf("completedDashboard.Totals.CompletedRuns = %d, want %d", got, want)
		}
		if completedDashboard.Queue.BacklogWarning {
			t.Fatal("completedDashboard.Queue.BacklogWarning = true, want false")
		}
	})
}

func TestObserveTaskInboxReflectsApprovalAndTriageTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should reflect approval triage archive and dismissal transitions in the inbox", func(t *testing.T) {

		h := newHarness(t)
		clock := &observeTaskClock{current: h.now.Add(30 * time.Minute)}
		executor := &observeSessionExecutor{nextSessionID: "sess-observe-inbox"}
		manager := newObserveTaskManager(t, h, executor, clock)
		h.observer.now = clock.Now

		alice, err := taskpkg.DeriveHumanActorContext("alice", taskpkg.OriginKindCLI, "agh task inbox")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(alice) error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		myWork, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "My work",
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
		}, alice)
		if err != nil {
			t.Fatalf("CreateTask(myWork) error = %v", err)
		}
		clock.Advance(time.Minute)
		approveTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Approve me",
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
		}, alice)
		if err != nil {
			t.Fatalf("CreateTask(approveTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		rejectTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Reject me",
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
		}, alice)
		if err != nil {
			t.Fatalf("CreateTask(rejectTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		archiveTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Archive me",
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
		}, alice)
		if err != nil {
			t.Fatalf("CreateTask(archiveTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		failTask, err := manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Fail me",
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
			MaxAttempts: intPtr(1),
		}, alice)
		if err != nil {
			t.Fatalf("CreateTask(failTask) error = %v", err)
		}

		clock.Advance(time.Minute)
		failRun, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{TaskID: failTask.ID}, alice)
		if err != nil {
			t.Fatalf("EnqueueRun(failTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		seedNonLeasedClaimedObserveRun(t, h, failRun.ID, daemonActor, clock.Now())
		clock.Advance(time.Minute)
		if _, err := manager.StartRun(
			testutil.Context(t),
			failRun.ID,
			taskpkg.StartRun{IdempotencyKey: "start-inbox-fail-1"},
			daemonActor,
		); err != nil {
			t.Fatalf("StartRun(failTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.FailRun(
			testutil.Context(t),
			failRun.ID,
			taskpkg.RunFailure{Error: "boom"},
			daemonActor,
		); err != nil {
			t.Fatalf("FailRun(failTask) error = %v", err)
		}

		initial, err := h.observer.QueryTaskInbox(testutil.Context(t), TaskInboxQuery{}, alice.Actor)
		if err != nil {
			t.Fatalf("QueryTaskInbox(initial) error = %v", err)
		}
		if got, want := initial.Total, 5; got != want {
			t.Fatalf("initial.Total = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, initial.Groups, TaskInboxLaneApprovals).Count, 2; got != want {
			t.Fatalf("initial approvals count = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, initial.Groups, TaskInboxLaneMyWork).Count, 2; got != want {
			t.Fatalf("initial my_work count = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, initial.Groups, TaskInboxLaneFailedRuns).Count, 1; got != want {
			t.Fatalf("initial failed count = %d, want %d", got, want)
		}

		if _, err := manager.MarkTaskRead(testutil.Context(t), myWork.ID, alice); err != nil {
			t.Fatalf("MarkTaskRead(myWork) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.ArchiveTask(testutil.Context(t), archiveTask.ID, alice); err != nil {
			t.Fatalf("ArchiveTask(archiveTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.DismissTask(testutil.Context(t), failTask.ID, alice); err != nil {
			t.Fatalf("DismissTask(failTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.ApproveTask(
			testutil.Context(t),
			approveTask.ID,
			taskpkg.ExecutionRequest{},
			alice,
		); err != nil {
			t.Fatalf("ApproveTask(approveTask) error = %v", err)
		}
		clock.Advance(time.Minute)
		if _, err := manager.RejectTask(testutil.Context(t), rejectTask.ID, alice); err != nil {
			t.Fatalf("RejectTask(rejectTask) error = %v", err)
		}

		updated, err := h.observer.QueryTaskInbox(testutil.Context(t), TaskInboxQuery{}, alice.Actor)
		if err != nil {
			t.Fatalf("QueryTaskInbox(updated) error = %v", err)
		}
		if got, want := updated.Total, 4; got != want {
			t.Fatalf("updated.Total = %d, want %d", got, want)
		}
		if got, want := updated.UnreadTotal, 2; got != want {
			t.Fatalf("updated.UnreadTotal = %d, want %d", got, want)
		}
		if got, want := updated.ArchivedTotal, 1; got != want {
			t.Fatalf("updated.ArchivedTotal = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, updated.Groups, TaskInboxLaneApprovals).Count, 0; got != want {
			t.Fatalf("updated approvals count = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, updated.Groups, TaskInboxLaneFailedRuns).Count, 0; got != want {
			t.Fatalf("updated failed count = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, updated.Groups, TaskInboxLaneArchived).Count, 1; got != want {
			t.Fatalf("updated archived count = %d, want %d", got, want)
		}
		blocked := requireInboxGroup(t, updated.Groups, TaskInboxLaneBlocked)
		if got, want := blocked.Count, 1; got != want {
			t.Fatalf("updated blocked count = %d, want %d", got, want)
		}
		if got, want := blocked.Items[0].BlockingReason, taskInboxBlockingReasonApprovalRejected; got != want {
			t.Fatalf("blocked reason = %q, want %q", got, want)
		}
		myWorkGroup := requireInboxGroup(t, updated.Groups, TaskInboxLaneMyWork)
		if got, want := myWorkGroup.Count, 2; got != want {
			t.Fatalf("updated my_work count = %d, want %d", got, want)
		}
		if got, want := inboxItemTaskIDs(
			myWorkGroup.Items,
		), []string{
			approveTask.ID,
			myWork.ID,
		}; !equalStringSlices(
			got,
			want,
		) {
			t.Fatalf("updated my_work ids = %#v, want %#v", got, want)
		}

		unread := true
		unreadOnly, err := h.observer.QueryTaskInbox(
			testutil.Context(t),
			TaskInboxQuery{Unread: &unread},
			alice.Actor,
		)
		if err != nil {
			t.Fatalf("QueryTaskInbox(unread) error = %v", err)
		}
		if got, want := unreadOnly.Total, 2; got != want {
			t.Fatalf("unreadOnly.Total = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, unreadOnly.Groups, TaskInboxLaneMyWork).Count, 1; got != want {
			t.Fatalf("unread my_work count = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, unreadOnly.Groups, TaskInboxLaneBlocked).Count, 1; got != want {
			t.Fatalf("unread blocked count = %d, want %d", got, want)
		}
	})
}

func newObserveTaskManager(
	t *testing.T,
	h *harness,
	executor *observeSessionExecutor,
	clock *observeTaskClock,
) *taskpkg.Service {
	t.Helper()

	sequence := 0
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(h.registry),
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithManagerNow(clock.Now),
		taskpkg.WithIDGenerator(func(prefix string) string {
			sequence++
			return prefix + "-observe-" + strconv.FormatInt(clock.current.UnixNano(), 10) + "-" + strconv.Itoa(sequence)
		}),
		taskpkg.WithCancelGracePeriod(0),
		taskpkg.WithParticipationResolver(observeParticipationResolver{}),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	return manager
}

type observeParticipationResolver struct{}

func (observeParticipationResolver) Resolve(
	_ context.Context,
	input participation.ResolveInput,
) (participation.Spec, error) {
	if input.Request == nil || input.Request.Mode == nil || *input.Request.Mode == participation.ModeLocal {
		return participation.LocalSpec(), nil
	}
	bounds, err := participation.ResolveBounds(
		input.Request.Bounds,
		observeParticipationDefaults(),
		participation.Limits{},
	)
	if err != nil {
		return participation.Spec{}, err
	}
	source := input.RequestSource
	if source == "" {
		source = participation.SourceExplicitRequest
	}
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     input.WorkspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(*input.Request.ChannelID),
		Source:          source,
		Bounds:          bounds,
	}, nil
}

func observeParticipationDefaults() participation.Bounds {
	return participation.Bounds{
		MaxWakes:         4,
		MaxWakeWallTime:  "30s",
		MaxTotalWallTime: "2m",
		MaxInputTokens:   4096,
		MaxOutputTokens:  4096,
		MaxWakeDepth:     4,
		CoalesceWindow:   "250ms",
	}
}

func observeNamedParticipation(channel string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	channel = strings.TrimSpace(channel)
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channel,
	}
}

func intPtr(value int) *int {
	return &value
}
