package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
)

const GetCollectionResponseLabel = "GetCollectionResponse"

type GetCollectionResponseInput struct {
	ParentState    *task.State      `json:"parent_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
}

type GetCollectionResponse struct {
	taskRepo      task.Repository
	taskResponder *services.TaskResponder
	configStore   services.ConfigStore
}

// NewGetCollectionResponse creates a new GetCollectionResponse activity
func NewGetCollectionResponse(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
) *GetCollectionResponse {
	return &GetCollectionResponse{
		taskRepo:      taskRepo,
		taskResponder: services.NewTaskResponder(workflowRepo, taskRepo),
		configStore:   configStore,
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

	executionError := a.processCollectionTask(ctx, input, taskConfig)
	var itemCount, skippedCount int
	if input.ParentState.Output != nil {
		if metadata, exists := (*input.ParentState.Output)["collection_metadata"]; exists {
			if metadataMap, ok := metadata.(map[string]any); ok {
				itemCount = toInt(metadataMap["item_count"])
				skippedCount = toInt(metadataMap["skipped_count"])
			}
		}
	}

	// Use TaskResponder to handle the collection response
	response, err := a.taskResponder.HandleCollection(ctx, &services.CollectionResponseInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskState:      input.ParentState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
		ItemCount:      itemCount,
		SkippedCount:   skippedCount,
	})
	if err != nil {
		return nil, err
	}

	// If there was an execution error, the collection should be considered failed
	if executionError != nil {
		return nil, executionError
	}

	return response, nil
}

// processCollectionTask handles collection task processing logic and returns execution error if any
func (a *GetCollectionResponse) processCollectionTask(
	ctx context.Context,
	input *GetCollectionResponseInput,
	taskConfig *task.Config,
) error {
	return processParentTask(ctx, a.taskRepo, input.ParentState, taskConfig, task.TaskTypeCollection)
}

// toInt safely converts any value to int, handling nil and different numeric types
func toInt(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
