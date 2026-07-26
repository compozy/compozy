package testutil

import (
	"context"

	core "github.com/compozy/agh/internal/api/core"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *StubTaskManager) GetTask(
	ctx context.Context,
	id string,
	actor taskpkg.ActorContext,
) (*taskpkg.View, error) {
	if s.GetTaskFn != nil {
		return s.GetTaskFn(ctx, id, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) InspectTask(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorContext,
) (*taskpkg.InspectView, error) {
	if s.InspectTaskFn != nil {
		return s.InspectTaskFn(ctx, taskID, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) InspectRun(
	ctx context.Context,
	runID string,
	actor taskpkg.ActorContext,
) (*taskpkg.InspectView, error) {
	if s.InspectRunFn != nil {
		return s.InspectRunFn(ctx, runID, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (s *StubTaskManager) ListTaskRuns(
	ctx context.Context,
	taskID string,
	query taskpkg.RunQuery,
	actor taskpkg.ActorContext,
) ([]taskpkg.Run, error) {
	if s.ListTaskRunsFn != nil {
		return s.ListTaskRunsFn(ctx, taskID, query, actor)
	}
	return nil, nil
}

func (s *StubTaskManager) Timeline(
	ctx context.Context,
	taskID string,
	query taskpkg.TimelineQuery,
	actor taskpkg.ActorContext,
) ([]taskpkg.TimelineItem, error) {
	if s.TimelineFn != nil {
		return s.TimelineFn(ctx, taskID, query, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) Stream(
	ctx context.Context,
	taskID string,
	query taskpkg.StreamQuery,
	actor taskpkg.ActorContext,
) (<-chan taskpkg.StreamEvent, error) {
	if s.StreamFn != nil {
		return s.StreamFn(ctx, taskID, query, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) Tree(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorContext,
) (*taskpkg.TreeView, error) {
	if s.TreeFn != nil {
		return s.TreeFn(ctx, taskID, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) RunDetail(
	ctx context.Context,
	runID string,
	actor taskpkg.ActorContext,
) (*taskpkg.RunDetailView, error) {
	if s.RunDetailFn != nil {
		return s.RunDetailFn(ctx, runID, actor)
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

var _ core.TaskService = (*StubTaskManager)(nil)
