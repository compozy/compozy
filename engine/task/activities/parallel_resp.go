package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const GetParallelResponseLabel = "GetParallelResponse"

type GetParallelResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type GetParallelResponse struct {
	taskRepo         task.Repository
	handleResponseUC *uc.HandleResponse
}

// NewGetParallelResponse creates a new GetParallelResponse activity
func NewGetParallelResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *GetParallelResponse {
	return &GetParallelResponse{
		taskRepo:         taskRepo,
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
	}
}

func (a *GetParallelResponse) Run(ctx context.Context, input *GetParallelResponseInput) (*task.Response, error) {
	// Get progress information for the parent task using repository-based aggregation
	progressInfo, err := a.taskRepo.GetProgressInfo(ctx, input.ParentState.TaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress info: %w", err)
	}

	// Determine execution error based on parallel strategy and child task statuses
	var executionError error
	if input.TaskConfig.Type == task.TaskTypeParallel {
		strategy := input.TaskConfig.GetStrategy()

		// Calculate overall status using new progress aggregation
		overallStatus := progressInfo.CalculateOverallStatus(strategy)

		// Set execution error if parent task should fail
		if overallStatus == core.StatusFailed {
			executionError = fmt.Errorf("parallel task failed: completed=%d, failed=%d, total=%d",
				progressInfo.CompletedCount, progressInfo.FailedCount, progressInfo.TotalChildren)
		}

		// Update parent state with aggregated information
		input.ParentState.Status = overallStatus

		// Store progress information in task metadata (optional - could be useful for monitoring)
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
	}

	response, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle parallel task response: %w", err)
	}
	return response, nil
}
