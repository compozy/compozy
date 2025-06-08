package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	executeTaskUC    *uc.ExecuteTask
	handleResponseUC *uc.HandleResponse
	taskRepo         task.Repository
}

// NewExecuteCollectionItem creates a new ExecuteCollectionItem activity
func NewExecuteCollectionItem(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *ExecuteCollectionItem {
	return &ExecuteCollectionItem{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo),
		executeTaskUC:    uc.NewExecuteTask(runtime),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
		taskRepo:         taskRepo,
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
	itemTaskConfig := a.createItemTaskConfig(input, parentState)

	// Normalize the item task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize item task config: %w", err)
	}

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
	input *ExecuteCollectionItemInput,
	parentState *task.State,
) *task.Config {
	// Clone the task template
	taskTemplate := input.TaskConfig
	collectionState := parentState.CollectionState

	// Create a copy of the task config for this item
	itemConfig := &task.Config{}
	*itemConfig = *taskTemplate

	// Generate unique ID for this item task
	itemConfig.ID = fmt.Sprintf("%s.item[%d]", taskTemplate.ID, input.ItemIndex)

	// Pre-process the task template to resolve collection-specific variables
	itemVar := collectionState.GetItemVar()
	indexVar := collectionState.GetIndexVar()

	// Create collection context for template processing
	collectionContext := map[string]any{
		itemVar:  input.Item,
		indexVar: input.ItemIndex,
	}

	// Convert task config to map for template processing
	configMap, err := itemConfig.AsMap()
	if err != nil {
		// If conversion fails, fall back to original behavior
		return a.createItemTaskConfigFallback(input, parentState)
	}

	// Process templates in the config map with collection context
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processedConfigMap, err := engine.ParseMap(configMap, collectionContext)
	if err != nil {
		// If template processing fails, fall back to original behavior
		return a.createItemTaskConfigFallback(input, parentState)
	}

	// Update config from processed map
	if err := itemConfig.FromMap(processedConfigMap); err != nil {
		// If update fails, fall back to original behavior
		return a.createItemTaskConfigFallback(input, parentState)
	}

	// Now inject any remaining variables into the input (in case template used them)
	if itemConfig.With == nil {
		itemConfig.With = &core.Input{}
	}

	// Create a copy of the input
	itemInput := make(core.Input)
	if itemConfig.With != nil {
		for k, v := range *itemConfig.With {
			itemInput[k] = v
		}
	}

	// Inject collection variables for any templates that might still need them
	itemInput[itemVar] = input.Item
	itemInput[indexVar] = input.ItemIndex

	itemConfig.With = &itemInput

	return itemConfig
}

// createItemTaskConfigFallback provides the original behavior as fallback
func (a *ExecuteCollectionItem) createItemTaskConfigFallback(
	input *ExecuteCollectionItemInput,
	parentState *task.State,
) *task.Config {
	// Original implementation as fallback
	taskTemplate := input.TaskConfig
	collectionState := parentState.CollectionState

	itemConfig := &task.Config{}
	*itemConfig = *taskTemplate

	itemConfig.ID = fmt.Sprintf("%s.item[%d]", taskTemplate.ID, input.ItemIndex)

	if itemConfig.With == nil {
		itemConfig.With = &core.Input{}
	}

	itemInput := make(core.Input)
	if taskTemplate.With != nil {
		for k, v := range *taskTemplate.With {
			itemInput[k] = v
		}
	}

	itemVar := collectionState.GetItemVar()
	indexVar := collectionState.GetIndexVar()

	itemInput[itemVar] = input.Item
	itemInput[indexVar] = input.ItemIndex

	itemConfig.With = &itemInput

	return itemConfig
}
