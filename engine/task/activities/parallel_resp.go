package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
)

const GetParallelResponseLabel = "GetParallelResponse"

type GetParallelResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type GetParallelResponse struct {
	taskRepo      task.Repository
	taskResponder *services.TaskResponder
}

// NewGetParallelResponse creates a new GetParallelResponse activity
func NewGetParallelResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *GetParallelResponse {
	return &GetParallelResponse{
		taskRepo:      taskRepo,
		taskResponder: services.NewTaskResponder(workflowRepo, taskRepo),
	}
}

func (a *GetParallelResponse) Run(
	ctx context.Context,
	input *GetParallelResponseInput,
) (*task.MainTaskResponse, error) {
	executionError := a.processParallelTask(ctx, input)

	// Handle main task response
	response, err := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskState:      input.ParentState,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, err
	}

	// If there was an execution error, the parallel task should be considered failed
	if executionError != nil {
		return response, executionError
	}

	return response, nil
}

// processParallelTask handles parallel task processing logic and returns execution error if any
func (a *GetParallelResponse) processParallelTask(ctx context.Context, input *GetParallelResponseInput) error {
	if input.TaskConfig.Type != task.TaskTypeParallel {
		return fmt.Errorf("expected parallel task type, got: %s", input.TaskConfig.Type)
	}
	progressInfo, err := a.taskRepo.GetProgressInfo(ctx, input.ParentState.TaskExecID)
	if err != nil {
		return fmt.Errorf("failed to get progress info: %w", err)
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
			"parallel task failed: completed=%d, failed=%d, total=%d, status_counts=%v",
			progressInfo.CompletedCount, progressInfo.FailedCount,
			progressInfo.TotalChildren, progressInfo.StatusCounts)
	}
	return nil
}
