package uc

import (
	"context"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// ListChildrenExecutions
// -----------------------------------------------------------------------------

type ListChildrenExecutions struct {
	repo           workflow.Repository
	workflowExecID core.ID
}

func NewListChildrenExecutions(
	repo workflow.Repository,
	workflowExecID core.ID,
) *ListChildrenExecutions {
	return &ListChildrenExecutions{
		repo:           repo,
		workflowExecID: workflowExecID,
	}
}

func (uc *ListChildrenExecutions) Execute(ctx context.Context) ([]core.Execution, error) {
	return uc.repo.ListChildrenExecutions(ctx, uc.workflowExecID)
}

// -----------------------------------------------------------------------------
// ListChildrenExecutions
// -----------------------------------------------------------------------------

type ListChildrenExecutionsByID struct {
	repo       workflow.Repository
	workflowID string
}

func NewListChildrenExecutionsByID(
	repo workflow.Repository,
	workflowID string,
) *ListChildrenExecutionsByID {
	return &ListChildrenExecutionsByID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListChildrenExecutionsByID) Execute(ctx context.Context) ([]core.Execution, error) {
	return uc.repo.ListChildrenExecutionsByWorkflowID(ctx, uc.workflowID)
}

// -----------------------------------------------------------------------------
// ListTaskExecutionsByExecID
// -----------------------------------------------------------------------------

type ListTaskExecutionsByExecID struct {
	repo           task.Repository
	workflowExecID core.ID
}

func NewListTaskExecutionsByExecID(repo task.Repository, workflowExecID core.ID) *ListTaskExecutionsByExecID {
	return &ListTaskExecutionsByExecID{
		repo:           repo,
		workflowExecID: workflowExecID,
	}
}

func (uc *ListTaskExecutionsByExecID) Execute(ctx context.Context) ([]task.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowExecID(ctx, uc.workflowExecID)
}

// -----------------------------------------------------------------------------
// ListTaskExecutionsByID
// -----------------------------------------------------------------------------

type ListTaskExecutionsByID struct {
	repo       task.Repository
	workflowID string
}

func NewListTaskExecutionsByID(repo task.Repository, workflowID string) *ListTaskExecutionsByID {
	return &ListTaskExecutionsByID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListTaskExecutionsByID) Execute(ctx context.Context) ([]task.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowID(ctx, uc.workflowID)
}

// -----------------------------------------------------------------------------
// ListAgentExecutionsByExecID
// -----------------------------------------------------------------------------

type ListAgentExecutionsByExecID struct {
	repo           agent.Repository
	workflowExecID core.ID
}

func NewListAgentExecutionsByExecID(repo agent.Repository, workflowExecID core.ID) *ListAgentExecutionsByExecID {
	return &ListAgentExecutionsByExecID{
		repo:           repo,
		workflowExecID: workflowExecID,
	}
}

func (uc *ListAgentExecutionsByExecID) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowExecID(ctx, uc.workflowExecID)
}

// -----------------------------------------------------------------------------
// ListAgentExecutionsByID
// -----------------------------------------------------------------------------

type ListAgentExecutionsByID struct {
	repo       agent.Repository
	workflowID string
}

func NewListAgentExecutionsByID(repo agent.Repository, workflowID string) *ListAgentExecutionsByID {
	return &ListAgentExecutionsByID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListAgentExecutionsByID) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowID(ctx, uc.workflowID)
}

// -----------------------------------------------------------------------------
// ListToolExecutionsByExecID
// -----------------------------------------------------------------------------

type ListToolExecutionsByExecID struct {
	repo           tool.Repository
	workflowExecID core.ID
}

func NewListToolExecutionsByExecID(repo tool.Repository, workflowExecID core.ID) *ListToolExecutionsByExecID {
	return &ListToolExecutionsByExecID{
		repo:           repo,
		workflowExecID: workflowExecID,
	}
}

func (uc *ListToolExecutionsByExecID) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowExecID(ctx, uc.workflowExecID)
}

// -----------------------------------------------------------------------------
// ListToolExecutionsByID
// -----------------------------------------------------------------------------

type ListToolExecutionsByID struct {
	repo       tool.Repository
	workflowID string
}

func NewListToolExecutionsByID(repo tool.Repository, workflowID string) *ListToolExecutionsByID {
	return &ListToolExecutionsByID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListToolExecutionsByID) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutionsByWorkflowID(ctx, uc.workflowID)
}
