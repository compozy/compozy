package globaldb

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRepo) attachTerminalCoordinatorSettlementWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion taskpkg.CoordinatorCompletion,
	result *taskpkg.CoordinatorCompletionResult,
	updated taskpkg.Run,
) (taskpkg.CoordinatorCompletionResult, error) {
	if result.Terminal && len(result.EnqueuedRuns) == 0 {
		settlement, err := g.settleCompletedTaskHierarchyWithExecutor(
			ctx,
			exec,
			updated.TaskID,
			completion.Actor,
			completion.Now,
		)
		if err != nil {
			return taskpkg.CoordinatorCompletionResult{}, err
		}
		settlement.Run = updated
		result.Settlement = &settlement
	}
	return *result, nil
}
