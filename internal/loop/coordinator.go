package loop

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

const (
	generationOutputPending        = "pending"
	generationOutputEnqueued       = "enqueued"
	generationOutputRunning        = "running"
	generationOutputAwaitingChild  = "awaiting_child"
	generationOutputControlPending = GenerationOutputStatusControlPending
	generationOutputAwaitingGoal   = GenerationOutputStatusAwaitingGoal
	generationOutputSucceeded      = "succeeded"
	generationOutputFailed         = "failed"

	childLoopTimeoutReason     = "child_loop_timeout"
	blockingIssuesRepeatedCode = "blocking_issues_repeated"
)

const (
	// GenerationOutputStatusControlPending identifies a completed worker whose Loop control is unsettled.
	GenerationOutputStatusControlPending = "control_pending"
	// GenerationOutputStatusAwaitingGoal identifies a settled pause or approval gate awaiting re-entry.
	GenerationOutputStatusAwaitingGoal = "awaiting_goal"
)

// CoordinatorTaskRunReader is the task-run read seam required by the loop coordinator.
type CoordinatorTaskRunReader interface {
	GetTaskRun(ctx context.Context, id string) (task.Run, error)
}

// GenerationOutputReader reads loop-owned generation output snapshots for coordinator reclaims.
type GenerationOutputReader interface {
	ListGenerationOutputs(
		ctx context.Context,
		runID RunID,
		generation int,
	) ([]GenerationOutput, error)
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
	hooks              HookDispatcher
	gateEvaluator      gate.GateEvaluator
	actionRegistry     *ActionRegistry
	logger             *slog.Logger
	now                func() time.Time
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
		watchSilenceWindow: DefaultWatchSilenceWindow,
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
	coordinatorFSM := newCoordinatorFSM(r.logger, loopRun)
	if err := coordinatorFSM.transition(ctx, coordinatorEventDerive); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	resolved, err := r.resolvePinnedDefinition(ctx, loopRun)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := coordinatorFSM.transition(ctx, coordinatorEventEvaluate); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan, err := r.buildCoordinatorPlan(ctx, taskRun, loopRun, resolved)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := coordinatorFSM.transition(ctx, coordinatorEventAssemble); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := coordinatorFSM.transition(ctx, coordinatorEventYield); err != nil {
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
	if run.Generation > 0 {
		outputs, err := r.outputs.ListGenerationOutputs(ctx, run.ID, run.Generation)
		if err != nil {
			return task.CoordinatorCompletionPlan{}, err
		}
		if len(outputs) > 0 {
			plan, err := r.buildGenerationFinisherPlan(
				ctx,
				taskRun,
				run,
				run.Generation,
				resolved,
				effective,
				fanOutWidth,
				outputs,
			)
			if err != nil {
				return task.CoordinatorCompletionPlan{}, err
			}
			return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
		}
	}
	generation := run.Generation + 1
	if terminal := iterationCapTerminal(run, generation); terminal != nil {
		plan := terminalCoordinatorPlan(run, terminal)
		return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
	}
	if denied, plan := r.dispatchGenerationPre(ctx, taskRun, run, generation); denied {
		return plan, nil
	}
	plan, err := buildInitialControlAwareCoordinatorPlan(
		ctx,
		run,
		generation,
		resolved,
		effective,
		r.gateEvaluator,
		r.store,
		fanOutWidth,
		r.watchRuntime(),
		r.watchEventsRuntime(),
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	r.dispatchGenerationPost(ctx, taskRun, run, plan)
	return r.dispatchGateHooks(ctx, taskRun, run, plan), nil
}
