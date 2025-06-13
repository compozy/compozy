package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
)

const GetParallelResponseLabel = "GetParallelResponse"

type GetParallelResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

type GetParallelResponse struct {
	taskRepo      task.Repository
	taskResponder *services.TaskResponder
	configStore   services.ConfigStore
}

// NewGetParallelResponse creates a new GetParallelResponse activity
func NewGetParallelResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
) *GetParallelResponse {
	return &GetParallelResponse{
		taskRepo:      taskRepo,
		taskResponder: services.NewTaskResponder(workflowRepo, taskRepo),
		configStore:   configStore,
	}
}

func (a *GetParallelResponse) Run(
	ctx context.Context,
	input *GetParallelResponseInput,
) (*task.MainTaskResponse, error) {
	// Fetch task config from Redis using parent state's TaskExecID
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}

	executionError := a.processParallelTask(ctx, input, taskConfig)

	// Handle main task response
	response, err := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskState:      input.ParentState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, err
	}

	// If there was an execution error, the parallel task should be considered failed
	if executionError != nil {
		return nil, executionError
	}

	return response, nil
}

// processParallelTask handles parallel task processing logic and returns execution error if any
func (a *GetParallelResponse) processParallelTask(
	ctx context.Context,
	input *GetParallelResponseInput,
	taskConfig *task.Config,
) error {
	return processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeParallel)
}
