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
		"success", progressInfo.SuccessCount,
		"failed", progressInfo.FailedCount,
		"canceled", progressInfo.CanceledCount,
		"timed_out", progressInfo.TimedOutCount,
		"terminal", progressInfo.TerminalCount,
		"running", progressInfo.RunningCount,
		"status_counts", progressInfo.StatusCounts)

	// Check if NO children have reached a terminal state (actual race condition)
	// This happens when child task status updates haven't propagated to the DB yet
	if progressInfo.TerminalCount == 0 && progressInfo.TotalChildren > 0 {
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
	errorMsg.WriteString(
		fmt.Sprintf(
			"%s task failed: success=%d, failed=%d, canceled=%d, timed_out=%d, terminal=%d/%d, status_counts=%v",
			expectedType,
			progressInfo.SuccessCount,
			progressInfo.FailedCount,
			progressInfo.CanceledCount,
			progressInfo.TimedOutCount,
			progressInfo.TerminalCount,
			progressInfo.TotalChildren,
			progressInfo.StatusCounts,
		),
	)

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

// aggregateChildOutputs handles output aggregation for collection and parallel tasks
func aggregateChildOutputs(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	progressInfo *task.ProgressInfo,
	taskType task.Type,
) error {
	log := logger.FromContext(ctx)
	childOutputs, err := taskRepo.ListChildrenOutputs(ctx, parentState.TaskExecID)
	if err != nil {
		log.Error("Failed to aggregate child outputs",
			"error", err,
			"parent_id", parentState.TaskExecID,
			"task_type", taskType)
		return fmt.Errorf("failed to aggregate child outputs: %w", err)
	}
	// Convert to map[string]any
	outputsMap := make(map[string]any, len(childOutputs))
	for taskID, output := range childOutputs {
		if output != nil {
			outputsMap[taskID] = *output
		}
	}
	// Race condition check: ensure successful children have outputs
	// We only check successful tasks because failed tasks might legitimately have no output
	if len(outputsMap) < progressInfo.SuccessCount {
		log.Warn("Child outputs not yet visible for successful tasks, retrying",
			"parent_id", parentState.TaskExecID,
			"expected_success_outputs", progressInfo.SuccessCount,
			"actual_outputs", len(outputsMap),
			"terminal_count", progressInfo.TerminalCount)
		return fmt.Errorf(
			"%s child outputs not yet visible for taskExecID %s: have %d outputs but %d successful tasks",
			taskType,
			parentState.TaskExecID,
			len(outputsMap),
			progressInfo.SuccessCount,
		)
	}
	// Store under "outputs" key to avoid collisions
	(*parentState.Output)["outputs"] = outputsMap
	log.Debug("Aggregated child outputs",
		"parent_id", parentState.TaskExecID,
		"child_count", len(outputsMap))
	return nil
}

// processParentTask is a generalized helper for both parallel and collection tasks.
func processParentTask(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	taskConfig *task.Config,
	expectedType task.Type,
) error {
	if err := validateTaskType(taskConfig, expectedType); err != nil {
		return err
	}
	progressInfo, err := fetchAndValidateProgressInfo(ctx, taskRepo, parentState, expectedType)
	if err != nil {
		return err
	}
	overallStatus := calculateParentStatus(ctx, progressInfo, taskConfig, parentState)
	if err := updateParentOutput(ctx, taskRepo, parentState, progressInfo, taskConfig); err != nil {
		return err
	}
	return handleParentTaskCompletion(ctx, taskRepo, progressInfo, parentState, taskConfig, expectedType, overallStatus)
}

// validateTaskType ensures the task config matches the expected type
func validateTaskType(taskConfig *task.Config, expectedType task.Type) error {
	if taskConfig.Type != expectedType {
		return fmt.Errorf("expected %s task type, got: %s", expectedType, taskConfig.Type)
	}
	return nil
}

// fetchAndValidateProgressInfo retrieves and validates progress information for the parent task
func fetchAndValidateProgressInfo(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	expectedType task.Type,
) (*task.ProgressInfo, error) {
	log := logger.FromContext(ctx)
	log.Debug("Processing parent task",
		"task_type", expectedType,
		"parent_id", parentState.TaskExecID)
	progressInfo, err := taskRepo.GetProgressInfo(ctx, parentState.TaskExecID)
	if err != nil {
		log.Error("Failed to get progress info", "parent_id", parentState.TaskExecID, "error", err)
		return nil, core.NewError(
			err,
			"PROGRESS_INFO_FETCH_FAILED",
			map[string]any{"task_exec_id": parentState.TaskExecID},
		)
	}
	return progressInfo, validateAndLogProgress(ctx, progressInfo, parentState, expectedType)
}

// calculateParentStatus determines the overall status for the parent task
func calculateParentStatus(
	ctx context.Context,
	progressInfo *task.ProgressInfo,
	taskConfig *task.Config,
	parentState *task.State,
) core.StatusType {
	log := logger.FromContext(ctx)
	strategy := taskConfig.GetStrategy()
	overallStatus := progressInfo.CalculateOverallStatus(strategy)
	parentState.Status = overallStatus
	log.Debug("Parent status calculated",
		"parent_id", parentState.TaskExecID,
		"strategy", strategy,
		"overall_status", overallStatus)
	return overallStatus
}

// updateParentOutput updates the parent task output with progress info and child outputs
func updateParentOutput(
	ctx context.Context,
	taskRepo task.Repository,
	parentState *task.State,
	progressInfo *task.ProgressInfo,
	taskConfig *task.Config,
) error {
	if parentState.Output == nil {
		output := make(core.Output)
		parentState.Output = &output
	}
	(*parentState.Output)["progress_info"] = progressInfo
	if taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel {
		return aggregateChildOutputs(ctx, taskRepo, parentState, progressInfo, taskConfig.Type)
	}
	return nil
}

// handleParentTaskCompletion handles the final completion logic for parent tasks
func handleParentTaskCompletion(
	ctx context.Context,
	taskRepo task.Repository,
	progressInfo *task.ProgressInfo,
	parentState *task.State,
	taskConfig *task.Config,
	expectedType task.Type,
	overallStatus core.StatusType,
) error {
	log := logger.FromContext(ctx)
	if overallStatus == core.StatusFailed {
		return buildDetailedFailureError(ctx, taskRepo, progressInfo, parentState, taskConfig, expectedType)
	}
	log.Debug("Parent task processed successfully",
		"parent_id", parentState.TaskExecID,
		"final_status", overallStatus)
	return nil
}
