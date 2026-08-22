package globaldb

import (
	"context"

	"github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

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
		if err := g.applyCoordinatorGenerationSnapshotIntentsWithExecutor(
			ctx,
			exec,
			loopRun,
			*postReserveSnapshot,
			completion.Now,
		); err != nil {
			return err
		}
		quarantinedContinuations, err := quarantinedLoopContinuationOutputs(postReserveSnapshot)
		if err != nil {
			return err
		}
		if len(quarantinedContinuations) == 0 || len(completion.Plan.NodeTasks) > 0 {
			snapshot = *postReserveSnapshot
		}
	}
	result.EnqueuedRuns = enqueued
	return applyCoordinatorYieldBoundary(ctx, exec, snapshot, result)
}
