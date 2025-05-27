package uc

import (
	"context"
	"errors"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// -----------------------------------------------------------------------------
// ListExecutions
// -----------------------------------------------------------------------------

type ListExecutions struct {
	repo       task.Repository
	workflowID string
	taskID     string
}

func NewListExecutionsUC(repo task.Repository, workflowID, taskID string) *ListExecutions {
	return &ListExecutions{
		repo:       repo,
		workflowID: workflowID,
		taskID:     taskID,
	}
}

func (uc *ListExecutions) Execute(ctx context.Context) ([]*task.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowAndTask(ctx, uc.workflowID, uc.taskID)
}

// -----------------------------------------------------------------------------
// ListWorkflowExecutions
// -----------------------------------------------------------------------------

type ListWorkflowExecutions struct {
	repo           task.Repository
	workflowID     *string
	workflowExecID *core.ID
}

func NewListWorkflowExecutionsUC(
	repo task.Repository,
	workflowID *string,
	workflowExecID *core.ID,
) *ListWorkflowExecutions {
	return &ListWorkflowExecutions{
		repo:           repo,
		workflowID:     workflowID,
		workflowExecID: workflowExecID,
	}
}

func (uc *ListWorkflowExecutions) Execute(ctx context.Context) ([]*task.Execution, error) {
	if uc.workflowID != nil {
		return uc.repo.ListExecutionsByWorkflow(ctx, *uc.workflowID)
	}
	if uc.workflowExecID != nil {
		return uc.repo.ListExecutionsByWorkflowExecID(ctx, *uc.workflowExecID)
	}
	return nil, errors.New("workflowID or workflowExecID is required")
}
