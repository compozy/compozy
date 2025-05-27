package uc

import (
	"context"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
)

// -----------------------------------------------------------------------------
// ListChildrenExecutions
// -----------------------------------------------------------------------------

type ListChildrenExecutions struct {
	repo       task.Repository
	taskExecID core.ID
}

func NewListChildrenExecutions(
	repo task.Repository,
	taskExecID core.ID,
) *ListChildrenExecutions {
	return &ListChildrenExecutions{
		repo:       repo,
		taskExecID: taskExecID,
	}
}

func (uc *ListChildrenExecutions) Execute(ctx context.Context) ([]core.Execution, error) {
	return uc.repo.ListChildrenExecutions(ctx, uc.taskExecID)
}

// -----------------------------------------------------------------------------
// ListChildrenExecutionsByID
// -----------------------------------------------------------------------------

type ListChildrenExecutionsByID struct {
	repo   task.Repository
	taskID string
}

func NewListChildrenExecutionsByID(
	repo task.Repository,
	taskID string,
) *ListChildrenExecutionsByID {
	return &ListChildrenExecutionsByID{
		repo:   repo,
		taskID: taskID,
	}
}

func (uc *ListChildrenExecutionsByID) Execute(ctx context.Context) ([]core.Execution, error) {
	return uc.repo.ListChildrenExecutionsByTaskID(ctx, uc.taskID)
}

// -----------------------------------------------------------------------------
// ListAgentExecutionsByExecID
// -----------------------------------------------------------------------------

type ListAgentExecutionsByExecID struct {
	repo       agent.Repository
	taskExecID core.ID
}

func NewListAgentExecutionsByExecID(repo agent.Repository, taskExecID core.ID) *ListAgentExecutionsByExecID {
	return &ListAgentExecutionsByExecID{
		repo:       repo,
		taskExecID: taskExecID,
	}
}

func (uc *ListAgentExecutionsByExecID) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutionsByTaskExecID(ctx, uc.taskExecID)
}

// -----------------------------------------------------------------------------
// ListAgentExecutionsByID
// -----------------------------------------------------------------------------

type ListAgentExecutionsByID struct {
	repo   agent.Repository
	taskID string
}

func NewListAgentExecutionsByID(repo agent.Repository, taskID string) *ListAgentExecutionsByID {
	return &ListAgentExecutionsByID{
		repo:   repo,
		taskID: taskID,
	}
}

func (uc *ListAgentExecutionsByID) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutionsByTaskID(ctx, uc.taskID)
}

// -----------------------------------------------------------------------------
// ListToolExecutionsByExecID
// -----------------------------------------------------------------------------

type ListToolExecutionsByExecID struct {
	repo       tool.Repository
	taskExecID core.ID
}

func NewListToolExecutionsByExecID(repo tool.Repository, taskExecID core.ID) *ListToolExecutionsByExecID {
	return &ListToolExecutionsByExecID{
		repo:       repo,
		taskExecID: taskExecID,
	}
}

func (uc *ListToolExecutionsByExecID) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutionsByTaskExecID(ctx, uc.taskExecID)
}

// -----------------------------------------------------------------------------
// ListToolExecutionsByID
// -----------------------------------------------------------------------------

type ListToolExecutionsByID struct {
	repo   tool.Repository
	taskID string
}

func NewListToolExecutionsByID(repo tool.Repository, taskID string) *ListToolExecutionsByID {
	return &ListToolExecutionsByID{
		repo:   repo,
		taskID: taskID,
	}
}

func (uc *ListToolExecutionsByID) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutionsByTaskID(ctx, uc.taskID)
}
