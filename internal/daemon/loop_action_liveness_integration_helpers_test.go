//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/workspace"
)

const loopActionLivenessIntegrationTimeout = 40 * time.Millisecond

var errLoopFailureBreakerIntegration = errors.New("forced breaker failure")

func testLoopActionLivenessIntegration(t *testing.T) {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(root, store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	now := time.Now().UTC()
	workspaceID := "workspace-action-liveness"
	if err := db.InsertWorkspace(ctx, workspace.Workspace{
		ID: workspaceID, Name: "Action liveness", RootDir: root, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	resolved := compileManagedGoalDefinition(t, "action-liveness", "worker", "main")
	run := looppkg.Run{
		ID:                "looprun-action-liveness",
		WorkspaceID:       looppkg.WorkspaceID(workspaceID),
		LoopName:          resolved.Definition.Meta.Name,
		Status:            looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		CreatedAt:         now,
		StartedAt:         now,
		LastProgressAt:    now,
		Inputs:            map[string]any{},
		IterationCap:      3,
		BudgetTokens:      100,
		BudgetWallSec:     60,
		BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
	}
	applyResolvedLoopRunPinningForTest(t, &run, now, resolved)
	created, err := db.CreateLoopRunForStart(ctx, run, loopdsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}

	actions, err := looppkg.NewActionRegistry(
		inertActionToolRegistry{},
		looppkg.WithActionGoalExecutor(wedgedLoopActionExecutor{}),
	)
	if err != nil {
		t.Fatalf("loop.NewActionRegistry() error = %v", err)
	}
	coordinator, err := looppkg.NewCoordinatorRunner(
		db,
		db,
		db,
		discardLogger(),
		looppkg.WithCoordinatorActionRegistry(actions),
	)
	if err != nil {
		t.Fatalf("loop.NewCoordinatorRunner() error = %v", err)
	}
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithCoordinatorRunner(coordinator),
		taskpkg.WithGenerationStateFinalizer(looppkg.NewStoreFinalizer()),
		taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
			return looppkg.Status(status).Valid()
		}),
		taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
			return looppkg.Status(status).Terminal()
		}),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop-action", "daemon.loop-action")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	initial := claimLoopRunForIntegration(
		t,
		manager,
		db,
		created,
		taskpkg.RunKindCoordinator,
		"",
		actor,
	)
	if _, err := manager.StartRun(ctx, initial.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + initial.Run.ID,
		ClaimToken:     initial.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(initial coordinator) error = %v", err)
	}

	worker := queuedLoopWorkerForIntegration(t, db, created.ID, 1)
	taskRecord, err := db.GetTask(ctx, worker.TaskID)
	if err != nil {
		t.Fatalf("GetTask(worker) error = %v", err)
	}
	runtime, err := newLoopActionRuntime(
		manager,
		db,
		coordinator,
		nil,
		discardLogger(),
		nil,
		loopActionLivenessIntegrationTimeout,
	)
	if err != nil {
		t.Fatalf("newLoopActionRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.shutdown(shutdownCtx); err != nil {
			t.Errorf("loopActionRuntime.shutdown() error = %v", err)
		}
	})

	if err := runtime.executeQueuedRun(ctx, taskRecord, worker, loopActionRuntimeReasonEnqueued); err == nil {
		t.Fatal("executeQueuedRun(wedged) error = nil, want liveness failure")
	}
	failed, err := db.GetTaskRun(ctx, worker.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(failed worker) error = %v", err)
	}
	if failed.Status.Normalize() != taskpkg.TaskRunStatusFailed || !failed.LeaseUntil.IsZero() {
		t.Fatalf("failed worker lease state = %#v", failed)
	}
	if !strings.Contains(failed.Error, loopActionReasonNoProgress) {
		t.Fatalf("failed worker error = %q, want reason %q", failed.Error, loopActionReasonNoProgress)
	}
	outputs, err := db.ListGenerationOutputs(ctx, created.ID, 1)
	if err != nil {
		t.Fatalf("ListGenerationOutputs(failed worker) error = %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("failed generation outputs = %#v, want exactly one", outputs)
	}
	var actionFailure looppkg.ActionFailure
	if err := json.Unmarshal([]byte(outputs[0].OutputRef), &actionFailure); err != nil {
		t.Fatalf("decode action failure output ref error = %v", err)
	}
	if actionFailure.Code != loopActionReasonNoProgress {
		t.Fatalf("action failure code = %q, want %q", actionFailure.Code, loopActionReasonNoProgress)
	}

	reconciled, err := db.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, time.Now().UTC())
	if err != nil {
		t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
	}
	if len(reconciled) != 1 {
		t.Fatalf("reconciled coordinator runs = %#v, want exactly one", reconciled)
	}
	next := claimLoopRunForIntegration(
		t,
		manager,
		db,
		created,
		taskpkg.RunKindCoordinator,
		reconciled[0].ID,
		actor,
	)
	if _, err := manager.StartRun(ctx, next.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + next.Run.ID,
		ClaimToken:     next.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(retry coordinator) error = %v", err)
	}
	advanced, err := db.GetLoopRunByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(advanced) error = %v", err)
	}
	if advanced.Generation != 2 || advanced.Status != looppkg.StatusRunning {
		t.Fatalf("advanced Loop = %#v, want running generation 2", advanced)
	}
	generationTwoOutputs, err := db.ListGenerationOutputs(ctx, created.ID, 2)
	if err != nil {
		t.Fatalf("ListGenerationOutputs(generation two) error = %v", err)
	}
	if len(generationTwoOutputs) != 1 || generationTwoOutputs[0].Status != "pending" {
		t.Fatalf("generation two outputs = %#v, want one pending retry", generationTwoOutputs)
	}
}

func testLoopFailureBreakerIntegration(t *testing.T) {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(root, store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	now := time.Now().UTC()
	workspaceID := "workspace-failure-breaker"
	if err := db.InsertWorkspace(ctx, workspace.Workspace{
		ID: workspaceID, Name: "Failure breaker", RootDir: root, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	resolved := compileFailureBreakerDefinition(t)
	run := looppkg.Run{
		ID:                "looprun-failure-breaker",
		WorkspaceID:       looppkg.WorkspaceID(workspaceID),
		LoopName:          resolved.Definition.Meta.Name,
		Status:            looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		CreatedAt:         now,
		StartedAt:         now,
		LastProgressAt:    now,
		Inputs:            map[string]any{},
		IterationCap:      3,
		BudgetTokens:      100,
		BudgetWallSec:     60,
		BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
	}
	applyResolvedLoopRunPinningForTest(t, &run, now, resolved)
	created, err := db.CreateLoopRunForStart(ctx, run, loopdsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}

	actions, err := looppkg.NewActionRegistry(
		inertActionToolRegistry{},
		looppkg.WithActionGoalExecutor(failureBreakerActionExecutor{}),
	)
	if err != nil {
		t.Fatalf("loop.NewActionRegistry() error = %v", err)
	}
	coordinator, err := looppkg.NewCoordinatorRunner(
		db,
		db,
		db,
		discardLogger(),
		looppkg.WithCoordinatorActionRegistry(actions),
	)
	if err != nil {
		t.Fatalf("loop.NewCoordinatorRunner() error = %v", err)
	}
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithCoordinatorRunner(coordinator),
		taskpkg.WithGenerationStateFinalizer(looppkg.NewStoreFinalizer()),
		taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
			return looppkg.Status(status).Valid()
		}),
		taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
			return looppkg.Status(status).Terminal()
		}),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop-breaker", "daemon.loop-breaker")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	initial := claimLoopRunForIntegration(t, manager, db, created, taskpkg.RunKindCoordinator, "", actor)
	if _, err := manager.StartRun(ctx, initial.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + initial.Run.ID,
		ClaimToken:     initial.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(initial coordinator) error = %v", err)
	}

	runtime, err := newLoopActionRuntime(
		manager,
		db,
		coordinator,
		nil,
		discardLogger(),
		nil,
		loopActionLivenessIntegrationTimeout,
	)
	if err != nil {
		t.Fatalf("newLoopActionRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.shutdown(shutdownCtx); err != nil {
			t.Errorf("loopActionRuntime.shutdown() error = %v", err)
		}
	})

	executeFailureBreakerGeneration(t, runtime, db, created.ID, 1, 2)
	firstWake, added, err := db.EnqueueLoopCoordinatorWake(
		ctx,
		string(created.ID),
		"failure-breaker-generation-1",
		actor.Origin,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("EnqueueLoopCoordinatorWake(first) error = %v", err)
	}
	if !added {
		t.Fatal("EnqueueLoopCoordinatorWake(first) added = false, want true")
	}
	firstRetry := claimLoopRunForIntegration(
		t,
		manager,
		db,
		created,
		taskpkg.RunKindCoordinator,
		firstWake.ID,
		actor,
	)
	firstRetryPlan, err := coordinator.Run(ctx, taskpkg.RunID(firstRetry.Run.ID))
	if err != nil {
		t.Fatalf("CoordinatorRunner.Run(first retry) error = %v", err)
	}
	if firstRetryPlan.Terminal != nil || firstRetryPlan.NextCoordinator == nil {
		t.Fatalf(
			"first retry plan terminal/next = %#v/%#v, want retry continuation",
			firstRetryPlan.Terminal,
			firstRetryPlan.NextCoordinator,
		)
	}
	if _, err := manager.StartRun(ctx, firstRetry.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + firstRetry.Run.ID,
		ClaimToken:     firstRetry.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(first retry coordinator) error = %v", err)
	}
	generationTwoScheduler := claimLoopRunForIntegration(
		t,
		manager,
		db,
		created,
		taskpkg.RunKindCoordinator,
		"",
		actor,
	)
	if _, err := manager.StartRun(ctx, generationTwoScheduler.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + generationTwoScheduler.Run.ID,
		ClaimToken:     generationTwoScheduler.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(generation two scheduler) error = %v", err)
	}

	executeFailureBreakerGeneration(t, runtime, db, created.ID, 2, 1)
	secondWake, added, err := db.EnqueueLoopCoordinatorWake(
		ctx,
		string(created.ID),
		"failure-breaker-generation-2",
		actor.Origin,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("EnqueueLoopCoordinatorWake(second) error = %v", err)
	}
	if !added {
		t.Fatal("EnqueueLoopCoordinatorWake(second) added = false, want true")
	}
	secondRetry := claimLoopRunForIntegration(
		t,
		manager,
		db,
		created,
		taskpkg.RunKindCoordinator,
		secondWake.ID,
		actor,
	)
	plan, err := coordinator.Run(ctx, taskpkg.RunID(secondRetry.Run.ID))
	if err != nil {
		t.Fatalf("CoordinatorRunner.Run(second retry) error = %v", err)
	}
	if plan.Terminal == nil {
		t.Fatal("CoordinatorRunner.Run(second retry) terminal = nil")
	}
	if got, want := plan.Terminal.Status, string(looppkg.StatusStalled); got != want {
		t.Fatalf("terminal status = %q, want %q", got, want)
	}
	if got, want := plan.Terminal.Cause, string(looppkg.TransitionCauseNoProgress); got != want {
		t.Fatalf("terminal cause = %q, want %q", got, want)
	}
	if got, want := plan.Terminal.ReasonCode, "circuit_breaker"; got != want {
		t.Fatalf("terminal reason = %q, want %q", got, want)
	}
	if _, err := manager.StartRun(ctx, secondRetry.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + secondRetry.Run.ID,
		ClaimToken:     secondRetry.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(second retry coordinator) error = %v", err)
	}
	terminalRun, err := db.GetLoopRunByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(terminal) error = %v", err)
	}
	if terminalRun.Status != looppkg.StatusStalled || terminalRun.Generation != 2 {
		t.Fatalf("terminal Loop = %#v, want stalled generation 2", terminalRun)
	}
}

func compileFailureBreakerDefinition(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()

	base := compileManagedGoalDefinition(t, "failure-breaker", "worker", "failure")
	definition := base.Definition
	definition.Contract.NoProgress.Window = 10
	failing := definition.Graph.Nodes[0]
	failing.ID = "failing"
	failingSession := *failing.Session
	failingSession.Handle = "failure"
	failing.Session = &failingSession
	passing := failing
	passing.ID = "passing"
	passingSession := *passing.Session
	passingSession.Handle = "passing"
	passing.Session = &passingSession
	definition.Graph.Nodes = []loopdsl.Node{failing, passing}

	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(failure breaker definition) error = %v", err)
	}
	return resolved
}

func executeFailureBreakerGeneration(
	t *testing.T,
	runtime *loopActionRuntime,
	db *globaldb.GlobalDB,
	loopRunID looppkg.RunID,
	generation int,
	wantRuns int,
) {
	t.Helper()

	ctx := testutil.Context(t)
	runs := queuedLoopWorkersForIntegration(t, db, loopRunID, generation)
	if len(runs) != wantRuns {
		t.Fatalf("generation %d queued workers = %#v, want %d", generation, runs, wantRuns)
	}
	for _, run := range runs {
		taskRecord, err := db.GetTask(ctx, run.TaskID)
		if err != nil {
			t.Fatalf("GetTask(%s) error = %v", run.TaskID, err)
		}
		var metadata loopGoalIntegrationRunMetadata
		if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
			t.Fatalf("decode generation %d worker metadata error = %v", generation, err)
		}
		executeErr := runtime.executeQueuedRun(ctx, taskRecord, run, loopActionRuntimeReasonEnqueued)
		if metadata.NodeID == "failing" {
			if !errors.Is(executeErr, errLoopFailureBreakerIntegration) {
				t.Fatalf("executeQueuedRun(failing generation %d) error = %v, want %v", generation, executeErr, errLoopFailureBreakerIntegration)
			}
			continue
		}
		if executeErr != nil {
			t.Fatalf("executeQueuedRun(passing generation %d) error = %v", generation, executeErr)
		}
	}
}

func queuedLoopWorkersForIntegration(
	t *testing.T,
	db taskStore,
	loopRunID looppkg.RunID,
	generation int,
) []taskpkg.Run {
	t.Helper()

	runs, err := db.ListTaskRunsByStatus(
		testutil.Context(t),
		[]taskpkg.RunStatus{taskpkg.TaskRunStatusQueued},
	)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
	}
	workers := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if run.LoopRunID != string(loopRunID) || run.RunKind.Normalize() != taskpkg.RunKindWorker {
			continue
		}
		var metadata loopGoalIntegrationRunMetadata
		if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
			t.Fatalf("decode worker metadata error = %v", err)
		}
		if metadata.Generation == generation {
			workers = append(workers, run)
		}
	}
	return workers
}

type failureBreakerActionExecutor struct{}

func (failureBreakerActionExecutor) Execute(
	_ context.Context,
	node loopdsl.Node,
	_ looppkg.ActionExecutionInput,
) (looppkg.ActionRawResult, error) {
	if node.ID == "failing" {
		return looppkg.ActionRawResult{}, errLoopFailureBreakerIntegration
	}
	return looppkg.ActionRawResult{Value: map[string]any{"status": "complete"}}, nil
}

func (failureBreakerActionExecutor) Harvest(
	_ context.Context,
	raw looppkg.ActionRawResult,
	_ loopdsl.Node,
) (looppkg.ActionOutput, error) {
	return looppkg.ActionOutput{Value: raw.Value}, nil
}

func queuedLoopWorkerForIntegration(
	t *testing.T,
	db taskStore,
	loopRunID looppkg.RunID,
	generation int,
) taskpkg.Run {
	t.Helper()

	runs, err := db.ListTaskRunsByStatus(
		testutil.Context(t),
		[]taskpkg.RunStatus{taskpkg.TaskRunStatusQueued},
	)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
	}
	for _, run := range runs {
		if run.LoopRunID != string(loopRunID) || run.RunKind.Normalize() != taskpkg.RunKindWorker {
			continue
		}
		var metadata loopGoalIntegrationRunMetadata
		if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
			t.Fatalf("decode worker metadata error = %v", err)
		}
		if metadata.Generation == generation {
			return run
		}
	}
	t.Fatalf("no queued Loop worker for generation %d", generation)
	return taskpkg.Run{}
}

type wedgedLoopActionExecutor struct{}

func (wedgedLoopActionExecutor) Execute(
	ctx context.Context,
	_ loopdsl.Node,
	_ looppkg.ActionExecutionInput,
) (looppkg.ActionRawResult, error) {
	<-ctx.Done()
	return looppkg.ActionRawResult{}, ctx.Err()
}

func (wedgedLoopActionExecutor) Harvest(
	context.Context,
	looppkg.ActionRawResult,
	loopdsl.Node,
) (looppkg.ActionOutput, error) {
	return looppkg.ActionOutput{}, nil
}
