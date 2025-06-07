package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const GetParallelResponseLabel = "GetParallelResponse"

type GetParallelResponseInput struct {
	ParentState    *task.State             `json:"parent_state"`
	Results        []*task.SubtaskResponse `json:"results"`
	WorkflowConfig *workflow.Config        `json:"workflow_config"`
	TaskConfig     *task.Config            `json:"task_config"`
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
	// Update all subtask states in memory first
	for i := range input.Results {
		result := input.Results[i]
		// Skip nil results (defensive programming)
		if result == nil {
			continue
		}
		taskID := result.TaskID
		status := result.Status
		output := result.Output
		coreError := result.Error
		// Update the subtask state in the parent parallel state
		_, err := input.ParentState.UpdateSubtaskState(taskID, status, output, coreError)
		if err != nil {
			return nil, fmt.Errorf("failed to update subtask state: %w", err)
		}
	}

	// Check if the parallel task should fail according to strategy
	executionError := input.ParentState.IsParallelFailed()
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
