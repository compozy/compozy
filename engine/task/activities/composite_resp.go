package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

const GetCompositeResponseLabel = "GetCompositeResponse"

type GetCompositeResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

// GetCompositeResponse handles composite response using task2 integration
type GetCompositeResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	task2Factory task2.Factory
	configStore  services.ConfigStore
	cwd          *core.PathCWD
}

// NewGetCompositeResponse creates a new GetCompositeResponse activity with task2 integration
func NewGetCompositeResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
	cwd *core.PathCWD,
) *GetCompositeResponse {
	return &GetCompositeResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		task2Factory: task2Factory,
		configStore:  configStore,
		cwd:          cwd,
	}
}

func (a *GetCompositeResponse) Run(
	ctx context.Context,
	input *GetCompositeResponseInput,
) (*task.MainTaskResponse, error) {
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config for exec %s not found", input.ParentState.TaskExecID)
	}
	workflowState, err := a.workflowRepo.GetState(ctx, input.ParentState.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	executionError := processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeComposite)
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeComposite)
	if err != nil {
		return nil, fmt.Errorf("failed to create composite response handler: %w", err)
	}
	responseInput := &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		WorkflowState:  workflowState,
	}
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle composite response: %w", err)
	}
	if executionError != nil {
		return nil, fmt.Errorf("composite task failed: %w", executionError)
	}
	mainTaskResponse := a.convertToMainTaskResponse(ctx, result)
	return mainTaskResponse, nil
}

// convertToMainTaskResponse converts shared.ResponseOutput to task.MainTaskResponse
func (a *GetCompositeResponse) convertToMainTaskResponse(
	ctx context.Context,
	result *shared.ResponseOutput,
) *task.MainTaskResponse {
	var mainTaskResponse *task.MainTaskResponse
	if result.Response != nil {
		if mtr, ok := result.Response.(*task.MainTaskResponse); ok {
			mainTaskResponse = mtr
		}
	}
	if mainTaskResponse == nil {
		mainTaskResponse = &task.MainTaskResponse{
			State: result.State,
		}
		configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.cwd)
		if err != nil {
			logger.FromContext(ctx).Error("failed to create task config repository", "error", err)
		} else {
			metadata, err := configRepo.LoadCompositeMetadata(ctx, result.State.TaskExecID)
			if err == nil && metadata != nil {
				if compositeMetadata, ok := metadata.(*task2core.CompositeTaskMetadata); ok {
					if result.State.Output == nil {
						output := make(core.Output)
						result.State.Output = &output
					}
					(*result.State.Output)["composite_metadata"] = map[string]any{
						"child_count": len(compositeMetadata.ChildConfigs),
						"strategy":    compositeMetadata.Strategy,
						"sequential":  true, // Composite tasks are always sequential
					}
				}
			}
		}
	}
	return mainTaskResponse
}
