package loop

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

const (
	generationOutputPending        = "pending"
	generationOutputEnqueued       = "enqueued"
	generationOutputRunning        = "running"
	generationOutputRetrying       = "retrying"
	generationOutputWaiting        = "waiting"
	generationOutputPaused         = "paused"
	generationOutputAwaitingChild  = "awaiting_child"
	generationOutputControlPending = GenerationOutputStatusControlPending
	generationOutputAwaitingGoal   = GenerationOutputStatusAwaitingGoal
	generationOutputSucceeded      = "succeeded"
	generationOutputFailed         = "failed"
	generationOutputCanceled       = "canceled"
	generationOutputQuarantined    = "quarantined"

	childLoopTimeoutReason     = "child_loop_timeout"
	blockingIssuesRepeatedCode = "blocking_issues_repeated"
)

const (
	// GenerationOutputStatusControlPending identifies a completed worker whose Loop control is unsettled.
	GenerationOutputStatusControlPending = "control_pending"
	// GenerationOutputStatusAwaitingGoal identifies a settled pause or approval gate awaiting re-entry.
	GenerationOutputStatusAwaitingGoal = "awaiting_goal"
)

// GenerationOutputStatusParked reports whether a cell is deliberately excluded
// from scheduling, rerun, and no-progress arithmetic.
func GenerationOutputStatusParked(status string) bool {
	switch strings.TrimSpace(status) {
	case generationOutputPaused,
		generationOutputWaiting,
		generationOutputAwaitingGoal,
		generationOutputQuarantined:
		return true
	default:
		return false
	}
}

// CoordinatorTaskRunReader is the task-run read seam required by the loop coordinator.
type CoordinatorTaskRunReader interface {
	GetTaskRun(ctx context.Context, id string) (task.Run, error)
}

// GenerationOutputLister reads loop-owned generation output snapshots.
type GenerationOutputLister interface {
	ListGenerationOutputs(
		ctx context.Context,
		workspaceID WorkspaceID,
		runID RunID,
		generation int,
	) ([]GenerationOutput, error)
}

// GenerationOutputReader lists snapshots and hydrates externalized output payloads for the coordinator.
type GenerationOutputReader interface {
	GenerationOutputLister
	GenerationOutputPayloadReader
}

// NodeAttemptReader reads the immutable attempt ledger for lifecycle planning and repair context.
type NodeAttemptReader interface {
	ListNodeAttempts(
		ctx context.Context,
		workspaceID WorkspaceID,
		runID RunID,
	) ([]NodeAttempt, error)
}

// NodeControlReader reads cross-generation parked state for succession planning.
type NodeControlReader interface {
	ListNodeControls(context.Context, WorkspaceID, RunID) ([]NodeControl, error)
}

// GateDecisionReader reads persisted human decisions for coordinator gate re-evaluation.
type GateDecisionReader interface {
	ListLoopGateDecisions(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		generation int,
		gateID NodeID,
	) (map[string]gate.HumanDecision, error)
}

// CoordinatorRunner computes loop generation plans without mutating task storage.
type CoordinatorRunner struct {
	taskRuns           CoordinatorTaskRunReader
	store              Store
	outputs            GenerationOutputReader
	attempts           NodeAttemptReader
	controls           NodeControlReader
	verdicts           gate.VerdictReader
	hooks              HookDispatcher
	gateEvaluator      gate.GateEvaluator
	actionRegistry     *ActionRegistry
	runtimeCatalog     WorkspaceRuntimeCatalog
	targetHealth       TargetHealth
	targetProbes       sync.Map
	logger             *slog.Logger
	now                func() time.Time
	retryRand          func() float64
	watchPoller        WatchPoller
	watchEventsLedger  WatchEventsLedger
	watchSilenceWindow time.Duration
}

var _ task.CoordinatorRunner = (*CoordinatorRunner)(nil)

// NewCoordinatorRunner constructs the in-daemon loop generation coordinator.
func NewCoordinatorRunner(
	taskRuns CoordinatorTaskRunReader,
	loopStore Store,
	outputs GenerationOutputReader,
	logger *slog.Logger,
	opts ...CoordinatorRunnerOption,
) (*CoordinatorRunner, error) {
	if taskRuns == nil {
		return nil, fmt.Errorf("%w: task run reader is required", ErrValidation)
	}
	if loopStore == nil {
		return nil, fmt.Errorf("%w: loop store is required", ErrValidation)
	}
	if outputs == nil {
		return nil, fmt.Errorf("%w: generation output reader is required", ErrValidation)
	}
	if logger == nil {
		logger = slog.Default()
	}
	runner := &CoordinatorRunner{
		taskRuns:           taskRuns,
		store:              loopStore,
		outputs:            outputs,
		logger:             logger,
		now:                time.Now,
		retryRand:          rand.Float64,
		watchSilenceWindow: DefaultWatchSilenceWindow,
	}
	if verdicts, ok := outputs.(gate.VerdictReader); ok {
		runner.verdicts = verdicts
	}
	if attempts, ok := outputs.(NodeAttemptReader); ok {
		runner.attempts = attempts
	}
	if controls, ok := loopStore.(NodeControlReader); ok {
		runner.controls = controls
	}
	for _, opt := range opts {
		opt(runner)
	}
	return runner, nil
}

// Run returns the coordinator plan for a claimed coordinator task_run.
func (r *CoordinatorRunner) Run(
	ctx context.Context,
	taskRunID task.RunID,
) (task.CoordinatorCompletionPlan, error) {
	trimmedRunID := strings.TrimSpace(string(taskRunID))
	if trimmedRunID == "" {
		return task.CoordinatorCompletionPlan{}, fmt.Errorf(
			"%w: task run id is required",
			ErrValidation,
		)
	}
	taskRun, err := r.taskRuns.GetTaskRun(ctx, trimmedRunID)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if taskRun.RunKind.Normalize() != task.RunKindCoordinator {
		return task.CoordinatorCompletionPlan{}, fmt.Errorf(
			"%w: task run %q is %q, not %q",
			ErrValidation,
			taskRun.ID,
			taskRun.RunKind.Normalize(),
			task.RunKindCoordinator,
		)
	}
	loopRunID := RunID(strings.TrimSpace(taskRun.LoopRunID))
	if loopRunID == "" {
		return task.CoordinatorCompletionPlan{}, fmt.Errorf(
			"%w: coordinator task run has no loop_run_id",
			ErrValidation,
		)
	}
	loopRun, err := r.store.GetLoopRunByID(ctx, loopRunID)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	resolved, err := r.resolvePinnedDefinition(ctx, loopRun)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan, err := r.buildCoordinatorPlan(ctx, taskRun, loopRun, resolved)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan, err = r.applyPausedNodeControls(ctx, loopRun, plan)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := attachCoordinatorEffectIntents(loopRun, resolved, &plan); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := attachCoordinatorParentCloseIntents(loopRun, resolved, &plan); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	return plan, nil
}

func (r *CoordinatorRunner) buildCoordinatorPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	resolved *ResolvedDefinition,
) (task.CoordinatorCompletionPlan, error) {
	executionResolved, err := controlExecutionResolved(resolved)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	resolved = executionResolved
	effective, err := pinnedEffectiveConfig(resolved)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	resolved = coordinatorResolvedWithEffectiveConfig(resolved, effective)
	def := resolved.Definition
	graph := def.Graph
	fanOutWidth := coordinatorFanOutWidth(effective)
	if len(graph.Nodes) == 0 {
		plan := noOpCoordinatorPlan(run)
		return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
	}
	if plan, found, err := r.buildExistingGenerationPlan(
		ctx, taskRun, run, resolved, effective, fanOutWidth,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	} else if found {
		return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
	}
	generation := run.Generation + 1
	if terminal := iterationCapTerminal(run, generation); terminal != nil {
		plan := terminalCoordinatorPlan(run, terminal)
		return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
	}
	intent := GenerationIntent{
		Generation:       int64(generation),
		ParentGeneration: 0,
		Origin:           OriginInitial,
	}
	if denied, plan := r.dispatchGenerationPre(ctx, taskRun, run, intent); denied {
		return plan, nil
	}
	history, err := r.readGenerationHistory(ctx, run, generation)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan, err := buildInitialControlAwareCoordinatorPlan(
		ctx,
		run,
		generation,
		resolved,
		effective,
		r.gateEvaluator,
		r.store,
		r.controls,
		r.runtimeCatalog,
		fanOutWidth,
		r.watchRuntime(),
		r.watchEventsRuntime(),
		history,
		r.now().UTC(),
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	r.dispatchGenerationPost(ctx, taskRun, run, intent)
	return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
}

func (r *CoordinatorRunner) buildExistingGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	fanOutWidth int,
) (task.CoordinatorCompletionPlan, bool, error) {
	if run.Generation <= 0 {
		return task.CoordinatorCompletionPlan{}, false, nil
	}
	outputs, err := r.outputs.ListGenerationOutputs(ctx, run.WorkspaceID, run.ID, run.Generation)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	if len(outputs) == 0 {
		return task.CoordinatorCompletionPlan{}, false, nil
	}
	cancellation, err := r.prepareCoordinatorCancellation(ctx, run, outputs)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	if cancellation.waitingDelivery {
		return cancellationWaitCoordinatorPlan(run, cancellation.outputs), true, nil
	}
	if run.CancelRequested {
		plan, err := runCancellationCoordinatorPlan(run, cancellation)
		return plan, true, err
	}
	outputs = cancellation.outputs
	if plan, pending, planErr := r.buildPendingRequeuePlan(
		ctx,
		taskRun,
		run,
		resolved.Definition.Graph,
		outputs,
	); pending || planErr != nil {
		return plan, pending, planErr
	}
	outputs, err = generationOutputRuntimeView(ctx, r.outputs, generationOutputRuntimeScope{
		workspaceID: run.WorkspaceID,
		runID:       run.ID,
		generation:  run.Generation,
	}, outputs)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	plan, err := r.buildGenerationFinisherPlan(
		ctx, taskRun, run, run.Generation, resolved, effective, fanOutWidth, outputs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	plan, err = appendCoordinatorCancellationState(plan, cancellation)
	return plan, true, err
}
