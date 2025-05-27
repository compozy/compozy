package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// -----------------------------------------------------------------------------
// GetExecution
// -----------------------------------------------------------------------------

type GetExecution struct {
	repo       task.Repository
	taskExecID core.ID
}

func NewGetExecution(repo task.Repository, taskExecID core.ID) *GetExecution {
	return &GetExecution{repo: repo, taskExecID: taskExecID}
}

func (uc *GetExecution) Execute(ctx context.Context) (*task.Execution, error) {
	return uc.repo.GetExecution(ctx, uc.taskExecID)
}

// -----------------------------------------------------------------------------
// ListAllExecutions
// -----------------------------------------------------------------------------

type ListAllExecutions struct {
	repo task.Repository
}

func NewListAllExecutions(repo task.Repository) *ListAllExecutions {
	return &ListAllExecutions{repo: repo}
}

func (uc *ListAllExecutions) Execute(ctx context.Context) ([]task.Execution, error) {
	return uc.repo.ListExecutions(ctx)
}

// -----------------------------------------------------------------------------
// ListExecutionsByTaskID
// -----------------------------------------------------------------------------

type ListExecutionsByTaskID struct {
	repo       task.Repository
	workflowID string
	taskID     string
}

func NewListExecutionsByTaskID(repo task.Repository, workflowID, taskID string) *ListExecutionsByTaskID {
	return &ListExecutionsByTaskID{repo: repo, workflowID: workflowID, taskID: taskID}
}

func (uc *ListExecutionsByTaskID) Execute(ctx context.Context) ([]task.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowAndTaskID(ctx, uc.workflowID, uc.taskID)
}
