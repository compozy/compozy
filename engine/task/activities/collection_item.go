package activities

import (
	"context"

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
	executeCollectionItemUC *uc.ExecuteCollectionItem
}

// NewExecuteCollectionItem creates a new ExecuteCollectionItem activity
func NewExecuteCollectionItem(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *ExecuteCollectionItem {
	return &ExecuteCollectionItem{
		executeCollectionItemUC: uc.NewExecuteCollectionItem(workflows, workflowRepo, taskRepo, runtime),
	}
}

func (a *ExecuteCollectionItem) Run(
	ctx context.Context,
	input *ExecuteCollectionItemInput,
) (*ExecuteCollectionItemResult, error) {
	// Convert to use case input and execute
	result, err := a.executeCollectionItemUC.Execute(ctx, &uc.ExecuteCollectionItemInput{
		ParentTaskExecID: input.ParentTaskExecID,
		ItemIndex:        input.ItemIndex,
		Item:             input.Item,
		TaskConfig:       input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}

	// Convert back to activity result
	return &ExecuteCollectionItemResult{
		ItemIndex:  result.ItemIndex,
		TaskExecID: result.TaskExecID,
		Status:     result.Status,
		Output:     result.Output,
		Error:      result.Error,
	}, nil
}


