package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

type ExecuteCollectionItemInput struct {
	ParentTaskExecID core.ID      `json:"parent_task_exec_id"`
	ItemIndex        int          `json:"item_index"`
	Item             any          `json:"item"`
	TaskConfig       *task.Config `json:"task_config"`
}

type ExecuteCollectionItemResult struct {
	ItemIndex  int             `json:"item_index"`
	TaskExecID core.ID         `json:"task_exec_id"`
	Status     core.StatusType `json:"status"`
	Output     *core.Output    `json:"output,omitempty"`
	Error      *core.Error     `json:"error,omitempty"`
}

type ExecuteCollectionItem struct {
	loadWorkflowUC        *LoadWorkflow
	createStateUC         *CreateState
	executeTaskUC         *ExecuteTask
	taskRepo              task.Repository
	taskTemplateEvaluator *TaskTemplateEvaluator
}

func NewExecuteCollectionItem(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtimeManager *runtime.Manager,
) *ExecuteCollectionItem {
	return &ExecuteCollectionItem{
		loadWorkflowUC:        NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:         NewCreateState(taskRepo),
		executeTaskUC:         NewExecuteTask(runtimeManager),
		taskRepo:              taskRepo,
		taskTemplateEvaluator: NewTaskTemplateEvaluator(),
	}
}

func (uc *ExecuteCollectionItem) Execute(ctx context.Context, input *ExecuteCollectionItemInput) (*ExecuteCollectionItemResult, error) {
	// Get the parent collection task state
	parentState, err := uc.taskRepo.GetState(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent collection state: %w", err)
	}

	if !parentState.IsCollection() {
		return nil, fmt.Errorf("parent task is not a collection task")
	}

	// Load workflow state and config
	workflowState, workflowConfig, err := uc.loadWorkflowUC.Execute(ctx, &LoadWorkflowInput{
		WorkflowID:     parentState.WorkflowID,
		WorkflowExecID: parentState.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Create item-specific task config with injected variables
	itemTaskConfig, err := uc.createItemTaskConfig(ctx, input, parentState, workflowState, workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create item task config: %w", err)
	}

	// Create task state for this item
	taskState, err := uc.createStateUC.Execute(ctx, &CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create item task state: %w", err)
	}

	// Execute the item task
	output, err := uc.executeTaskUC.Execute(ctx, &ExecuteTaskInput{
		TaskConfig: itemTaskConfig,
	})

	if err != nil {
		// Handle execution error
		taskState.UpdateStatus(core.StatusFailed)
		taskState.Error = core.NewError(err, "collection_item_execution_failed", nil)

		// Update task state
		if updateErr := uc.taskRepo.UpsertState(ctx, taskState); updateErr != nil {
			return nil, fmt.Errorf("failed to update failed task state: %w", updateErr)
		}

		return &ExecuteCollectionItemResult{
			ItemIndex:  input.ItemIndex,
			TaskExecID: taskState.TaskExecID,
			Status:     core.StatusFailed,
			Output:     nil,
			Error:      core.NewError(err, "collection_item_execution_failed", nil),
		}, nil // Don't return the error - we want to handle it gracefully
	}

	// Update state with successful result
	taskState.UpdateStatus(core.StatusSuccess)
	taskState.Output = output

	// Update task state
	if err := uc.taskRepo.UpsertState(ctx, taskState); err != nil {
		return nil, fmt.Errorf("failed to update successful task state: %w", err)
	}

	return &ExecuteCollectionItemResult{
		ItemIndex:  input.ItemIndex,
		TaskExecID: taskState.TaskExecID,
		Status:     core.StatusSuccess,
		Output:     output,
		Error:      nil,
	}, nil
}

func (uc *ExecuteCollectionItem) createItemTaskConfig(
	ctx context.Context,
	input *ExecuteCollectionItemInput,
	parentState *task.State,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*task.Config, error) {
	collectionState := parentState.CollectionState

	// Build evaluation context using normalizer with proper environment merging
	contextBuilder := normalizer.NewContextBuilder()
	taskConfigs := normalizer.BuildTaskConfigsMap(workflowConfig.Tasks)

	// Merge environments: workflow -> task -> collection item
	envMerger := &core.EnvMerger{}
	mergedEnv, err := envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		input.TaskConfig.GetEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to merge environments: %w", err)
	}

	normCtx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   input.TaskConfig.With,
		MergedEnv:      &mergedEnv,
	}

	evaluationContext := contextBuilder.BuildContext(normCtx)

	// Use shared service to evaluate task template
	itemTaskConfig, err := uc.taskTemplateEvaluator.EvaluateTaskTemplate(
		input.TaskConfig,
		input.Item,
		input.ItemIndex,
		collectionState.GetItemVar(),
		collectionState.GetIndexVar(),
		evaluationContext,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate task template: %w", err)
	}

	// Generate unique ID for this item task
	itemTaskConfig.ID = fmt.Sprintf("%s.item[%d]", input.TaskConfig.ID, input.ItemIndex)

	// Ensure the collection item data is available in the task config's 'with' field
	// This is crucial for agent action normalization to work properly
	if itemTaskConfig.With == nil {
		itemTaskConfig.With = &core.Input{}
	}

	// Add the collection item variables to the task input
	// This ensures they're available during normalization phase
	(*itemTaskConfig.With)[collectionState.GetItemVar()] = input.Item
	(*itemTaskConfig.With)[collectionState.GetIndexVar()] = input.ItemIndex

	return itemTaskConfig, nil
}
