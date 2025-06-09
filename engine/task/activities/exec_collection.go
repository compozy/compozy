package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const ExecuteCollectionLabel = "ExecuteCollection"

type ExecuteCollectionInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteCollection struct {
	loadWorkflowUC      *uc.LoadWorkflow
	executeCollectionUC *uc.ExecuteCollection
}

// NewExecuteCollection creates a new ExecuteCollection activity
func NewExecuteCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *ExecuteCollection {
	return &ExecuteCollection{
		loadWorkflowUC:      uc.NewLoadWorkflow(workflows, workflowRepo),
		executeCollectionUC: uc.NewExecuteCollection(taskRepo, workflowRepo),
	}
}

type CollectionResponse struct {
	*task.Response     // Inherit all fields from task response
	ItemCount      int `json:"item_count"`    // Number of items processed
	SkippedCount   int `json:"skipped_count"` // Number of items filtered out
}

func (a *ExecuteCollection) Run(ctx context.Context, input *ExecuteCollectionInput) (*CollectionResponse, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	result, err := a.executeCollectionUC.Execute(ctx, &uc.ExecuteCollectionInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute collection: %w", err)
	}
	return &CollectionResponse{
		Response:     result.Response,
		ItemCount:    result.ItemCount,
		SkippedCount: result.SkippedCount,
	}, nil
}
