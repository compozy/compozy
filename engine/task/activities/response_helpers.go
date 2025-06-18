package activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

// getFailedChildDetails retrieves error details from failed child tasks
func getFailedChildDetails(ctx context.Context, taskRepo task.Repository, parentStateID core.ID) ([]string, error) {
	log := logger.FromContext(ctx)

	children, err := taskRepo.ListChildren(ctx, parentStateID)
	if err != nil {
		log.Error("Failed to list children for error details", "parent_id", parentStateID, "error", err)
		return nil, fmt.Errorf("failed to list child tasks: %w", err)
	}

	var failedDetails []string
	for _, child := range children {
		if child.Status == core.StatusFailed {
			errorMsg := "unknown error"
			if child.Error != nil {
				if child.Error.Message != "" {
					errorMsg = child.Error.Message
				} else if child.Error.Code != "" {
					errorMsg = child.Error.Code
				}
			}
			failedDetails = append(failedDetails, fmt.Sprintf("task[%s]: %s", child.TaskID, errorMsg))
		}
	}

	log.Debug("Collected failed child task details",
		"parent_id", parentStateID,
		"failed_count", len(failedDetails),
		"failed_tasks", failedDetails)

	return failedDetails, nil
}

// validateAndLogProgress validates progress info and logs debug information
func validateAndLogProgress(
	ctx context.Context,
	progressInfo *task.ProgressInfo,
	parentState *task.State,
	expectedType task.Type,
) error {
	log := logger.FromContext(ctx)

	log.Debug("Progress info retrieved",
		"parent_id", parentState.TaskExecID,
		"total", progressInfo.TotalChildren,
		"completed", progressInfo.CompletedCount,
		"failed", progressInfo.FailedCount,
		"running", progressInfo.RunningCount,
		"status_counts", progressInfo.StatusCounts)

	// If no child tasks have completed or failed, it might be due to the DB commit race condition.
	// Return a retryable error to let Temporal handle the backoff.
	if progressInfo.CompletedCount == 0 && progressInfo.FailedCount == 0 && progressInfo.TotalChildren > 0 {
		log.Warn("Progress not yet visible, retrying",
			"parent_id", parentState.TaskExecID,
			"total_children", progressInfo.TotalChildren)
		return fmt.Errorf("%s progress not yet visible for taskExecID %s, total children: %d - retrying",
			expectedType, parentState.TaskExecID, progressInfo.TotalChildren)
	}

	return nil
}

// buildDetailedFailureError creates a comprehensive error message with child task details
func buildDetailedFailureError(
	ctx context.Context,
	taskRepo task.Repository,
	progressInfo *task.ProgressInfo,
	parentState *task.State,
	taskConfig *task.Config,
	expectedType task.Type,
) error {
	log := logger.FromContext(ctx)

	// Get detailed error information from failed child tasks
	failedDetails, detailErr := getFailedChildDetails(ctx, taskRepo, parentState.TaskExecID)

	// Create a comprehensive error message
	var errorMsg strings.Builder
	errorMsg.WriteString(fmt.Sprintf("%s task failed: completed=%d, failed=%d, total=%d, status_counts=%v",
		expectedType, progressInfo.CompletedCount, progressInfo.FailedCount,
		progressInfo.TotalChildren, progressInfo.StatusCounts))

	if detailErr != nil {
		log.Error("Failed to get child error details", "parent_id", parentState.TaskExecID, "error", detailErr)
		errorMsg.WriteString(fmt.Sprintf(" (failed to get error details: %v)", detailErr))
	} else if len(failedDetails) > 0 {
		errorMsg.WriteString(fmt.Sprintf(" | Failed tasks: [%s]", strings.Join(failedDetails, "; ")))
	}

	finalError := errorMsg.String()
	log.Error("Parent task failed with detailed errors",
		"parent_id", parentState.TaskExecID,
		"task_id", taskConfig.ID,
		"error", finalError)

	return fmt.Errorf("%s", finalError)
}

// processParentTask is a generalized helper for both parallel and collection tasks.
func processParentTask(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	taskConfig *task.Config,
	expectedType task.Type,
) error {
	log := logger.FromContext(ctx)

	if taskConfig.Type != expectedType {
		return fmt.Errorf("expected %s task type, got: %s", expectedType, taskConfig.Type)
	}

	log.Debug("Processing parent task",
		"task_type", expectedType,
		"parent_id", parentState.TaskExecID,
		"task_id", taskConfig.ID)

	progressInfo, err := taskRepo.GetProgressInfo(ctx, parentState.TaskExecID)
	if err != nil {
		log.Error("Failed to get progress info", "parent_id", parentState.TaskExecID, "error", err)
		return core.NewError(err, "PROGRESS_INFO_FETCH_FAILED", map[string]any{"task_exec_id": parentState.TaskExecID})
	}

	if err := validateAndLogProgress(ctx, progressInfo, parentState, expectedType); err != nil {
		return err
	}

	strategy := taskConfig.GetStrategy()
	overallStatus := progressInfo.CalculateOverallStatus(strategy)
	parentState.Status = overallStatus

	log.Debug("Parent status calculated",
		"parent_id", parentState.TaskExecID,
		"strategy", strategy,
		"overall_status", overallStatus)

	if parentState.Output == nil {
		output := make(core.Output)
		parentState.Output = &output
	}
	// Use the ProgressInfo struct directly for consistent JSON serialization
	(*parentState.Output)["progress_info"] = progressInfo

	if overallStatus == core.StatusFailed {
		return buildDetailedFailureError(ctx, taskRepo, progressInfo, parentState, taskConfig, expectedType)
	}

	log.Debug("Parent task processed successfully",
		"parent_id", parentState.TaskExecID,
		"final_status", overallStatus)

	return nil
}
