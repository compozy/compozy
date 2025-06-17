package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
)

const GetCompositeResponseLabel = "GetCompositeResponse"

type GetCompositeResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

type GetCompositeResponse struct {
	taskRepo      task.Repository
	taskResponder *services.TaskResponder
	configStore   services.ConfigStore
}

// NewGetCompositeResponse creates a new GetCompositeResponse activity
func NewGetCompositeResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
) *GetCompositeResponse {
	return &GetCompositeResponse{
		taskRepo:      taskRepo,
		taskResponder: services.NewTaskResponder(workflowRepo, taskRepo),
		configStore:   configStore,
	}
}

func (a *GetCompositeResponse) Run(
	ctx context.Context,
	input *GetCompositeResponseInput,
) (*task.MainTaskResponse, error) {
	// Fetch task config from Redis using parent state's TaskExecID
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config for exec %s not found", input.ParentState.TaskExecID)
	}
	executionError := processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeComposite)
	// Handle main task response
	response, err := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskState:      input.ParentState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		// If both errors exist, include execution error in the message
		if executionError != nil {
			return nil, fmt.Errorf("handler error: %w (execution error: %v)", err, executionError)
		}
		return nil, err
	}
	// If there was an execution error, the composite task should be considered failed
	if executionError != nil {
		return nil, fmt.Errorf("composite task failed: %w", executionError)
	}
	return response, nil
}
