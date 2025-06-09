package activities

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
)

const UpdateParentStatusLabel = "UpdateParentStatus"

type UpdateParentStatusInput struct {
	ParentStateID core.ID               `json:"parent_state_id"`
	Strategy      task.ParallelStrategy `json:"strategy"`
}

type UpdateParentStatus struct {
	parentStatusUpdater *services.ParentStatusUpdater
}

// NewUpdateParentStatus creates a new UpdateParentStatus activity
func NewUpdateParentStatus(taskRepo task.Repository) *UpdateParentStatus {
	// Fail fast on invalid input – protects against silent no-op updates.
	if taskRepo == nil {
		panic("taskRepo must not be nil")
	}
	return &UpdateParentStatus{
		parentStatusUpdater: services.NewParentStatusUpdater(taskRepo),
	}
}

// defaultStrategy returns StrategyWaitAll if the provided strategy is empty,
// otherwise returns the provided strategy unchanged.
func defaultStrategy(s task.ParallelStrategy) task.ParallelStrategy {
	if s == "" {
		return task.StrategyWaitAll
	}
	return s
}

func (a *UpdateParentStatus) Run(ctx context.Context, input *UpdateParentStatusInput) (*task.State, error) {
	return a.parentStatusUpdater.UpdateParentStatus(ctx, &services.UpdateParentStatusInput{
		ParentStateID: input.ParentStateID,
		Strategy:      defaultStrategy(input.Strategy),
		Recursive:     false,
		ChildState:    nil,
	})
}
