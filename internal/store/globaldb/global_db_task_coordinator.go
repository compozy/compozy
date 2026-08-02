package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	loopCoordinatorActorRef = "loop"
	taskRunResultKindKey    = "kind"
)

// CompleteCoordinatorAndEnqueueNext applies a coordinator plan in one BEGIN IMMEDIATE transaction.
func (g *TaskRepo) CompleteCoordinatorAndEnqueueNext(
	ctx context.Context,
	completion taskpkg.CoordinatorCompletion,
	finalizer taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	if err := g.checkReady(ctx, "complete coordinator and enqueue next"); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if finalizer == nil {
		return taskpkg.CoordinatorCompletionResult{}, fmt.Errorf(
			"%w: generation state finalizer is required",
			taskpkg.ErrValidation,
		)
	}
	normalized, err := completion.Normalize(g.now())
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}

	var result taskpkg.CoordinatorCompletionResult
	if err := g.withTaskImmediateTransaction(
		ctx,
		"complete coordinator and enqueue next",
		func(exec taskSQLExecutor) error {
			applied, applyErr := g.completeCoordinatorAndEnqueueNextWithExecutor(
				ctx,
				exec,
				&normalized,
				finalizer,
			)
			if applyErr != nil {
				return applyErr
			}
			result = applied
			return nil
		},
	); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := g.enqueueCoordinatorPostCommitWakes(ctx, normalized); err != nil {
		return result, err
	}
	return result, nil
}

func (g *TaskRepo) enqueueCoordinatorPostCommitWakes(
	ctx context.Context,
	completion taskpkg.CoordinatorCompletion,
) error {
	for _, wake := range completion.Plan.PostCommitWakes {
		normalized := wake.Normalize()
		if _, _, err := g.enqueueLoopCoordinatorWake(
			ctx,
			normalized.LoopRunID,
			normalized.IdempotencyKey,
			completion.Actor.Origin,
			completion.Now,
		); err != nil {
			return fmt.Errorf("store: enqueue coordinator post-commit wake %q: %w", normalized.LoopRunID, err)
		}
	}
	return nil
}

func (g *TaskRepo) completeCoordinatorAndEnqueueNextWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	finalizer taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	current, loopRunID, err := g.prepareCoordinatorCompletionWithExecutor(ctx, exec, *completion)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	superseded, err := protectCanceledLoopFromStaleCoordinator(ctx, exec, completion, loopRunID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	state, err := g.finalizeCoordinatorGenerationWithExecutor(
		ctx,
		exec,
		*completion,
		current,
		loopRunID,
		finalizer,
	)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result, err := g.applyCoordinatorBoundaryWithExecutor(
		ctx,
		exec,
		completion,
		&current,
		&state,
		finalizer,
	)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result.PlanSuperseded = superseded

	updated, err := g.getTaskRunWithExecutor(ctx, exec, current.ID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result.Run = updated
	return g.attachTerminalCoordinatorSettlementWithExecutor(ctx, exec, *completion, &result, updated)
}

type coordinatorBoundaryState struct {
	snapshot            taskpkg.GenerationSnapshot
	postReserveSnapshot *taskpkg.GenerationSnapshot
	loopRun             loop.Run
	tokensUsed          int64
}

func (g *TaskRepo) finalizeCoordinatorGenerationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion taskpkg.CoordinatorCompletion,
	current taskpkg.Run,
	loopRunID string,
	finalizer taskpkg.GenerationStateFinalizer,
) (coordinatorBoundaryState, error) {
	if err := validateCoordinatorSnapshotLoopIdentity(completion.Plan, loopRunID); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := g.ensureCoordinatorPlanTasksWithExecutor(
		ctx,
		exec,
		completion.Plan,
		current,
		completion.Actor.Origin,
		completion.Now,
	); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := completeCoordinatorRunWithExecutor(ctx, exec, current, completion.Now); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := resetTaskBlockRecurrencesWithExecutor(ctx, exec, current.TaskID); err != nil {
		return coordinatorBoundaryState{}, err
	}

	snapshot := completion.Plan.Snapshot
	if strings.TrimSpace(snapshot.LoopRunID) == "" {
		snapshot.LoopRunID = loopRunID
	}
	postReserve := normalizePostReserveSnapshot(completion.Plan.PostReserveSnapshot, snapshot, loopRunID)
	if err := finalizer.WriteGenerationSnapshot(ctx, exec, snapshot); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := applyCoordinatorRunStopsWithExecutor(ctx, exec, completion.Plan.RunStops, completion.Now); err != nil {
		return coordinatorBoundaryState{}, err
	}
	if err := validateCoordinatorParentClosesWithExecutor(
		ctx, exec, loop.RunID(loopRunID), completion.Plan,
	); err != nil {
		return coordinatorBoundaryState{}, err
	}
	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return coordinatorBoundaryState{}, err
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(loopRunID))
	if err != nil {
		return coordinatorBoundaryState{}, err
	}
	if snapshot.Generation > loopRun.Generation {
		if err := updateLoopGenerationWithExecutor(ctx, exec, loopRunID, snapshot.Generation); err != nil {
			return coordinatorBoundaryState{}, err
		}
	}
	if err := applyCoordinatorGenerationSnapshotIntentsWithExecutor(
		ctx,
		exec,
		loopRun,
		snapshot,
		completion.Now,
	); err != nil {
		return coordinatorBoundaryState{}, err
	}
	return coordinatorBoundaryState{
		snapshot:            snapshot,
		postReserveSnapshot: postReserve,
		loopRun:             loopRun,
		tokensUsed:          tokensUsed,
	}, nil
}

func (g *TaskRepo) prepareCoordinatorCompletionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion taskpkg.CoordinatorCompletion,
) (taskpkg.Run, string, error) {
	current, err := g.getTaskRunWithExecutor(ctx, exec, completion.RunID)
	if err != nil {
		return taskpkg.Run{}, "", err
	}
	if current.RunKind.Normalize() != taskpkg.RunKindCoordinator {
		return taskpkg.Run{}, "", fmt.Errorf(
			"%w: task run %q is %q, not %q",
			taskpkg.ErrInvalidStatusTransition,
			current.ID,
			current.RunKind.Normalize(),
			taskpkg.RunKindCoordinator,
		)
	}
	loopRunID := strings.TrimSpace(current.LoopRunID)
	if loopRunID == "" {
		return taskpkg.Run{}, "", fmt.Errorf(
			"%w: coordinator run %q has no loop_run_id",
			taskpkg.ErrValidation,
			current.ID,
		)
	}
	if err := requireCurrentRunLease(current, completion.ClaimToken, completion.Now); err != nil {
		return taskpkg.Run{}, "", err
	}
	if err := requireLeaseTerminalTransition(current, taskpkg.TaskRunStatusCompleted); err != nil {
		return taskpkg.Run{}, "", err
	}
	return current, loopRunID, nil
}

func (g *TaskRepo) applyCoordinatorBoundaryWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	current *taskpkg.Run,
	state *coordinatorBoundaryState,
	finalizer taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	snapshot := state.snapshot
	loopRun := state.loopRun
	contextPayload, err := coordinatorResultContext(loopRun, loopRun.Generation, loopRun.Status)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result := taskpkg.CoordinatorCompletionResult{
		LoopRunID:  strings.TrimSpace(snapshot.LoopRunID),
		Context:    contextPayload,
		TokensUsed: state.tokensUsed,
	}
	err = g.dispatchCoordinatorBoundaryWithExecutor(ctx, exec, completion, current, state, finalizer, &result)
	return result, err
}

func (g *TaskRepo) dispatchCoordinatorBoundaryWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	current *taskpkg.Run,
	state *coordinatorBoundaryState,
	finalizer taskpkg.GenerationStateFinalizer,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	snapshot := state.snapshot
	postReserveSnapshot := state.postReserveSnapshot
	loopRun := state.loopRun
	budgetExceeded := loopBudgetExceeded(
		loopRun, state.tokensUsed, completion.Now, coordinatorPlanHasParkedOutputs(completion.Plan),
	)
	switch {
	case loopRun.Status == loop.StatusWatching && completion.Plan.Terminal != nil:
		return applyCoordinatorTerminalBoundary(ctx, exec, completion, snapshot, loopRun, result)
	case loopRun.Status == loop.StatusWatching && coordinatorPlanHasContinuation(completion.Plan):
		return g.applyCoordinatorWatchReadyBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			loopRun,
			result,
		)
	case loopRun.Status != loop.StatusRunning:
		return applyCoordinatorYieldBoundary(ctx, exec, snapshot, result)
	case shouldDeferCoordinatorBoundary(completion.Plan, loopRun, budgetExceeded):
		return applyCoordinatorYieldBoundary(ctx, exec, snapshot, result)
	case shouldApplyCoordinatorBudgetExceededBoundary(completion.Plan, budgetExceeded):
		return g.applyCoordinatorBudgetExceededBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			loopRun,
			result,
		)
	case completion.Plan.Terminal != nil:
		return applyCoordinatorTerminalBoundary(ctx, exec, completion, snapshot, loopRun, result)
	case completion.Plan.Yield:
		return applyCoordinatorYieldBoundary(ctx, exec, snapshot, result)
	case loopRun.PauseRequested && loopRun.Status == loop.StatusRunning:
		return applyCoordinatorPauseBoundary(ctx, exec, completion, snapshot, loopRun, result)
	default:
		return g.applyCoordinatorContinueBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			loopRun,
			result,
		)
	}
}

func shouldApplyCoordinatorBudgetExceededBoundary(
	plan taskpkg.CoordinatorCompletionPlan,
	budgetExceeded bool,
) bool {
	return !plan.Yield && plan.Terminal == nil && budgetExceeded
}

func coordinatorPlanHasContinuation(plan taskpkg.CoordinatorCompletionPlan) bool {
	return len(plan.NodeRuns) > 0 || plan.NextCoordinator != nil
}

func shouldDeferCoordinatorBoundary(
	plan taskpkg.CoordinatorCompletionPlan,
	loopRun loop.Run,
	budgetExceeded bool,
) bool {
	if !plan.GenerationInFlight {
		return false
	}
	return plan.Terminal != nil || budgetExceeded || loopRun.PauseRequested
}

func applyCoordinatorYieldBoundary(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot taskpkg.GenerationSnapshot,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	if snapshot.Generation > 0 {
		if err := updateLoopGenerationWithExecutor(ctx, exec, result.LoopRunID, snapshot.Generation); err != nil {
			return err
		}
		if err := updateCoordinatorResultContext(result, snapshot.Generation, ""); err != nil {
			return err
		}
	}
	return nil
}

func applyCoordinatorPauseBoundary(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	snapshot taskpkg.GenerationSnapshot,
	loopRun loop.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	if err := updateLoopBoundaryStatusWithExecutor(
		ctx,
		exec,
		loopRun,
		loop.StatusPaused,
		loop.TransitionCausePauseBoundary,
		completion.Now,
		snapshot.Generation,
	); err != nil {
		return err
	}
	if err := updateCoordinatorResultContext(result, snapshot.Generation, loop.StatusPaused); err != nil {
		return err
	}
	result.Paused = true
	return nil
}

func (g *TaskRepo) applyCoordinatorContinueBoundaryWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	current *taskpkg.Run,
	snapshot taskpkg.GenerationSnapshot,
	postReserveSnapshot *taskpkg.GenerationSnapshot,
	finalizer taskpkg.GenerationStateFinalizer,
	loopRun loop.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	enqueued, err := g.reserveCoordinatorPlanRunsWithExecutor(
		ctx,
		exec,
		completion.Plan,
		*current,
		completion.Actor.Origin,
		completion.Now,
	)
	if err != nil {
		return err
	}
	if postReserveSnapshot != nil {
		if err := finalizer.WriteGenerationSnapshot(ctx, exec, *postReserveSnapshot); err != nil {
			return err
		}
		if err := applyCoordinatorGenerationSnapshotIntentsWithExecutor(
			ctx,
			exec,
			loopRun,
			*postReserveSnapshot,
			completion.Now,
		); err != nil {
			return err
		}
		snapshot = *postReserveSnapshot
	}
	result.EnqueuedRuns = enqueued
	return applyCoordinatorYieldBoundary(ctx, exec, snapshot, result)
}

func (g *TaskRepo) applyCoordinatorWatchReadyBoundaryWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	current *taskpkg.Run,
	snapshot taskpkg.GenerationSnapshot,
	postReserveSnapshot *taskpkg.GenerationSnapshot,
	finalizer taskpkg.GenerationStateFinalizer,
	loopRun loop.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	if err := updateLoopBoundaryStatusWithExecutor(
		ctx,
		exec,
		loopRun,
		loop.StatusRunning,
		loop.TransitionCauseWatchPoll,
		completion.Now,
		snapshot.Generation,
	); err != nil {
		return err
	}
	if err := updateCoordinatorResultContext(result, snapshot.Generation, loop.StatusRunning); err != nil {
		return err
	}
	return g.applyCoordinatorContinueBoundaryWithExecutor(
		ctx,
		exec,
		completion,
		current,
		snapshot,
		postReserveSnapshot,
		finalizer,
		loopRun,
		result,
	)
}
