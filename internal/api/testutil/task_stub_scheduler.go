package testutil

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *StubTaskManager) PauseTask(
	ctx context.Context,
	taskID string,
	req taskpkg.PauseTaskRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.PauseTaskFn != nil {
		return s.PauseTaskFn(ctx, taskID, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ResumeTask(
	ctx context.Context,
	taskID string,
	req taskpkg.ResumeTaskRequest,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	if s.ResumeTaskFn != nil {
		return s.ResumeTaskFn(ctx, taskID, req, actor)
	}
	return nil, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) SchedulerStatus(
	ctx context.Context,
	actor taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	if s.SchedulerStatusFn != nil {
		return s.SchedulerStatusFn(ctx, actor)
	}
	return taskpkg.SchedulerStatus{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) PauseScheduler(
	ctx context.Context,
	req taskpkg.SchedulerPauseRequest,
	actor taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	if s.PauseSchedulerFn != nil {
		return s.PauseSchedulerFn(ctx, req, actor)
	}
	return taskpkg.SchedulerStatus{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) ResumeScheduler(
	ctx context.Context,
	req taskpkg.SchedulerResumeRequest,
	actor taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	if s.ResumeSchedulerFn != nil {
		return s.ResumeSchedulerFn(ctx, req, actor)
	}
	return taskpkg.SchedulerStatus{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) DrainScheduler(
	ctx context.Context,
	req taskpkg.SchedulerDrainRequest,
	actor taskpkg.ActorContext,
) (taskpkg.SchedulerDrainResult, error) {
	if s.DrainSchedulerFn != nil {
		return s.DrainSchedulerFn(ctx, req, actor)
	}
	return taskpkg.SchedulerDrainResult{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) SchedulerBacklog(
	ctx context.Context,
	query taskpkg.SchedulerBacklogQuery,
	actor taskpkg.ActorContext,
) (taskpkg.SchedulerBacklog, error) {
	if s.SchedulerBacklogFn != nil {
		return s.SchedulerBacklogFn(ctx, query, actor)
	}
	return taskpkg.SchedulerBacklog{}, taskpkg.ErrTaskNotFound
}
