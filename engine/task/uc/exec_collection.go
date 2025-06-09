package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// -----------------------------------------------------------------------------
// ExecuteCollection
// -----------------------------------------------------------------------------

type ExecuteCollectionInput struct {
	WorkflowState  *workflow.State  `json:"workflow_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type ExecuteCollectionOutput struct {
	Response     *task.Response `json:"response"`
	ItemCount    int            `json:"item_count"`
	SkippedCount int            `json:"skipped_count"`
}

type ExecuteCollection struct {
	createStateUC        *CreateState
	handleResponseUC     *HandleResponse
	contextBuilder       *normalizer.ContextBuilder
	collectionNormalizer *normalizer.CollectionNormalizer
	configBuilder        *normalizer.CollectionConfigBuilder
}

func NewExecuteCollection(
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
) *ExecuteCollection {
	return &ExecuteCollection{
		createStateUC:        NewCreateState(taskRepo),
		handleResponseUC:     NewHandleResponse(workflowRepo, taskRepo),
		contextBuilder:       normalizer.NewContextBuilder(),
		collectionNormalizer: normalizer.NewCollectionNormalizer(),
		configBuilder:        normalizer.NewCollectionConfigBuilder(),
	}
}

func (uc *ExecuteCollection) Execute(
	ctx context.Context,
	input *ExecuteCollectionInput,
) (*ExecuteCollectionOutput, error) {
	templateContext := uc.contextBuilder.BuildCollectionContext(input.WorkflowState, input.TaskConfig)
	filteredItems, skippedCount, err := uc.processCollectionItems(ctx, input.TaskConfig, templateContext)
	if err != nil {
		return nil, err
	}
	childConfigs, err := uc.configBuilder.CreateChildConfigs(input.TaskConfig, filteredItems, templateContext)
	if err != nil {
		return nil, err
	}
	response, err := uc.executeAsParallelTask(
		ctx,
		input.TaskConfig,
		childConfigs,
		input.WorkflowState,
		input.WorkflowConfig,
	)
	if err != nil {
		return nil, err
	}
	return &ExecuteCollectionOutput{
		Response:     response,
		ItemCount:    len(filteredItems),
		SkippedCount: skippedCount,
	}, nil
}

// processCollectionItems expands and filters collection items using the normalizer
func (uc *ExecuteCollection) processCollectionItems(
	ctx context.Context,
	taskConfig *task.Config,
	templateContext map[string]any,
) ([]any, int, error) {
	items, err := uc.collectionNormalizer.ExpandCollectionItems(ctx, &taskConfig.CollectionConfig, templateContext)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to expand collection items: %w", err)
	}
	filteredItems, err := uc.collectionNormalizer.FilterCollectionItems(
		ctx,
		&taskConfig.CollectionConfig,
		items,
		templateContext,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to filter collection items: %w", err)
	}
	skippedCount := len(items) - len(filteredItems)
	return filteredItems, skippedCount, nil
}

// executeAsParallelTask executes collection tasks as a parallel task
func (uc *ExecuteCollection) executeAsParallelTask(
	ctx context.Context,
	taskConfig *task.Config,
	childConfigs []task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*task.Response, error) {
	parallelConfig := taskConfig.ParallelTask
	parallelConfig.Tasks = childConfigs
	if taskConfig.GetMode() == task.CollectionModeSequential {
		parallelConfig.MaxWorkers = 1
	}
	if taskConfig.Batch > 0 && taskConfig.GetMode() == task.CollectionModeSequential {
		parallelConfig.MaxWorkers = taskConfig.Batch
	}
	collectionTaskConfig := *taskConfig
	collectionTaskConfig.Type = task.TaskTypeParallel
	collectionTaskConfig.ParallelTask = parallelConfig
	state, err := uc.createStateUC.Execute(ctx, &CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     &collectionTaskConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create collection state: %w", err)
	}
	response, err := uc.handleResponseUC.Execute(ctx, &HandleResponseInput{
		TaskState:      state,
		WorkflowConfig: workflowConfig,
		TaskConfig:     &collectionTaskConfig,
		ExecutionError: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}
	return response, nil
}
