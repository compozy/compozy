package globaldb

import (
	"context"

	taskpkg "github.com/compozy/compozy/internal/task"
)

// attachCoordinatorSettlementResultWithExecutor completes the public result shape only.
// Terminal lifecycle mutation is owned exclusively by settleLoopRunTerminalWithReason.
func (g *TaskRepo) attachCoordinatorSettlementResultWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	result *taskpkg.CoordinatorCompletionResult,
	updated taskpkg.Run,
) (taskpkg.CoordinatorCompletionResult, error) {
	currentTask, err := g.getTaskWithExecutor(ctx, exec, updated.TaskID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	if result.Settlement == nil {
		result.Settlement = &taskpkg.CompletedRunSettlement{}
	}
	result.Settlement.Run = updated
	result.Settlement.Task = currentTask
	return *result, nil
}
