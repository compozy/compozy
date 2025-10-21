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

const GetParallelResponseLabel = "GetParallelResponse"

type GetParallelResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

// GetParallelResponse handles parallel response using task2 integration
type GetParallelResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	task2Factory task2.Factory
	configStore  services.ConfigStore
	cwd          *core.PathCWD
}

// NewGetParallelResponse creates a new GetParallelResponse activity with task2 integration
func NewGetParallelResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
	cwd *core.PathCWD,
) *GetParallelResponse {
	return &GetParallelResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		task2Factory: task2Factory,
		configStore:  configStore,
		cwd:          cwd,
	}
}

func (a *GetParallelResponse) Run(
	ctx context.Context,
	input *GetParallelResponseInput,
) (*task.MainTaskResponse, error) {
	taskConfig, err := a.loadParentTaskConfig(ctx, input)
	if err != nil {
		return nil, err
	}
	workflowState, err := a.loadWorkflowState(ctx, input)
	if err != nil {
		return nil, err
	}
	executionError := a.processParallelTask(ctx, input, taskConfig)
	handler, err := a.createParallelResponseHandler(ctx)
	if err != nil {
		return nil, err
	}
	responseInput := buildParallelResponseInput(taskConfig, input, workflowState)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle parallel response: %w", err)
	}
	if err := a.applyDeferredTransformation(ctx, handler, responseInput, executionError, input.ParentState); err != nil {
		return nil, err
	}
	if executionError != nil {
		return nil, executionError
	}
	return a.convertToMainTaskResponse(ctx, result), nil
}

// loadParentTaskConfig retrieves the parent task configuration from the config store.
func (a *GetParallelResponse) loadParentTaskConfig(
	ctx context.Context,
	input *GetParallelResponseInput,
) (*task.Config, error) {
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config for exec %s not found", input.ParentState.TaskExecID)
	}
	return taskConfig, nil
}

// loadWorkflowState fetches the workflow state required for response handling.
func (a *GetParallelResponse) loadWorkflowState(
	ctx context.Context,
	input *GetParallelResponseInput,
) (*workflow.State, error) {
	workflowState, err := a.workflowRepo.GetState(ctx, input.ParentState.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	return workflowState, nil
}

// createParallelResponseHandler returns the response handler for parallel tasks.
func (a *GetParallelResponse) createParallelResponseHandler(ctx context.Context) (shared.TaskResponseHandler, error) {
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeParallel)
	if err != nil {
		return nil, fmt.Errorf("failed to create parallel response handler: %w", err)
	}
	return handler, nil
}

// buildParallelResponseInput assembles the input for the response handler invocation.
func buildParallelResponseInput(
	taskConfig *task.Config,
	input *GetParallelResponseInput,
	workflowState *workflow.State,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		WorkflowState:  workflowState,
	}
}

// applyDeferredTransformation runs post-processing when parallel handlers expose it.
func (a *GetParallelResponse) applyDeferredTransformation(
	ctx context.Context,
	handler shared.TaskResponseHandler,
	responseInput *shared.ResponseInput,
	executionError error,
	parentState *task.State,
) error {
	parallelHandler, ok := handler.(interface {
		ApplyDeferredOutputTransformation(context.Context, *shared.ResponseInput) error
	})
	if !ok {
		return nil
	}
	if executionError != nil || parentState.Status == core.StatusFailed {
		return nil
	}
	if err := parallelHandler.ApplyDeferredOutputTransformation(ctx, responseInput); err != nil {
		return fmt.Errorf("failed to apply deferred output transformation: %w", err)
	}
	return nil
}

// processParallelTask handles parallel task processing logic and returns execution error if any
func (a *GetParallelResponse) processParallelTask(
	ctx context.Context,
	input *GetParallelResponseInput,
	taskConfig *task.Config,
) error {
	return processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeParallel)
}

// convertToMainTaskResponse converts shared.ResponseOutput to task.MainTaskResponse
func (a *GetParallelResponse) convertToMainTaskResponse(
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
			metadata, err := configRepo.LoadParallelMetadata(ctx, result.State.TaskExecID)
			if err == nil && metadata != nil {
				if parallelMetadata, ok := metadata.(*task2core.ParallelTaskMetadata); ok {
					if result.State.Output == nil {
						output := make(core.Output)
						result.State.Output = &output
					}
					(*result.State.Output)["parallel_metadata"] = map[string]any{
						"child_count": len(parallelMetadata.ChildConfigs),
						"strategy":    parallelMetadata.Strategy,
						"max_workers": parallelMetadata.MaxWorkers,
					}
				}
			}
		}
	}
	return mainTaskResponse
}
