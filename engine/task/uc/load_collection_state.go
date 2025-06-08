package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

type LoadCollectionStateInput struct {
	TaskExecID core.ID
}

type LoadCollectionState struct {
	taskRepo task.Repository
}

func NewLoadCollectionState(taskRepo task.Repository) *LoadCollectionState {
	return &LoadCollectionState{
		taskRepo: taskRepo,
	}
}

func (uc *LoadCollectionState) Execute(ctx context.Context, input *LoadCollectionStateInput) (*task.State, error) {
	collectionState, err := uc.taskRepo.GetState(ctx, input.TaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection state: %w", err)
	}

	if !collectionState.IsCollection() {
		return nil, fmt.Errorf("task is not a collection task")
	}

	return collectionState, nil
}
