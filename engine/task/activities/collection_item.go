package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const ExecuteCollectionItemLabel = "ExecuteCollectionItem"

type ExecuteCollectionItemInput struct {
	ParentTaskExecID core.ID      `json:"parent_task_exec_id"` // Parent collection task
	ItemIndex        int          `json:"item_index"`          // Index in collection
	Item             any          `json:"item"`                // The collection item
	TaskConfig       *task.Config `json:"task_config"`         // Template task config
	WorkflowID       string       `json:"workflow_id"`
	WorkflowExecID   core.ID      `json:"workflow_exec_id"`
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
	executeUC        *uc.ExecuteTask
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
		executeUC:        uc.NewExecuteTask(runtime),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
		taskRepo:         taskRepo,
	}
}

func (a *ExecuteCollectionItem) Run(
	ctx context.Context,
	input *ExecuteCollectionItemInput,
) (*ExecuteCollectionItemResult, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Get parent collection task to access configuration
	parentState, err := a.taskRepo.GetState(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load parent collection state: %w", err)
	}

	if !parentState.IsCollection() || parentState.CollectionState == nil {
		return nil, fmt.Errorf("parent task is not a collection task")
	}

	// Create a copy of the task template for this item
	itemTaskConfig := &task.Config{}
	if err := itemTaskConfig.Merge(input.TaskConfig); err != nil {
		return nil, fmt.Errorf("failed to copy task config: %w", err)
	}

	// Set unique ID for this item task
	itemTaskConfig.ID = fmt.Sprintf("%s.item[%d]", parentState.TaskID, input.ItemIndex)

	// Create a custom workflow state that includes item and index variables
	// This is a workaround since the normalizer doesn't support custom evaluators yet
	extendedWorkflowState := &workflow.State{}
	*extendedWorkflowState = *workflowState // Copy the original state

	// Add item and index to the workflow state for template processing
	if extendedWorkflowState.Input == nil {
		extendedWorkflowState.Input = &core.Input{}
	}
	(*extendedWorkflowState.Input)[parentState.CollectionState.ItemVar] = input.Item
	(*extendedWorkflowState.Input)[parentState.CollectionState.IndexVar] = input.ItemIndex

	// Normalize task config with extended context
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  extendedWorkflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize item task config: %w", err)
	}

	// Create task state for this item
	itemState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create item task state: %w", err)
	}

	// Execute the item task
	output, execErr := a.executeUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig: itemTaskConfig,
	})

	// Handle execution result
	var taskError *core.Error
	status := core.StatusSuccess

	if execErr != nil {
		status = core.StatusFailed
		taskError = core.NewError(execErr, "item_execution_error", map[string]any{
			"item_index": input.ItemIndex,
			"item":       input.Item,
		})
		itemState.Error = taskError
	} else {
		itemState.Output = output
	}

	itemState.Status = status

	// Update the item state in database
	_, err = a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      itemState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     itemTaskConfig,
		ExecutionError: execErr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle item task response: %w", err)
	}

	return &ExecuteCollectionItemResult{
		ItemIndex:  input.ItemIndex,
		TaskExecID: itemState.TaskExecID,
		Status:     status,
		Output:     output,
		Error:      taskError,
	}, nil
}
