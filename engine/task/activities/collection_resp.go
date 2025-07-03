package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

const GetCollectionResponseLabel = "GetCollectionResponse"

type GetCollectionResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

// GetCollectionResponse handles collection response using task2 integration
type GetCollectionResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	task2Factory task2.Factory
	configStore  services.ConfigStore
}

// NewGetCollectionResponse creates a new GetCollectionResponse activity with task2 integration
func NewGetCollectionResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
) *GetCollectionResponse {
	return &GetCollectionResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		task2Factory: task2Factory,
		configStore:  configStore,
	}
}

func (a *GetCollectionResponse) Run(
	ctx context.Context,
	input *GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
	// Fetch task config from Redis using parent state's TaskExecID
	taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config not found for task execution ID: %s", input.ParentState.TaskExecID.String())
	}
	// Load workflow state
	workflowState, err := a.workflowRepo.GetState(ctx, input.ParentState.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	// Process the collection task
	executionError := a.processCollectionTask(ctx, input, taskConfig)
	// Use task2 ResponseHandler for collection type
	handler, err := a.task2Factory.CreateResponseHandler(task.TaskTypeCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}
	// Apply deferred output transformation for collection tasks after children are processed
	// This handles collection response aggregation with child outputs
	if collectionHandler, ok := handler.(interface {
		ApplyDeferredOutputTransformation(context.Context, *shared.ResponseInput) error
	}); ok {
		// CRITICAL: Only apply transformation if no execution error
		// This matches the condition in BaseResponseHandler.ApplyDeferredOutputTransformation
		if executionError == nil && input.ParentState.Status != core.StatusFailed {
			if err := collectionHandler.ApplyDeferredOutputTransformation(ctx, responseInput); err != nil {
				return nil, fmt.Errorf("failed to apply deferred output transformation: %w", err)
			}
		}
	}

	// If there was an execution error, the collection should be considered failed
	if executionError != nil {
		return nil, executionError
	}
	// Convert shared.ResponseOutput to task.CollectionResponse
	collectionResponse := a.convertToCollectionResponse(ctx, result)
	return collectionResponse, nil
}

// processCollectionTask handles collection task processing logic and returns execution error if any
func (a *GetCollectionResponse) processCollectionTask(
	ctx context.Context,
	input *GetCollectionResponseInput,
	taskConfig *task.Config,
) error {
	return processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeCollection)
}

// convertToCollectionResponse converts shared.ResponseOutput to task.CollectionResponse
func (a *GetCollectionResponse) convertToCollectionResponse(
	ctx context.Context,
	result *shared.ResponseOutput,
) *task.CollectionResponse {
	converter := NewResponseConverter()
	return converter.ConvertToCollectionResponse(ctx, result, a.configStore, a.task2Factory)
}
