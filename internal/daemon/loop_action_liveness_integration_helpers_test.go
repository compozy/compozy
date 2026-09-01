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

	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/workspace"
)

var errLoopFailureBreakerIntegration = errors.New("forced breaker failure")

func testLoopActionLivenessIntegration(t *testing.T) {
	t.Helper()
	testLoopActionSettlementIntegration(
		t,
		"action-liveness",
		quietLoopActionExecutor{},
		taskpkg.MaxResultBytes,
		false,
	)
}

func testLoopActionWithinBudgetResultIntegration(t *testing.T) {
	t.Helper()
	testLoopActionSettlementIntegration(
		t,
		"within-budget-result",
		oversizedLoopActionExecutor{dataBytes: taskpkg.MaxResultBytes},
		2*taskpkg.MaxResultBytes,
		false,
	)
}

func testLoopActionAboveBudgetResultIntegration(t *testing.T) {
	t.Helper()
	testLoopActionSettlementIntegration(
		t,
		"above-budget-result",
		oversizedLoopActionExecutor{dataBytes: taskpkg.MaxResultBytes},
		taskpkg.MaxResultBytes,
		true,
	)
}

func testLoopActionSettlementIntegration(
	t *testing.T,
	suffix string,
	executor looppkg.ActionExecutor,
	actionResultMaxBytes int64,
	wantFailure bool,
) {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(root, store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	now := time.Now().UTC()
	workspaceID := "workspace-" + suffix
	if err := db.InsertWorkspace(ctx, workspace.Workspace{
		ID: workspaceID, Name: "Action liveness", RootDir: root, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	resolved := compileManagedGoalDefinition(t, suffix, "worker", "main")
	run := looppkg.Run{
		ID:                looppkg.RunID("looprun-" + suffix),
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
		looppkg.WithActionGoalExecutor(executor),
		looppkg.WithActionDefaultMaxResultBytes(actionResultMaxBytes),
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
		taskpkg.WithActionResultMaxBytes(actionResultMaxBytes),
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

	runtime.heartbeatInterval = func(time.Duration) time.Duration { return 5 * time.Millisecond }
	runtime.livenessPollInterval = func(time.Duration) time.Duration { return 5 * time.Millisecond }
	executeErr := runtime.executeQueuedRun(ctx, taskRecord, worker, loopActionRuntimeReasonEnqueued)
	if wantFailure {
		if executeErr == nil || !errors.Is(executeErr, looppkg.ErrActionResultTooLarge) {
			t.Fatalf(
				"executeQueuedRun(above budget) error = %v, want ErrActionResultTooLarge",
				executeErr,
			)
		}
	} else if executeErr != nil {
		t.Fatalf("executeQueuedRun(quiet) error = %v", executeErr)
	}
	settled, err := db.GetTaskRun(ctx, worker.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(settled worker) error = %v", err)
	}
	wantStatus := taskpkg.TaskRunStatusCompleted
	if wantFailure {
		wantStatus = taskpkg.TaskRunStatusFailed
	}
	if settled.Status.Normalize() != wantStatus || !settled.LeaseUntil.IsZero() {
		t.Fatalf("worker lease state = %#v, want %s without an owned lease", settled, wantStatus)
	}
	if settled.ClaimTokenHash == "" {
		t.Fatalf("worker ClaimTokenHash = empty, want retained fencing history")
	}
	outputs, err := db.ListGenerationOutputs(ctx, created.WorkspaceID, created.ID, 1)
	if err != nil {
		t.Fatalf("ListGenerationOutputs(completed worker) error = %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("generation outputs = %#v, want one node cell", outputs)
	}
	if wantFailure && outputs[0].Status != "failed" {
		t.Fatalf("oversized generation output = %#v, want failed", outputs[0])
	}
	if wantFailure && (!strings.Contains(outputs[0].OutputRef, "action_result_too_large") ||
		!strings.Contains(outputs[0].OutputRef, `action node \"converge\"`) ||
		!strings.Contains(outputs[0].OutputRef, "65536-byte result limit")) {
		t.Fatalf("oversized generation output = %#v, want bounded node-aware diagnostic", outputs[0])
	}
	if !wantFailure && outputs[0].Status == "failed" {
		t.Fatalf("quiet generation outputs = %#v, want no liveness failure", outputs)
	}
	if !wantFailure && strings.Contains(suffix, "within-budget") {
		if len(settled.ResultValue()) != 0 || settled.ResultReference() == "" ||
			settled.ResultByteCount() <= taskpkg.MaxResultBytes {
			t.Fatalf("settled result = %#v, want external descriptor above envelope cap", settled)
		}
		if outputs[0].OutputRef != settled.ResultReference() {
			t.Fatalf(
				"generation output ref = %q, want settled ref %q",
				outputs[0].OutputRef,
				settled.ResultReference(),
			)
		}
	}
}

func testLoopFailureBreakerIntegration(t *testing.T) {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(root, store.GlobalDatabaseName))
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
		looppkg.WithActionTransformExecutor(failureBreakerActionExecutor{}),
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
	if plan.Terminal != nil || plan.NextCoordinator == nil {
		t.Fatalf(
			"second retry plan terminal/next = %#v/%#v, want quarantine continuation",
			plan.Terminal,
			plan.NextCoordinator,
		)
	}
	if _, err := manager.StartRun(ctx, secondRetry.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + secondRetry.Run.ID,
		ClaimToken:     secondRetry.ClaimToken,
	}, actor); err != nil {
		t.Fatalf("StartRun(second retry coordinator) error = %v", err)
	}
	activeRun, err := db.GetLoopRunByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLoopRunByID(active) error = %v", err)
	}
	if activeRun.Status != looppkg.StatusRunning || activeRun.Generation != 3 {
		t.Fatalf("active Loop = %#v, want running generation 3", activeRun)
	}
	controls, err := db.ListNodeControls(ctx, created.WorkspaceID, created.ID)
	if err != nil {
		t.Fatalf("ListNodeControls() error = %v", err)
	}
	controlByNode := make(map[looppkg.NodeID]looppkg.NodeControl, len(controls))
	for _, control := range controls {
		controlByNode[control.NodeID] = control
	}
	if !controlByNode["failing"].Quarantined || controlByNode["passing"].Quarantined {
		t.Fatalf("node controls = %#v, want only failing quarantined", controls)
	}
}

func compileFailureBreakerDefinition(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()

	base := compileManagedGoalDefinition(t, "failure-breaker", "worker", "failure")
	definition := base.Definition
	definition.Contract.NoProgress.Window = 10
	failing := definition.Graph.Nodes[0]
	failing.ID = "failing"
	failing.Kind = string(loopdsl.ActionTransform)
	failing.Session = nil
	failing.Params = loopdsl.NodeParams{"map": map[string]any{
		"status": map[string]any{"value": "failed"},
	}}
	passing := failing
	passing.ID = "passing"
	passing.Params = loopdsl.NodeParams{"map": map[string]any{
		"status": map[string]any{"value": "complete"},
	}}
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
				t.Fatalf(
					"executeQueuedRun(failing generation %d) error = %v, want %v",
					generation,
					executeErr,
					errLoopFailureBreakerIntegration,
				)
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

type quietLoopActionExecutor struct{}

func (e quietLoopActionExecutor) Execute(
	ctx context.Context,
	_ loopdsl.Node,
	_ looppkg.ActionExecutionInput,
) (looppkg.ActionRawResult, error) {
	timer := time.NewTimer(75 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return looppkg.ActionRawResult{Value: map[string]any{"status": "complete"}}, nil
	case <-ctx.Done():
		return looppkg.ActionRawResult{}, ctx.Err()
	}
}

func (quietLoopActionExecutor) Harvest(
	_ context.Context,
	raw looppkg.ActionRawResult,
	_ loopdsl.Node,
) (looppkg.ActionOutput, error) {
	return looppkg.ActionOutput{Value: raw.Value}, nil
}

type oversizedLoopActionExecutor struct {
	dataBytes int
}

func (e oversizedLoopActionExecutor) Execute(
	_ context.Context,
	_ loopdsl.Node,
	_ looppkg.ActionExecutionInput,
) (looppkg.ActionRawResult, error) {
	return looppkg.ActionRawResult{Value: map[string]any{
		"status": "complete",
		"data":   strings.Repeat("x", e.dataBytes),
	}}, nil
}

func (oversizedLoopActionExecutor) Harvest(
	_ context.Context,
	raw looppkg.ActionRawResult,
	_ loopdsl.Node,
) (looppkg.ActionOutput, error) {
	return looppkg.ActionOutput{Value: raw.Value}, nil
}
