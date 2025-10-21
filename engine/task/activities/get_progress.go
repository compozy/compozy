package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const GetProgressLabel = "GetProgress"

type GetProgressInput struct {
	ParentStateID core.ID `json:"parent_state_id"`
}

type GetProgress struct {
	taskRepo task.Repository
}

// NewGetProgress creates a new GetProgress activity
func NewGetProgress(taskRepo task.Repository) *GetProgress {
	return &GetProgress{
		taskRepo: taskRepo,
	}
}

func (a *GetProgress) Run(ctx context.Context, input *GetProgressInput) (*task.ProgressInfo, error) {
	if input == nil || input.ParentStateID == "" {
		return nil, fmt.Errorf("GetProgress: missing parent_state_id")
	}
	progressInfo, err := a.taskRepo.GetProgressInfo(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress info for parent %s: %w", input.ParentStateID, err)
	}
	return progressInfo, nil
}
