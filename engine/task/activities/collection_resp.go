package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
)

const GetCollectionResponseLabel = "GetCollectionResponse"

type GetCollectionResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type GetCollectionResponse struct {
	taskRepo          task.Repository
	taskResponder     *services.TaskResponder
	maxRetries        int
	initialRetryDelay time.Duration
}

// NewGetCollectionResponse creates a new GetCollectionResponse activity
func NewGetCollectionResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *GetCollectionResponse {
	return &GetCollectionResponse{
		taskRepo:          taskRepo,
		taskResponder:     services.NewTaskResponder(workflowRepo, taskRepo),
		maxRetries:        3,
		initialRetryDelay: 200 * time.Millisecond,
	}
}

// NewGetCollectionResponseWithRetryConfig creates a new GetCollectionResponse activity with custom retry configuration
func NewGetCollectionResponseWithRetryConfig(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	maxRetries int,
	initialRetryDelay time.Duration,
) *GetCollectionResponse {
	return &GetCollectionResponse{
		taskRepo:          taskRepo,
		taskResponder:     services.NewTaskResponder(workflowRepo, taskRepo),
		maxRetries:        maxRetries,
		initialRetryDelay: initialRetryDelay,
	}
}

func (a *GetCollectionResponse) Run(
	ctx context.Context,
	input *GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
	executionError := a.processCollectionTask(ctx, input)

	// Extract collection metadata from parent state
	var itemCount, skippedCount int
	if input.ParentState.Output != nil {
		if metadata, exists := (*input.ParentState.Output)["collection_metadata"]; exists {
			if metadataMap, ok := metadata.(map[string]any); ok {
				if count, ok := metadataMap["item_count"].(int); ok {
					itemCount = count
				}
				if count, ok := metadataMap["skipped_count"].(int); ok {
					skippedCount = count
				}
			}
		}
	}

	// Use TaskResponder to handle the collection response
	return a.taskResponder.HandleCollection(ctx, &services.CollectionResponseInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskState:      input.ParentState,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
		ItemCount:      itemCount,
		SkippedCount:   skippedCount,
	})
}

// processCollectionTask handles collection task processing logic and returns execution error if any
func (a *GetCollectionResponse) processCollectionTask(ctx context.Context, input *GetCollectionResponseInput) error {
	if input.TaskConfig.Type != task.TaskTypeCollection {
		return nil
	}

	// Retry mechanism to handle potential race conditions with database commits
	var progressInfo *task.ProgressInfo
	var err error
	retryDelay := a.initialRetryDelay

	for attempt := 0; attempt < a.maxRetries; attempt++ {
		progressInfo, err = a.taskRepo.GetProgressInfo(ctx, input.ParentState.TaskExecID)
		if err != nil {
			return fmt.Errorf("failed to get progress info: %w", err)
		}

		// If we have meaningful progress (completed or failed tasks) or reached max attempts, break out of retry loop
		// This handles the race condition where parallel tasks haven't committed yet
		if progressInfo.CompletedCount > 0 || progressInfo.FailedCount > 0 || attempt == a.maxRetries-1 {
			break
		}

		// Small delay to allow database transactions to commit
		time.Sleep(retryDelay)
		retryDelay *= 2 // Exponential backoff
	}

	strategy := input.TaskConfig.GetStrategy()
	overallStatus := progressInfo.CalculateOverallStatus(strategy)

	// Update parent state with aggregated information
	input.ParentState.Status = overallStatus

	// Store progress information in task metadata
	if input.ParentState.Output == nil {
		input.ParentState.Output = &core.Output{}
	}
	progressOutput := map[string]any{
		"completion_rate": progressInfo.CompletionRate,
		"failure_rate":    progressInfo.FailureRate,
		"total_children":  progressInfo.TotalChildren,
		"status_counts":   progressInfo.StatusCounts,
	}
	(*input.ParentState.Output)["progress_info"] = progressOutput

	// Return execution error if parent task should fail
	if overallStatus == core.StatusFailed {
		return fmt.Errorf(
			"collection task failed: completed=%d, failed=%d, total=%d, status_counts=%v",
			progressInfo.CompletedCount, progressInfo.FailedCount,
			progressInfo.TotalChildren, progressInfo.StatusCounts)
	}

	return nil
}
