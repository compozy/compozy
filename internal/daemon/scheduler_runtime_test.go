package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	contractpkg "github.com/compozy/compozy/internal/api/contract"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	watchpkg "github.com/compozy/compozy/internal/loop/watch"
	schedulerpkg "github.com/compozy/compozy/internal/scheduler"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type schedulerSituationContextStub struct {
	payload contractpkg.AgentContextPayload
	err     error
}

func (s *schedulerSituationContextStub) ContextForSession(
	context.Context,
	*session.Info,
) (contractpkg.AgentContextPayload, error) {
	return s.payload, s.err
}

func TestSchedulerSessionSourcePreservesCapabilityCertainty(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a known empty capability projection", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{infos: []*session.Info{{
			ID:          "sess-known-empty",
			AgentName:   "frontend-agent",
			WorkspaceID: "ws-1",
			State:       session.StateActive,
		}}}
		source := schedulerSessionSource{
			sessions:  sessions,
			situation: &schedulerSituationContextStub{},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		snapshots, err := source.Sessions(testutil.Context(t))
		if err != nil {
			t.Fatalf("Sessions() error = %v", err)
		}
		if got, want := len(snapshots), 1; got != want {
			t.Fatalf("len(Sessions()) = %d, want %d", got, want)
		}
		if snapshots[0].CapabilityState != schedulerpkg.CapabilityStateKnown {
			t.Fatalf("CapabilityState = %q, want %q", snapshots[0].CapabilityState, schedulerpkg.CapabilityStateKnown)
		}
		if len(snapshots[0].Capabilities) != 0 {
			t.Fatalf("Capabilities = %v, want known empty projection", snapshots[0].Capabilities)
		}
	})

	t.Run("Should preserve an indeterminate session when capability projection fails", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		sessions := &fakeSessionManager{infos: []*session.Info{{
			ID:          "sess-indeterminate",
			AgentName:   "frontend-agent",
			WorkspaceID: "ws-1",
			State:       session.StateActive,
		}}}
		source := schedulerSessionSource{
			sessions:  sessions,
			situation: &schedulerSituationContextStub{err: context.DeadlineExceeded},
			logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		}

		snapshots, err := source.Sessions(testutil.Context(t))
		if err != nil {
			t.Fatalf("Sessions() error = %v", err)
		}
		if got, want := len(snapshots), 1; got != want {
			t.Fatalf("len(Sessions()) = %d, want %d", got, want)
		}
		if snapshots[0].CapabilityState != schedulerpkg.CapabilityStateUnknown {
			t.Fatalf(
				"CapabilityState = %q, want %q",
				snapshots[0].CapabilityState,
				schedulerpkg.CapabilityStateUnknown,
			)
		}
		logOutput := logs.String()
		if !strings.Contains(logOutput, "scheduler.session_context.error") ||
			!strings.Contains(logOutput, "sess-indeterminate") {
			t.Fatalf("scheduler projection log = %q, want event and session id", logOutput)
		}
	})
}

func TestSchedulerTaskSourcePendingRunsShouldExposeOnlyTaskAnchoredGenericWorkers(t *testing.T) {
	t.Parallel()

	t.Run("Should expose only generic queued worker runs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 5, 19, 0, 0, 0, time.UTC)
		db := openDaemonTestGlobalDB(t)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        "ws-scheduler",
			RootDir:   t.TempDir(),
			Name:      "Scheduler",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace(ws-scheduler) error = %v", err)
		}
		if err := db.RegisterSession(ctx, store.SessionInfo{
			ProfileID: store.DefaultProfileID, ID: "sess-1", AgentName: "coder", Provider: "test",
			WorkspaceID: "ws-scheduler", State: "active", CreatedAt: now, UpdatedAt: now,
			RuntimeStatus: store.SessionRuntimeUnbound,
		}); err != nil {
			t.Fatalf("RegisterSession(network wake target) error = %v", err)
		}
		normalTask := daemonTaskRecordForTest("task-normal-worker", now)
		if err := db.CreateTask(ctx, normalTask); err != nil {
			t.Fatalf("CreateTask(normal) error = %v", err)
		}
		loopTask := daemonTaskRecordForTest("task-loop-worker", now)
		if err := db.CreateTask(ctx, loopTask); err != nil {
			t.Fatalf("CreateTask(loop) error = %v", err)
		}
		seedRun := looppkg.Run{
			ID:                "looprun-scheduler-hidden",
			WorkspaceID:       "ws-scheduler",
			LoopName:          "software-delivery",
			Status:            looppkg.StatusRunning,
			ReattemptStrategy: looppkg.ReattemptFailedOnly,
			IterationCap:      50,
			BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
			CreatedAt:         now,
			LastProgressAt:    now,
		}
		applyLoopRunPinningForTest(t, &seedRun, now)
		if _, err := db.CreateLoopRunForStart(ctx, seedRun, loopdsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		enqueueSchedulerRunForTest(
			t,
			db,
			now,
			normalTask.ID,
			"run-normal-worker",
			taskpkg.RunKindWorker,
			"",
			"scheduler.normal",
		)
		enqueueSchedulerRunForTest(
			t,
			db,
			now,
			loopTask.ID,
			"run-loop-action",
			taskpkg.RunKindWorker,
			"looprun-scheduler-hidden",
			"scheduler.loop-action",
		)
		seedSchedulerNetworkWakeForTest(
			t,
			db,
			now,
			"ws-scheduler",
			"run-network-wake",
			"wake-1",
			"sess-1",
		)

		pending, err := (schedulerTaskSource{store: db}).PendingRuns(ctx)
		if err != nil {
			t.Fatalf("PendingRuns() error = %v", err)
		}
		if got, want := len(pending), 1; got != want {
			t.Fatalf("len(PendingRuns()) = %d, want %d", got, want)
		}
		if got, want := pending[0].Run.ID, "run-normal-worker"; got != want {
			t.Fatalf("PendingRuns()[0].Run.ID = %q, want %q", got, want)
		}

		if err := db.RegisterSession(ctx, store.SessionInfo{
			ProfileID: store.DefaultProfileID, ID: "sess-active", AgentName: "coder", Provider: "test",
			WorkspaceID: "ws-scheduler", State: "active", CreatedAt: now, UpdatedAt: now,
			RuntimeStatus: store.SessionRuntimeUnbound,
		}); err != nil {
			t.Fatalf("RegisterSession(active network wake target) error = %v", err)
		}
		seedSchedulerNetworkWakeForTest(
			t,
			db,
			now,
			"ws-scheduler",
			"run-network-wake-active",
			"wake-active",
			"sess-active",
		)
		activeClaim := claimSchedulerRunForTest(t, db, now, taskpkg.ClaimCriteria{
			RunID:            "run-network-wake-active",
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      "ws-scheduler",
			RunKind:          taskpkg.RunKindNetworkWake,
			TargetSessionID:  "sess-active",
			ClaimerSessionID: "sess-active",
			LeaseDuration:    time.Minute,
		})
		activeWake := activeClaim.Run
		active, err := (schedulerTaskSource{store: db}).ActiveRuns(ctx)
		if err != nil {
			t.Fatalf("ActiveRuns() error = %v", err)
		}
		if len(active) != 1 || active[0].ID != activeWake.ID {
			t.Fatalf("ActiveRuns() = %#v, want claimed taskless network wake", active)
		}
	})
}

func TestSchedulerTaskSourceLoopCoordinatorBackstopShouldLimitPerScope(t *testing.T) {
	// This test starts 32 coordinator runs against SQLite; keep it serial so the
	// package's race-heavy parallel tests do not exhaust the default test context.
	t.Run("Should limit coordinator starts per workspace scope", func(t *testing.T) {
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
		db := openDaemonTestGlobalDB(t)
		workspaceIDs := []string{"ws-backstop-alpha", "ws-backstop-beta"}
		for _, workspaceID := range workspaceIDs {
			if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
				ID:        workspaceID,
				RootDir:   t.TempDir(),
				Name:      workspaceID,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				t.Fatalf("InsertWorkspace(%s) error = %v", workspaceID, err)
			}
		}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(db),
			taskpkg.WithCoordinatorRunner(schedulerBackstopCoordinatorRunner{}),
			taskpkg.WithGenerationStateFinalizer(schedulerBackstopGenerationFinalizer{}),
			taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Valid()
			}),
			taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Terminal()
			}),
			taskpkg.WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		for index := range defaultLoopCoordinatorBackstopLimit + 1 {
			seedCoordinatorBackstopRun(t, db, now, "alpha", index, workspaceIDs[0])
			seedCoordinatorBackstopRun(t, db, now, "beta", index, workspaceIDs[1])
		}
		actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		started, err := (schedulerTaskSource{manager: manager, store: db}).RunLoopCoordinatorBackstop(ctx, now, actor)
		if err != nil {
			t.Fatalf("RunLoopCoordinatorBackstop() error = %v", err)
		}
		if got, want := started, defaultLoopCoordinatorBackstopLimit*len(workspaceIDs); got != want {
			t.Fatalf("started = %d, want %d", got, want)
		}
		queued, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
		}
		if got, want := len(queued), len(workspaceIDs); got != want {
			t.Fatalf("queued runs = %d, want one remaining per scope", got)
		}
	})
}

func TestSchedulerTaskSourceLoopCoordinatorBackstopShouldDeferAtWorkspaceCapacity(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 18, 22, 15, 0, 0, time.UTC)
	workspaceID := "ws-backstop-capacity"
	db := openDaemonTestGlobalDB(t)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID:        workspaceID,
		RootDir:   t.TempDir(),
		Name:      workspaceID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	activeTask := daemonTaskRecordForTest("task-backstop-capacity-active", now)
	activeTask.Scope = taskpkg.ScopeWorkspace
	activeTask.WorkspaceID = workspaceID
	if err := db.CreateTask(ctx, activeTask); err != nil {
		t.Fatalf("CreateTask(active) error = %v", err)
	}
	enqueueSchedulerRunForTest(
		t,
		db,
		now,
		activeTask.ID,
		"run-backstop-capacity-active",
		taskpkg.RunKindWorker,
		"",
		"scheduler.capacity.active",
	)
	claimSchedulerRunForTest(t, db, now, taskpkg.ClaimCriteria{
		RunID:            "run-backstop-capacity-active",
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		RunKind:          taskpkg.RunKindWorker,
		ClaimerSessionID: "sess-backstop-capacity",
		LeaseDuration:    time.Minute,
	})
	seedCoordinatorBackstopRun(t, db, now, "capacity", 0, workspaceID)
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithWorkspaceActiveRunCap(1),
		taskpkg.WithManagerNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	started, err := (schedulerTaskSource{manager: manager, store: db}).RunLoopCoordinatorBackstop(ctx, now, actor)
	if err != nil {
		t.Fatalf("RunLoopCoordinatorBackstop() error = %v", err)
	}
	if started != 0 {
		t.Fatalf("started = %d, want 0 while workspace capacity is full", started)
	}
	queued, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
	}
	if len(queued) != 1 || queued[0].RunKind.Normalize() != taskpkg.RunKindCoordinator {
		t.Fatalf("queued runs = %#v, want deferred coordinator", queued)
	}
}

func TestSchedulerTaskSourceLoopCoordinatorBackstopShouldDeferWhileConsumerLeaseIsActive(t *testing.T) {
	t.Parallel()

	t.Run("Should defer while the coordinator consumer lease is active", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 3, 22, 45, 0, 0, time.UTC)
		workspaceID := "ws-backstop-active-consumer"
		db := openDaemonTestGlobalDB(t)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        workspaceID,
			RootDir:   t.TempDir(),
			Name:      workspaceID,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		seedCoordinatorBackstopRun(t, db, now, "active-consumer", 0, workspaceID)
		seedCoordinatorBackstopRun(t, db, now, "active-consumer", 1, workspaceID)
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(db),
			taskpkg.WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		if _, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:         taskpkg.ScopeWorkspace,
			WorkspaceID:   workspaceID,
			RunKind:       taskpkg.RunKindCoordinator,
			ClaimedBy:     &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: loopCoordinatorActorRef},
			LeaseDuration: taskpkg.DefaultRunLeaseDuration,
			Now:           now,
		}, actor); err != nil {
			t.Fatalf("ClaimNextRun(active coordinator) error = %v", err)
		}

		started, err := (schedulerTaskSource{manager: manager, store: db}).RunLoopCoordinatorBackstop(ctx, now, actor)
		if err != nil {
			t.Fatalf("RunLoopCoordinatorBackstop() error = %v", err)
		}
		if started != 0 {
			t.Fatalf("started = %d, want 0 while the coordinator consumer is busy", started)
		}
		queued, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
		}
		if got, want := len(queued), 1; got != want {
			t.Fatalf("queued coordinator runs = %d, want %d deferred run", got, want)
		}
	})
}

func TestSchedulerTaskSourceLoopCoordinatorBackstopShouldRecoverWatchEventsGap(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should enqueue and start a coordinator wake when only the durable watch-events ledger has advanced",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 8, 19, 30, 0, 0, time.UTC)
			db := openDaemonTestGlobalDB(t)
			workspaceID := "ws-backstop-watch-events"
			if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
				ID:        workspaceID,
				RootDir:   t.TempDir(),
				Name:      "Watch Events Backstop",
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				t.Fatalf("InsertWorkspace() error = %v", err)
			}
			created, targetTask := parkSchedulerWatchEventsLoopForTest(ctx, t, db, now, workspaceID)
			appendSchedulerWatchEventForTest(ctx, t, db, targetTask.ID, now.Add(time.Second))
			manager, err := taskpkg.NewManager(
				taskpkg.WithStore(db),
				taskpkg.WithCoordinatorRunner(schedulerBackstopCoordinatorRunner{}),
				taskpkg.WithGenerationStateFinalizer(schedulerBackstopGenerationFinalizer{}),
				taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
					return looppkg.Status(strings.TrimSpace(status)).Valid()
				}),
				taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
					return looppkg.Status(strings.TrimSpace(status)).Terminal()
				}),
				taskpkg.WithManagerNow(func() time.Time { return now.Add(2 * time.Second) }),
			)
			if err != nil {
				t.Fatalf("task.NewManager() error = %v", err)
			}
			actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
			if err != nil {
				t.Fatalf("DeriveDaemonActorContext() error = %v", err)
			}

			started, err := (schedulerTaskSource{
				manager:            manager,
				store:              db,
				watchEventsGapScan: newLoopWatchEventsGapScanState(),
			}).RunLoopCoordinatorBackstop(
				ctx,
				now.Add(2*time.Second),
				actor,
			)
			if err != nil {
				t.Fatalf("RunLoopCoordinatorBackstop() error = %v", err)
			}
			if got, want := started, 1; got != want {
				t.Fatalf("started = %d, want %d", got, want)
			}
			startedRuns, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusCompleted})
			if err != nil {
				t.Fatalf("ListTaskRunsByStatus(completed) error = %v", err)
			}
			if !schedulerCompletedRunForLoop(startedRuns, string(created.ID)) {
				t.Fatalf("completed coordinator runs = %#v, want loop_run_id %q", startedRuns, created.ID)
			}
		},
	)
}

func TestSchedulerTaskSourceLoopCoordinatorBackstopShouldSettleWatchPollFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should terminalize a generation-zero Loop when watch polling fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 13, 11, 27, 32, 0, time.UTC)
		db := openDaemonTestGlobalDB(t)
		workspaceID := "ws-backstop-watch-poll-failure"
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        workspaceID,
			RootDir:   t.TempDir(),
			Name:      "Watch Poll Failure",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		loopName := "watch-poll-failure"
		definition, err := daemonLoopDefinitionFromSpec(testWatchLoopSpec(t, loopName))
		if err != nil {
			t.Fatalf("daemonLoopDefinitionFromSpec() error = %v", err)
		}
		resolved, err := looppkg.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(watch Loop) error = %v", err)
		}
		seedRun := looppkg.Run{
			ID:                "looprun-watch-poll-failure",
			WorkspaceID:       looppkg.WorkspaceID(workspaceID),
			LoopName:          loopName,
			Status:            looppkg.StatusRunning,
			ReattemptStrategy: looppkg.ReattemptFailedOnly,
			CreatedAt:         now,
			LastProgressAt:    now,
			IterationCap:      3,
			BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
			Inputs:            map[string]any{},
		}
		applyResolvedLoopRunPinningForTest(t, &seedRun, now, resolved)
		created, err := db.CreateLoopRunForStart(ctx, seedRun, loopdsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		pollErr := errors.New("gh repo view failed in /private/operator/workspace with token secret-value")
		runner, err := looppkg.NewCoordinatorRunner(
			db,
			db,
			db,
			discardLogger(),
			looppkg.WithCoordinatorWatchPoller(schedulerWatchPollerFunc(
				func(context.Context, watchpkg.PollRequest) (watchpkg.PollResponse, error) {
					return watchpkg.PollResponse{}, pollErr
				},
			)),
		)
		if err != nil {
			t.Fatalf("NewCoordinatorRunner() error = %v", err)
		}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(db),
			taskpkg.WithCoordinatorRunner(runner),
			taskpkg.WithGenerationStateFinalizer(looppkg.NewStoreFinalizer()),
			taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Valid()
			}),
			taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Terminal()
			}),
			taskpkg.WithManagerNow(func() time.Time { return now.Add(time.Second) }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		actor := schedulerCoordinatorActorContextForTest(t)

		started, err := (schedulerTaskSource{manager: manager, store: db}).RunLoopCoordinatorBackstop(
			ctx,
			now.Add(time.Second),
			actor,
		)
		if !errors.Is(err, pollErr) {
			t.Fatalf("RunLoopCoordinatorBackstop() error = %v, want wrapped poll error", err)
		}
		if started != 0 {
			t.Fatalf("started = %d, want 0 successful starts", started)
		}

		stored, err := db.GetLoopRunByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID() error = %v", err)
		}
		if got, want := stored.Status, looppkg.StatusFailed; got != want {
			t.Fatalf("Loop status = %q, want %q", got, want)
		}
		if stored.Generation != 0 {
			t.Fatalf("Loop generation = %d, want 0", stored.Generation)
		}

		failedRuns, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusFailed})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(failed) error = %v", err)
		}
		if got, want := len(failedRuns), 1; got != want {
			t.Fatalf("failed coordinator runs = %d, want %d", got, want)
		}
		failedRun := failedRuns[0]
		if !failedRun.LeaseUntil.IsZero() || !failedRun.HeartbeatAt.IsZero() {
			t.Fatalf("failed coordinator retained lease: %#v", failedRun)
		}
		if strings.Contains(failedRun.Error, "secret-value") || strings.Contains(failedRun.Error, "/private/operator") {
			t.Fatalf("failed coordinator persisted unsafe error = %q", failedRun.Error)
		}
		events, err := db.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			ReadScope:   store.ReadScope{AllProfiles: true},
			WorkspaceID: created.WorkspaceID,
			RunID:       created.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		assertGenerationZeroCoordinatorFailureEvent(t, events)
	})
}

func TestSchedulerTaskSourceWatchEventsGapRecoveryShouldRequireStoreCapability(t *testing.T) {
	t.Parallel()

	t.Run("Should fail when the scheduler store cannot enqueue watch-events gap wakes", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		source := schedulerTaskSource{
			store:              taskStoreWithoutWatchEventsGapWake{taskStore: db},
			watchEventsGapScan: newLoopWatchEventsGapScanState(),
		}
		err := source.enqueueWatchEventsGapWakes(
			testutil.Context(t),
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler-test"},
			time.Now().UTC(),
		)
		if !errors.Is(err, errLoopWatchEventsGapWakeStoreRequired) {
			t.Fatalf("enqueueWatchEventsGapWakes() error = %v, want %v", err, errLoopWatchEventsGapWakeStoreRequired)
		}
	})
}

func TestSchedulerTaskSourceLoopRetryDueRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the retry cursor when a page fails", func(t *testing.T) {
		t.Parallel()

		sentinel := errors.New("forced retry due page failure")
		initial := looppkg.RetryDueCursor{
			NextAttemptAt: time.Date(2026, 8, 2, 19, 0, 0, 0, time.UTC),
			LoopRunID:     "looprun-initial", Generation: 2, NodeID: "fetch", ItemIndex: 1,
		}
		next := looppkg.RetryDueCursor{
			NextAttemptAt: initial.NextAttemptAt.Add(time.Minute),
			LoopRunID:     "looprun-next", Generation: 3, NodeID: "publish", ItemIndex: 2,
		}
		state := newLoopRetryDueScanState()
		state.cursor = initial
		source := schedulerTaskSource{
			store:            retryDueErrorTaskStore{taskStore: openDaemonTestGlobalDB(t), next: next, err: sentinel},
			loopRetryDueScan: state,
		}
		err := source.enqueueDueLoopRetryWakes(
			testutil.Context(t),
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler-test"},
			initial.NextAttemptAt,
		)
		if !errors.Is(err, sentinel) {
			t.Fatalf("enqueueDueLoopRetryWakes() error = %v, want sentinel", err)
		}
		if state.cursor != initial {
			t.Fatalf("retry cursor = %#v, want unchanged %#v", state.cursor, initial)
		}
	})

	t.Run("Should preserve wait cursors when a page fails", func(t *testing.T) {
		t.Parallel()

		sentinel := errors.New("forced wait page failure")
		initialDue := looppkg.WaitDueCursor{
			ResumeAt:  time.Date(2026, 8, 2, 19, 0, 0, 0, time.UTC),
			LoopRunID: "looprun-due-initial", Generation: 2, NodeID: "wait", ItemIndex: 1,
		}
		nextDue := looppkg.WaitDueCursor{
			ResumeAt:  initialDue.ResumeAt.Add(time.Minute),
			LoopRunID: "looprun-due-next", Generation: 3, NodeID: "wait", ItemIndex: 2,
		}
		dueState := newLoopWaitDueScanState()
		dueState.cursor = initialDue
		dueSource := schedulerTaskSource{
			store: loopWaitDueErrorTaskStore{
				taskStore: openDaemonTestGlobalDB(t), next: nextDue, err: sentinel,
			},
			loopWaitDueScan: dueState,
		}
		if err := dueSource.resumeDueLoopWaits(testutil.Context(t), initialDue.ResumeAt); !errors.Is(err, sentinel) {
			t.Fatalf("resumeDueLoopWaits() error = %v, want sentinel", err)
		}
		if dueState.cursor != initialDue {
			t.Fatalf("wait due cursor = %#v, want unchanged %#v", dueState.cursor, initialDue)
		}

		initialEscalation := looppkg.WaitEscalationCursor{
			NextEscalationAt: initialDue.ResumeAt,
			LoopRunID:        "looprun-escalation-initial", Generation: 2, NodeID: "gate", ItemIndex: 1,
		}
		nextEscalation := looppkg.WaitEscalationCursor{
			NextEscalationAt: initialEscalation.NextEscalationAt.Add(time.Minute),
			LoopRunID:        "looprun-escalation-next", Generation: 3, NodeID: "gate", ItemIndex: 2,
		}
		escalationState := newLoopWaitEscalationScanState()
		escalationState.cursor = initialEscalation
		escalationSource := schedulerTaskSource{
			store: loopWaitEscalationErrorTaskStore{
				taskStore: openDaemonTestGlobalDB(t), next: nextEscalation, err: sentinel,
			},
			loopWaitEscalationScan: escalationState,
		}
		if err := escalationSource.escalateDueLoopWaits(
			testutil.Context(t), initialEscalation.NextEscalationAt,
		); !errors.Is(err, sentinel) {
			t.Fatalf("escalateDueLoopWaits() error = %v, want sentinel", err)
		}
		if escalationState.cursor != initialEscalation {
			t.Fatalf("wait escalation cursor = %#v, want unchanged %#v", escalationState.cursor, initialEscalation)
		}
	})

	t.Run("Should require admission claim sweeping support", func(t *testing.T) {
		t.Parallel()

		source := schedulerTaskSource{
			store: taskStoreWithoutWatchEventsGapWake{taskStore: openDaemonTestGlobalDB(t)},
		}
		err := source.sweepLoopAdmissionClaims(testutil.Context(t), time.Now().UTC())
		if !errors.Is(err, errLoopAdmissionClaimSweeperRequired) {
			t.Fatalf("sweepLoopAdmissionClaims() error = %v, want %v", err, errLoopAdmissionClaimSweeperRequired)
		}
	})

	t.Run("Should reserve and start an epoch-current due retry wake", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 20, 0, 0, 0, time.UTC)
		db := openDaemonTestGlobalDB(t)
		workspaceID := "ws-retry-due-backstop"
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID: workspaceID, RootDir: t.TempDir(), Name: workspaceID, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		loopRun := looppkg.Run{
			ID: "looprun-retry-due-backstop", WorkspaceID: looppkg.WorkspaceID(workspaceID),
			LoopName: "retry-due-backstop", Status: looppkg.StatusRunning,
			ReattemptStrategy: looppkg.ReattemptFailedOnly, IterationCap: 50,
			BudgetOnExceeded: loopdsl.BudgetExceededHalt, CreatedAt: now, LastProgressAt: now,
		}
		applyLoopRunPinningForTest(t, &loopRun, now)
		created, err := db.CreateLoopRunForStart(ctx, loopRun, loopdsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		actor := schedulerCoordinatorActorContextForTest(t)
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(db),
			taskpkg.WithCoordinatorRunner(schedulerBackstopCoordinatorRunner{}),
			taskpkg.WithGenerationStateFinalizer(schedulerBackstopGenerationFinalizer{}),
			taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Valid()
			}),
			taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
				return looppkg.Status(strings.TrimSpace(status)).Terminal()
			}),
			taskpkg.WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID, RunKind: taskpkg.RunKindCoordinator,
			ClaimerSessionID: "daemon-loop-retry-seed",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"},
			LeaseDuration:    time.Minute, Now: now.Add(time.Millisecond),
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(seed) error = %v", err)
		}
		firstScheduledAt := now.Add(-time.Minute)
		nextAttemptAt := now.Add(-time.Second)
		if _, err := db.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken, Actor: actor, Now: now.Add(2 * time.Millisecond),
			Plan: taskpkg.CoordinatorCompletionPlan{Yield: true, Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID: string(created.ID), Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					Generation: 1, NodeID: "fetch", Status: "retrying", Attempt: 1, Epoch: 1,
					FirstScheduledAt: &firstScheduledAt, NextAttemptAt: &nextAttemptAt,
				}}},
			}},
		}, looppkg.NewStoreFinalizer()); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(seed) error = %v", err)
		}

		started, err := (schedulerTaskSource{
			manager: manager, store: db, loopRetryDueScan: newLoopRetryDueScanState(),
		}).RunLoopCoordinatorBackstop(ctx, now, actor)
		if err != nil {
			t.Fatalf("RunLoopCoordinatorBackstop() error = %v", err)
		}
		if started != 1 {
			t.Fatalf("started = %d, want one due retry coordinator", started)
		}
		completed, err := db.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusCompleted})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(completed) error = %v", err)
		}
		if !schedulerCompletedRunForLoop(completed, string(created.ID)) {
			t.Fatalf("completed coordinator runs = %#v, want retry wake completion", completed)
		}
	})
}

func TestLoopRetryTimerShouldUseSharedWakeIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should fire through the epoch-fenced retry store after its due time", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 2, 20, 30, 0, 0, time.UTC)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		store := &recordingLoopRetryTimerStore{calls: make(chan looppkg.RetryDueCell, 1)}
		armer := newLoopRetryTimerArmer(ctx, store, discardLogger(), func() time.Time { return now })
		cell := looppkg.RetryDueCell{
			LoopRunID: "looprun-timer", Generation: 2, NodeID: "fetch", ItemIndex: 1,
			Attempt: 3, Epoch: 7, NextAttemptAt: now,
		}
		spec := taskpkg.CoordinatorTimerSpec{
			LoopRunID: string(cell.LoopRunID), Generation: cell.Generation, NodeID: string(cell.NodeID),
			ItemIndex: cell.ItemIndex, Attempt: cell.Attempt, IssuedEpoch: cell.Epoch,
			FireAt: cell.NextAttemptAt, IdempotencyKey: looppkg.RetryWakeIdempotencyKey(cell),
		}
		err := armer.ArmCoordinatorTimer(
			context.Background(),
			spec,
			schedulerCoordinatorActorContextForTest(t),
		)
		if err != nil {
			t.Fatalf("ArmCoordinatorTimer() error = %v", err)
		}
		select {
		case got := <-store.calls:
			if got != cell {
				t.Fatalf("timer cell = %#v, want %#v", got, cell)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("retry timer did not fire")
		}
	})
}

type recordingLoopRetryTimerStore struct {
	calls chan looppkg.RetryDueCell
}

func (s *recordingLoopRetryTimerStore) EnqueueLoopRetryWakeIfCurrent(
	_ context.Context,
	_ taskpkg.Origin,
	cell looppkg.RetryDueCell,
	_ time.Time,
) (taskpkg.Run, bool, bool, error) {
	s.calls <- cell
	return taskpkg.Run{}, true, true, nil
}

type taskStoreWithoutWatchEventsGapWake struct {
	taskStore
}

type retryDueErrorTaskStore struct {
	taskStore
	next looppkg.RetryDueCursor
	err  error
}

type loopWaitDueErrorTaskStore struct {
	taskStore
	next looppkg.WaitDueCursor
	err  error
}

func (s loopWaitDueErrorTaskStore) ResumeDueLoopWaitsPage(
	context.Context,
	time.Time,
	looppkg.WaitDueCursor,
	int,
) ([]taskpkg.Run, looppkg.WaitDueCursor, error) {
	return nil, s.next, s.err
}

type loopWaitEscalationErrorTaskStore struct {
	taskStore
	next looppkg.WaitEscalationCursor
	err  error
}

func (s loopWaitEscalationErrorTaskStore) EscalateDueLoopWaitsPage(
	context.Context,
	time.Time,
	looppkg.WaitEscalationCursor,
	int,
) (looppkg.WaitEscalationCursor, error) {
	return s.next, s.err
}

func (s retryDueErrorTaskStore) EnqueueDueLoopRetryWakesPage(
	context.Context,
	taskpkg.Origin,
	time.Time,
	looppkg.RetryDueCursor,
	int,
) ([]taskpkg.Run, looppkg.RetryDueCursor, error) {
	return nil, s.next, s.err
}

func enqueueSchedulerRunForTest(
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	taskID string,
	runID string,
	runKind taskpkg.RunKind,
	loopRunID string,
	idempotencyKey string,
) taskpkg.Run {
	t.Helper()

	sequence := 0
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithManagerNow(func() time.Time { return now }),
		taskpkg.WithIDGenerator(func(prefix string) (string, error) {
			if prefix == "run" {
				return runID, nil
			}
			sequence++
			return prefix + "-" + runID + "-" + strconv.Itoa(sequence), nil
		}),
	)
	if err != nil {
		t.Fatalf("task.NewManager(enqueue %q) error = %v", runID, err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(enqueue %q) error = %v", runID, err)
	}
	run, err := manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
		TaskID:         taskID,
		RunKind:        runKind,
		LoopRunID:      loopRunID,
		IdempotencyKey: idempotencyKey,
	}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(%q) error = %v", runID, err)
	}
	if run.ID != runID {
		t.Fatalf("EnqueueRun(%q).ID = %q", runID, run.ID)
	}
	return *run
}

func claimSchedulerRunForTest(
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	criteria taskpkg.ClaimCriteria,
) taskpkg.ClaimResult {
	t.Helper()

	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithManagerNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("task.NewManager(claim) error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(claim) error = %v", err)
	}
	criteria.Now = now
	claim, err := manager.ClaimNextRun(testutil.Context(t), criteria, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(%q) error = %v", criteria.RunID, err)
	}
	return *claim
}

func seedSchedulerNetworkWakeForTest(
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	workspaceID string,
	runID string,
	wakeID string,
	targetSessionID string,
) taskpkg.Run {
	t.Helper()

	senderSessionID := "sender-" + runID
	if err := db.RegisterSession(testutil.Context(t), store.SessionInfo{
		ProfileID: store.DefaultProfileID, ID: senderSessionID, AgentName: "coder", Provider: "test",
		WorkspaceID: workspaceID, State: "active", CreatedAt: now, UpdatedAt: now,
		RuntimeStatus: store.SessionRuntimeUnbound,
	}); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", senderSessionID, err)
	}
	if err := db.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		ProfileID: store.DefaultProfileID, WorkspaceID: workspaceID, Channel: "operations",
		Purpose: "Scheduler wake fixture", FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel(operations) error = %v", err)
	}
	directID, _, _, err := store.NetworkDirectRoomIdentity(
		workspaceID,
		"operations",
		senderSessionID,
		targetSessionID,
	)
	if err != nil {
		t.Fatalf("NetworkDirectRoomIdentity(%q) error = %v", runID, err)
	}
	result, err := db.AcceptNetworkMessage(testutil.Context(t), &store.AcceptNetworkMessageRequest{
		Message: store.NetworkConversationMessage{
			ProfileID: store.DefaultProfileID, MessageID: "message-" + runID, SessionID: senderSessionID,
			WorkspaceID: workspaceID, Channel: "operations",
			Surface: store.NetworkSurfaceDirect, DirectID: directID,
			Direction: "sent", PeerFrom: senderSessionID, PeerTo: targetSessionID,
			Kind: store.NetworkKindSay, Text: "scheduler wake", PreviewText: "scheduler wake",
			Body: []byte(`{"text":"scheduler wake"}`), Timestamp: now,
		},
		Dispositions: []store.NetworkMessageDisposition{{
			RecipientSessionID: targetSessionID,
			Decision:           store.NetworkDispositionDeliver,
		}},
		Admissions: []store.NetworkWakeAdmissionInput{{
			WorkspaceID:        workspaceID,
			RecipientSessionID: targetSessionID,
			OwnerKey:           "session:" + targetSessionID,
			Spec:               daemonTestLiveParticipation(workspaceID, "operations"),
			Trigger:            store.NetworkWakeTriggerDirect,
			Eligible:           true,
			Addressed:          true,
			RootID:             "message-" + runID,
			WakeID:             wakeID,
			TaskRunID:          runID,
		}},
	})
	if err != nil {
		t.Fatalf("AcceptNetworkMessage(%q) error = %v", runID, err)
	}
	if len(result.Admitted) != 1 || result.Admitted[0].TaskRunID != runID {
		t.Fatalf("AcceptNetworkMessage(%q).Admitted = %#v", runID, result.Admitted)
	}
	run, err := db.GetTaskRun(testutil.Context(t), runID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
	}
	return run
}

func seedCoordinatorBackstopRun(
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	prefix string,
	index int,
	workspaceID string,
) {
	t.Helper()

	at := now.Add(time.Duration(index) * time.Millisecond)
	loopRun := looppkg.Run{
		ID:                looppkg.RunID("looprun-backstop-" + prefix + "-" + strconv.Itoa(index)),
		WorkspaceID:       looppkg.WorkspaceID(workspaceID),
		LoopName:          "backstop-" + prefix + "-" + strconv.Itoa(index),
		Status:            looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		IterationCap:      50,
		BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
		CreatedAt:         at,
		LastProgressAt:    at,
	}
	applyLoopRunPinningForTest(t, &loopRun, now)
	if _, err := db.CreateLoopRunForStart(testutil.Context(t), loopRun, loopdsl.ConcurrencyAllow); err != nil {
		t.Fatalf("CreateLoopRunForStart(%s/%d) error = %v", prefix, index, err)
	}
}

func parkSchedulerWatchEventsLoopForTest(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	workspaceID string,
) (looppkg.Run, taskpkg.Task) {
	t.Helper()
	targetTask := daemonTaskRecordForTest("task-watch-events-backstop-target", now)
	targetTask.Scope = taskpkg.ScopeWorkspace
	targetTask.WorkspaceID = workspaceID
	if err := db.CreateTask(ctx, targetTask); err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	loopRun := looppkg.Run{
		ID:                "looprun-watch-events-backstop",
		WorkspaceID:       looppkg.WorkspaceID(workspaceID),
		LoopName:          "watch-events-backstop",
		Status:            looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		IterationCap:      50,
		BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
		CreatedAt:         now,
		LastProgressAt:    now,
		Inputs:            map[string]any{"target_task_id": targetTask.ID},
	}
	resolved := schedulerWatchEventsDefinitionForTest(t)
	applyResolvedLoopRunPinningForTest(t, &loopRun, now, resolved)
	created, err := db.CreateLoopRunForStart(ctx, loopRun, loopdsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	runner := newSchedulerWatchEventsCoordinatorForTest(t, db, targetTask.ID, resolved)
	actor := schedulerCoordinatorActorContextForTest(t)
	claim := claimSchedulerRunForTest(t, db, now, taskpkg.ClaimCriteria{
		Scope:         taskpkg.ScopeWorkspace,
		WorkspaceID:   workspaceID,
		RunKind:       taskpkg.RunKindCoordinator,
		ClaimedBy:     &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"},
		LeaseDuration: time.Minute,
	})
	plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
	if err != nil {
		t.Fatalf("Run(initial coordinator) error = %v", err)
	}
	if _, err := db.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      actor,
		Plan:       plan,
		Now:        now.Add(time.Second),
	}, looppkg.NewStoreFinalizer()); err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(initial) error = %v", err)
	}
	storedLoop, err := db.GetLoopRunByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(initial) error = %v", err)
	}
	if got, want := storedLoop.Status, looppkg.StatusWatching; got != want {
		t.Fatalf("initial terminal = %q, want %q", got, want)
	}
	return created, targetTask
}

func newSchedulerWatchEventsCoordinatorForTest(
	t *testing.T,
	db *globaldb.GlobalDB,
	targetTaskID string,
	resolved *looppkg.ResolvedDefinition,
) *looppkg.CoordinatorRunner {
	t.Helper()
	if resolved == nil {
		t.Fatal("resolved watch-events Loop definition is required")
	}
	runner, err := looppkg.NewCoordinatorRunner(
		db,
		db,
		db,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		looppkg.WithCoordinatorWatchEventsLedger(db),
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner(%s) error = %v", targetTaskID, err)
	}
	return runner
}

func schedulerWatchEventsDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(loopdsl.Definition{
		APIVersion: loopdsl.APIVersion,
		Kind:       loopdsl.KindLoop,
		Inputs: map[string]loopdsl.Input{
			"target_task_id": {Type: loopdsl.InputTypeString},
		},
		Graph: loopdsl.Graph{
			Nodes: []loopdsl.Node{{
				ID:    "watch_tasks",
				Class: loopdsl.NodeClassSource,
				Kind:  string(loopdsl.SourceWatchEvents),
				Events: []loopdsl.EventSubscription{{
					Kind:   string(hookspkg.HookTaskBlocked),
					Filter: `event.task_id == inputs.target_task_id`,
				}},
			}, {
				ID:    "summarize",
				Class: loopdsl.NodeClassAction,
				Kind:  string(loopdsl.ActionTransform),
				Params: loopdsl.NodeParams{
					"map": map[string]any{"ok": map[string]any{"value": true}},
				},
			}},
			Edges: []loopdsl.Edge{{From: "watch_tasks", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(watch-events backstop loop) error = %v", err)
	}
	return resolved
}

func appendSchedulerWatchEventForTest(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	taskID string,
	at time.Time,
) {
	t.Helper()
	actor := schedulerCoordinatorActorContextForTest(t)
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithManagerNow(func() time.Time { return at }),
	)
	if err != nil {
		t.Fatalf("task.NewManager(block event) error = %v", err)
	}
	if _, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
		TaskID: taskID,
		Kind:   taskpkg.BlockKindTransient,
		Reason: "scheduler backstop watch-events test",
	}, actor); err != nil {
		t.Fatalf("BlockTask() error = %v", err)
	}
}

func schedulerCoordinatorActorContextForTest(t *testing.T) taskpkg.ActorContext {
	t.Helper()
	actor, err := taskpkg.DeriveDaemonActorContext("loop", "daemon.loop")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(loop) error = %v", err)
	}
	return actor
}

func schedulerCompletedRunForLoop(runs []taskpkg.Run, loopRunID string) bool {
	for _, run := range runs {
		if strings.TrimSpace(run.LoopRunID) == loopRunID && run.RunKind.Normalize() == taskpkg.RunKindCoordinator {
			return true
		}
	}
	return false
}

type schedulerBackstopCoordinatorRunner struct{}

func (schedulerBackstopCoordinatorRunner) Run(
	context.Context,
	taskpkg.RunID,
) (taskpkg.CoordinatorCompletionPlan, error) {
	return taskpkg.CoordinatorCompletionPlan{
		Snapshot: taskpkg.GenerationSnapshot{Generation: 1},
		Terminal: &taskpkg.CoordinatorTerminal{
			Status: "no-op",
			Cause:  "backstop-test",
		},
	}, nil
}

type schedulerBackstopGenerationFinalizer struct{}

func (schedulerBackstopGenerationFinalizer) WriteGenerationSnapshot(
	context.Context,
	taskpkg.Tx,
	taskpkg.GenerationSnapshot,
) error {
	return nil
}

type schedulerWatchPollerFunc func(context.Context, watchpkg.PollRequest) (watchpkg.PollResponse, error)

func (f schedulerWatchPollerFunc) Poll(
	ctx context.Context,
	req watchpkg.PollRequest,
) (watchpkg.PollResponse, error) {
	return f(ctx, req)
}

func assertGenerationZeroCoordinatorFailureEvent(t *testing.T, events []looppkg.RunEvent) {
	t.Helper()

	var failureEvents []looppkg.RunEvent
	for _, event := range events {
		if event.Kind != "status_changed" {
			continue
		}
		var payload struct {
			Status  string `json:"status"`
			Failure *struct {
				Kind     string `json:"kind"`
				Code     string `json:"code"`
				Cause    string `json:"cause"`
				Recovery string `json:"recovery"`
			} `json:"failure"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal(status_changed payload) error = %v", err)
		}
		if payload.Status == string(looppkg.StatusFailed) && payload.Failure != nil {
			failureEvents = append(failureEvents, event)
			if got, want := payload.Failure.Kind, "coordinator_failure"; got != want {
				t.Fatalf("failure.kind = %q, want %q", got, want)
			}
			if got, want := payload.Failure.Code, "watch_poll_failed"; got != want {
				t.Fatalf("failure.code = %q, want %q", got, want)
			}
			if strings.TrimSpace(payload.Failure.Cause) == "" {
				t.Fatal("failure.cause is empty")
			}
			if strings.TrimSpace(payload.Failure.Recovery) == "" {
				t.Fatal("failure.recovery is empty")
			}
			if strings.Contains(string(event.Payload), "secret-value") ||
				strings.Contains(string(event.Payload), "/private/operator") {
				t.Fatalf("status_changed event persisted unsafe failure = %s", event.Payload)
			}
		}
	}
	if got, want := len(failureEvents), 1; got != want {
		t.Fatalf("generation-zero coordinator failure events = %d, want %d", got, want)
	}
}
