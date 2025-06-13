package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// processParentTask is a generalized helper for both parallel and collection tasks.
func processParentTask(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	taskConfig *task.Config,
	expectedType task.Type,
) error {
	if taskConfig.Type != expectedType {
		return fmt.Errorf("expected %s task type, got: %s", expectedType, taskConfig.Type)
	}

	progressInfo, err := taskRepo.GetProgressInfo(ctx, parentState.TaskExecID)
	if err != nil {
		return core.NewError(err, "PROGRESS_INFO_FETCH_FAILED", map[string]any{"task_exec_id": parentState.TaskExecID})
	}

	// If no child tasks have completed or failed, it might be due to the DB commit race condition.
	// Return a retryable error to let Temporal handle the backoff.
	if progressInfo.CompletedCount == 0 && progressInfo.FailedCount == 0 && progressInfo.TotalChildren > 0 {
		return fmt.Errorf("%s progress not yet visible for taskExecID %s, total children: %d - retrying",
			expectedType, parentState.TaskExecID, progressInfo.TotalChildren)
	}

	strategy := taskConfig.GetStrategy()
	overallStatus := progressInfo.CalculateOverallStatus(strategy)
	parentState.Status = overallStatus

	if parentState.Output == nil {
		output := make(core.Output)
		parentState.Output = &output
	}
	// Use the ProgressInfo struct directly for consistent JSON serialization
	(*parentState.Output)["progress_info"] = progressInfo

	if overallStatus == core.StatusFailed {
		return fmt.Errorf(
			"%s task failed: completed=%d, failed=%d, total=%d, status_counts=%v",
			expectedType, progressInfo.CompletedCount, progressInfo.FailedCount,
			progressInfo.TotalChildren, progressInfo.StatusCounts)
	}
	return nil
}
