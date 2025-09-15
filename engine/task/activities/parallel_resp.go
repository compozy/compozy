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
	// Fetch task config from Redis using parent state's TaskExecID
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config for exec %s not found", input.ParentState.TaskExecID)
	}
	// Load workflow state
	workflowState, err := a.workflowRepo.GetState(ctx, input.ParentState.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	// Process the parallel task
	executionError := a.processParallelTask(ctx, input, taskConfig)
	// Use task2 ResponseHandler for parallel type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeParallel)
	if err != nil {
		return nil, fmt.Errorf("failed to create parallel response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		WorkflowState:  workflowState,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle parallel response: %w", err)
	}

	// Apply deferred output transformation for parallel tasks after children are processed
	// This handles parallel response aggregation with child outputs
	if parallelHandler, ok := handler.(interface {
		ApplyDeferredOutputTransformation(context.Context, *shared.ResponseInput) error
	}); ok {
		// CRITICAL: Only apply transformation if no execution error
		// This matches the condition in BaseResponseHandler.ApplyDeferredOutputTransformation
		if executionError == nil && input.ParentState.Status != core.StatusFailed {
			if err := parallelHandler.ApplyDeferredOutputTransformation(ctx, responseInput); err != nil {
				return nil, fmt.Errorf("failed to apply deferred output transformation: %w", err)
			}
		}
	}

	// If there was an execution error, the parallel task should be considered failed
	if executionError != nil {
		return nil, executionError
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	mainTaskResponse := a.convertToMainTaskResponse(ctx, result)
	return mainTaskResponse, nil
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
	// Extract the MainTaskResponse from the response result
	var mainTaskResponse *task.MainTaskResponse
	if result.Response != nil {
		if mtr, ok := result.Response.(*task.MainTaskResponse); ok {
			mainTaskResponse = mtr
		}
	}
	// If no MainTaskResponse in Response field, create one from state
	if mainTaskResponse == nil {
		mainTaskResponse = &task.MainTaskResponse{
			State: result.State,
		}
		// Get parallel metadata from config store if available
		configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.cwd)
		if err != nil {
			// Log error but don't fail - metadata is optional for response
			logger.FromContext(ctx).Error("failed to create task config repository", "error", err)
		} else {
			metadata, err := configRepo.LoadParallelMetadata(ctx, result.State.TaskExecID)
			if err == nil && metadata != nil {
				if parallelMetadata, ok := metadata.(*task2core.ParallelTaskMetadata); ok {
					// Add parallel-specific metadata to output
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
