package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
)

const GetCollectionResponseLabel = "GetCollectionResponse"

type GetCollectionResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

// GetCollectionResponse handles collection response using tasks integration
type GetCollectionResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	tasksFactory tasks.Factory
	configStore  services.ConfigStore
	cwd          *core.PathCWD
}

// NewGetCollectionResponse creates a new GetCollectionResponse activity with tasks integration
func NewGetCollectionResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	tasksFactory tasks.Factory,
	cwd *core.PathCWD,
) *GetCollectionResponse {
	return &GetCollectionResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		tasksFactory: tasksFactory,
		configStore:  configStore,
		cwd:          cwd,
	}
}

func (a *GetCollectionResponse) Run(
	ctx context.Context,
	input *GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
	taskConfig, err := a.loadCollectionTaskConfig(ctx, input.ParentState.TaskExecID)
	if err != nil {
		return nil, err
	}
	workflowState, err := a.loadCollectionWorkflowState(ctx, input.ParentState.WorkflowExecID)
	if err != nil {
		return nil, err
	}
	executionError := a.processCollectionTask(ctx, input, taskConfig)
	handler, err := a.tasksFactory.CreateResponseHandler(ctx, task.TaskTypeCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection response handler: %w", err)
	}
	responseInput := a.buildCollectionResponseInput(input, workflowState, taskConfig, executionError)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}
	if err := a.applyDeferredCollectionTransformation(
		ctx,
		handler,
		responseInput,
		executionError,
		input.ParentState.Status,
	); err != nil {
		return nil, err
	}
	if executionError != nil {
		return nil, executionError
	}
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
	return converter.ConvertToCollectionResponse(ctx, result, a.configStore, a.tasksFactory, a.cwd)
}

// loadCollectionTaskConfig loads the stored task configuration for the parent state.
func (a *GetCollectionResponse) loadCollectionTaskConfig(
	ctx context.Context,
	taskExecID core.ID,
) (*task.Config, error) {
	taskConfig, err := a.configStore.Get(ctx, taskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task config: %w", err)
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config not found for task execution ID: %s", taskExecID.String())
	}
	return taskConfig, nil
}

// loadCollectionWorkflowState retrieves the workflow state for the parent execution.
func (a *GetCollectionResponse) loadCollectionWorkflowState(
	ctx context.Context,
	workflowExecID core.ID,
) (*workflow.State, error) {
	workflowState, err := a.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	return workflowState, nil
}

// buildCollectionResponseInput prepares the shared response input for the tasks handler.
func (a *GetCollectionResponse) buildCollectionResponseInput(
	input *GetCollectionResponseInput,
	workflowState *workflow.State,
	taskConfig *task.Config,
	executionError error,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      input.ParentState,
		WorkflowConfig: input.WorkflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
}

// applyDeferredCollectionTransformation executes deferred output transformations when necessary.
func (a *GetCollectionResponse) applyDeferredCollectionTransformation(
	ctx context.Context,
	handler shared.TaskResponseHandler,
	responseInput *shared.ResponseInput,
	executionError error,
	stateStatus core.StatusType,
) error {
	type deferredTransformer interface {
		ApplyDeferredOutputTransformation(context.Context, *shared.ResponseInput) error
	}
	if executionError != nil || stateStatus == core.StatusFailed {
		return nil
	}
	collectionHandler, ok := handler.(deferredTransformer)
	if !ok {
		return nil
	}
	if err := collectionHandler.ApplyDeferredOutputTransformation(ctx, responseInput); err != nil {
		return fmt.Errorf("failed to apply deferred output transformation: %w", err)
	}
	return nil
}
