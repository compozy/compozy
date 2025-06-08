package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

const ExecuteCollectionItemLabel = "ExecuteCollectionItem"

type ExecuteCollectionItemInput struct {
	ParentTaskExecID core.ID      `json:"parent_task_exec_id"` // Parent collection task
	ItemIndex        int          `json:"item_index"`          // Index in collection
	Item             any          `json:"item"`                // The actual item to process
	TaskConfig       *task.Config `json:"task_config"`         // Template task config
}

type ExecuteCollectionItemResult struct {
	ItemIndex  int             `json:"item_index"`
	TaskExecID core.ID         `json:"task_exec_id"` // Child task execution ID
	Status     core.StatusType `json:"status"`
	Output     *core.Output    `json:"output,omitempty"`
	Error      *core.Error     `json:"error,omitempty"`
}

type ExecuteCollectionItem struct {
	loadWorkflowUC        *uc.LoadWorkflow
	createStateUC         *uc.CreateState
	executeTaskUC         *uc.ExecuteTask
	handleResponseUC      *uc.HandleResponse
	taskRepo              task.Repository
	taskTemplateEvaluator *uc.TaskTemplateEvaluator
}

// NewExecuteCollectionItem creates a new ExecuteCollectionItem activity
func NewExecuteCollectionItem(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *ExecuteCollectionItem {
	return &ExecuteCollectionItem{
		loadWorkflowUC:        uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:         uc.NewCreateState(taskRepo),
		executeTaskUC:         uc.NewExecuteTask(runtime),
		handleResponseUC:      uc.NewHandleResponse(workflowRepo, taskRepo),
		taskRepo:              taskRepo,
		taskTemplateEvaluator: uc.NewTaskTemplateEvaluator(),
	}
}

func (a *ExecuteCollectionItem) Run(
	ctx context.Context,
	input *ExecuteCollectionItemInput,
) (*ExecuteCollectionItemResult, error) {
	// Get the parent collection task state
	parentState, err := a.taskRepo.GetState(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent collection state: %w", err)
	}

	if !parentState.IsCollection() {
		return nil, fmt.Errorf("parent task is not a collection task")
	}

	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     parentState.WorkflowID,
		WorkflowExecID: parentState.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Create item-specific task config with injected variables
	itemTaskConfig, err := a.createItemTaskConfig(ctx, input, parentState, workflowState)
	if err != nil {
		return nil, fmt.Errorf("failed to create item task config: %w", err)
	}

	// NOTE: Skip normalization for collection item task configs
	// The task config has already been fully processed by the template evaluator
	// Additional normalization would re-process agent actions without collection item context

	// Create task state for this item
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create item task state: %w", err)
	}

	// Execute the item task
	output, err := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig: itemTaskConfig,
	})

	if err != nil {
		// Handle execution error
		taskState.UpdateStatus(core.StatusFailed)
		taskState.Error = core.NewError(err, "collection_item_execution_failed", nil)

		// Update task state
		if updateErr := a.taskRepo.UpsertState(ctx, taskState); updateErr != nil {
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
	if err := a.taskRepo.UpsertState(ctx, taskState); err != nil {
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

func (a *ExecuteCollectionItem) createItemTaskConfig(
	ctx context.Context,
	input *ExecuteCollectionItemInput,
	parentState *task.State,
	workflowState *workflow.State,
) (*task.Config, error) {
	collectionState := parentState.CollectionState

	// Load workflow config to get environment and other configuration
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     workflowState.WorkflowID,
		WorkflowExecID: workflowState.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow config: %w", err)
	}

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
	itemTaskConfig, err := a.taskTemplateEvaluator.EvaluateTaskTemplate(
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
