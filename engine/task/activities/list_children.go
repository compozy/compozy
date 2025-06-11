package activities

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const ListChildStatesLabel = "ListChildStates"

type ListChildStatesInput struct {
	ParentTaskExecID core.ID `json:"parent_task_exec_id"`
}

type ListChildStates struct {
	taskRepo task.Repository
}

// NewListChildStates creates a new ListChildStates activity
func NewListChildStates(taskRepo task.Repository) *ListChildStates {
	return &ListChildStates{
		taskRepo: taskRepo,
	}
}

func (a *ListChildStates) Run(ctx context.Context, input *ListChildStatesInput) ([]*task.State, error) {
	return a.taskRepo.ListChildren(ctx, input.ParentTaskExecID)
}
