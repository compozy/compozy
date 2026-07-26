package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
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
				normalized,
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
	completion taskpkg.CoordinatorCompletion,
	finalizer taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	current, loopRunID, err := g.prepareCoordinatorCompletionWithExecutor(ctx, exec, completion)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := g.ensureCoordinatorPlanTasksWithExecutor(
		ctx,
		exec,
		completion.Plan,
		current,
		completion.Actor.Origin,
		completion.Now,
	); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := completeCoordinatorRunWithExecutor(ctx, exec, current, completion.Now); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := resetTaskBlockRecurrencesWithExecutor(ctx, exec, current.TaskID); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}

	snapshot := completion.Plan.Snapshot
	if strings.TrimSpace(snapshot.LoopRunID) == "" {
		snapshot.LoopRunID = loopRunID
	}
	postReserveSnapshot := normalizePostReserveSnapshot(completion.Plan.PostReserveSnapshot, snapshot, loopRunID)
	if err := finalizer.WriteGenerationSnapshot(ctx, exec, snapshot); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := applyCoordinatorRunStopsWithExecutor(ctx, exec, completion.Plan.RunStops, completion.Now); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}

	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(loopRunID))
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if err := appendLoopGenerationStartedEventWithExecutor(
		ctx,
		exec,
		loopRun,
		snapshot.Generation,
		completion.Now,
	); err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}

	result, err := g.applyCoordinatorBoundaryWithExecutor(
		ctx,
		exec,
		&completion,
		&current,
		snapshot,
		postReserveSnapshot,
		finalizer,
		loopRun,
		tokensUsed,
	)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}

	updated, err := g.getTaskRunWithExecutor(ctx, exec, current.ID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result.Run = updated
	return g.attachTerminalCoordinatorSettlementWithExecutor(ctx, exec, completion, &result, updated)
}

func normalizePostReserveSnapshot(
	postReserveSnapshot *taskpkg.GenerationSnapshot,
	base taskpkg.GenerationSnapshot,
	loopRunID string,
) *taskpkg.GenerationSnapshot {
	if postReserveSnapshot == nil {
		return nil
	}
	snapshot := *postReserveSnapshot
	if strings.TrimSpace(snapshot.LoopRunID) == "" {
		snapshot.LoopRunID = loopRunID
	}
	if snapshot.Generation <= 0 {
		snapshot.Generation = base.Generation
	}
	return &snapshot
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
	snapshot taskpkg.GenerationSnapshot,
	postReserveSnapshot *taskpkg.GenerationSnapshot,
	finalizer taskpkg.GenerationStateFinalizer,
	loopRun loop.Run,
	tokensUsed int64,
) (taskpkg.CoordinatorCompletionResult, error) {
	contextPayload, err := coordinatorResultContext(loopRun, loopRun.Generation, loopRun.Status)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	result := taskpkg.CoordinatorCompletionResult{
		LoopRunID:  strings.TrimSpace(snapshot.LoopRunID),
		Context:    contextPayload,
		TokensUsed: tokensUsed,
	}
	budgetExceeded := loopBudgetExceeded(loopRun, tokensUsed, completion.Now)
	switch {
	case loopRun.Status == loop.StatusWatching && completion.Plan.Terminal != nil:
		err := applyCoordinatorTerminalBoundary(ctx, exec, completion, snapshot, loopRun, &result)
		return result, err
	case loopRun.Status == loop.StatusWatching && coordinatorPlanHasContinuation(completion.Plan):
		err := g.applyCoordinatorWatchReadyBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			loopRun,
			&result,
		)
		return result, err
	case loopRun.Status != loop.StatusRunning:
		err := applyCoordinatorYieldBoundary(ctx, exec, snapshot, &result)
		return result, err
	case shouldDeferCoordinatorBoundary(completion.Plan, loopRun, budgetExceeded):
		err := applyCoordinatorYieldBoundary(ctx, exec, snapshot, &result)
		return result, err
	case shouldApplyCoordinatorBudgetExceededBoundary(completion.Plan, budgetExceeded):
		err := g.applyCoordinatorBudgetExceededBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			loopRun,
			&result,
		)
		return result, err
	case completion.Plan.Terminal != nil:
		err := applyCoordinatorTerminalBoundary(ctx, exec, completion, snapshot, loopRun, &result)
		return result, err
	case completion.Plan.Yield:
		err := applyCoordinatorYieldBoundary(ctx, exec, snapshot, &result)
		return result, err
	case loopRun.PauseRequested && loopRun.Status == loop.StatusRunning:
		err := applyCoordinatorPauseBoundary(ctx, exec, completion, snapshot, loopRun, &result)
		return result, err
	default:
		err := g.applyCoordinatorContinueBoundaryWithExecutor(
			ctx,
			exec,
			completion,
			current,
			snapshot,
			postReserveSnapshot,
			finalizer,
			&result,
		)
		return result, err
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

type coordinatorResultContextPayload struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"loop_name,omitempty"`
	ParentRunID string `json:"parent_loop_run_id,omitempty"`
	Generation  int    `json:"generation,omitempty"`
	Status      string `json:"status,omitempty"`
}

func coordinatorResultContext(
	run loop.Run,
	generation int,
	status loop.Status,
) (json.RawMessage, error) {
	payload := coordinatorResultContextPayload{
		WorkspaceID: string(run.WorkspaceID),
		Name:        strings.TrimSpace(run.LoopName),
		ParentRunID: string(run.ParentLoopRunID),
		Generation:  generation,
		Status:      string(status),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("store: marshal coordinator context: %w", err)
	}
	return data, nil
}

func updateCoordinatorResultContext(
	result *taskpkg.CoordinatorCompletionResult,
	generation int,
	status loop.Status,
) error {
	var payload coordinatorResultContextPayload
	if len(result.Context) > 0 {
		if err := json.Unmarshal(result.Context, &payload); err != nil {
			return fmt.Errorf("store: decode coordinator context: %w", err)
		}
	}
	if generation > 0 {
		payload.Generation = generation
	}
	if strings.TrimSpace(string(status)) != "" {
		payload.Status = string(status)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("store: marshal coordinator context: %w", err)
	}
	result.Context = data
	return nil
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
		result,
	)
}
