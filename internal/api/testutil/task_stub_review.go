package testutil

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *StubTaskManager) GetExecutionProfile(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	if s.GetExecutionProfileFn != nil {
		return s.GetExecutionProfileFn(ctx, taskID, actor)
	}
	return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
}

func (s *StubTaskManager) SetExecutionProfile(
	ctx context.Context,
	taskID string,
	profile *taskpkg.ExecutionProfile,
	actor taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	if s.SetExecutionProfileFn != nil {
		return s.SetExecutionProfileFn(ctx, taskID, profile, actor)
	}
	return taskpkg.ExecutionProfile{}, taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) DeleteExecutionProfile(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorContext,
) error {
	if s.DeleteExecutionProfileFn != nil {
		return s.DeleteExecutionProfileFn(ctx, taskID, actor)
	}
	return taskpkg.ErrExecutionProfileNotFound
}

func (s *StubTaskManager) RequestRunReview(
	ctx context.Context,
	req taskpkg.RunReviewRequest,
	actor taskpkg.ActorContext,
) (taskpkg.RunReview, bool, error) {
	if s.RequestRunReviewFn != nil {
		return s.RequestRunReviewFn(ctx, req, actor)
	}
	return taskpkg.RunReview{}, false, taskpkg.ErrRunReviewNotFound
}

func (s *StubTaskManager) GetRunReview(
	ctx context.Context,
	reviewID string,
	actor taskpkg.ActorContext,
) (taskpkg.RunReview, error) {
	if s.GetRunReviewFn != nil {
		return s.GetRunReviewFn(ctx, reviewID, actor)
	}
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (s *StubTaskManager) RecordRunReview(
	ctx context.Context,
	req taskpkg.RecordRunReviewRequest,
	actor taskpkg.ActorContext,
) (taskpkg.RunReviewResult, error) {
	if s.RecordRunReviewFn != nil {
		return s.RecordRunReviewFn(ctx, req, actor)
	}
	return taskpkg.RunReviewResult{}, taskpkg.ErrRunReviewNotFound
}

func (s *StubTaskManager) BindRunReviewSession(
	ctx context.Context,
	req taskpkg.BindRunReviewSessionRequest,
	actor taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	if s.BindRunReviewSessionFn != nil {
		return s.BindRunReviewSessionFn(ctx, req, actor)
	}
	return taskpkg.RunReviewBinding{}, taskpkg.ErrRunReviewNotFound
}

func (s *StubTaskManager) LookupRunReviewForSession(
	_ context.Context,
	_ string,
	_ taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	return taskpkg.RunReviewBinding{}, taskpkg.ErrRunReviewNotFound
}

func (s *StubTaskManager) ListRunReviews(
	ctx context.Context,
	query taskpkg.RunReviewQuery,
	actor taskpkg.ActorContext,
) ([]taskpkg.RunReview, error) {
	if s.ListRunReviewsFn != nil {
		return s.ListRunReviewsFn(ctx, query, actor)
	}
	return nil, nil
}

func (s *StubTaskManager) AddDependency(
	ctx context.Context,
	spec taskpkg.AddDependency,
	actor taskpkg.ActorContext,
) error {
	if s.AddDependencyFn != nil {
		return s.AddDependencyFn(ctx, spec, actor)
	}
	return taskpkg.ErrTaskNotFound
}

func (s *StubTaskManager) RemoveDependency(
	ctx context.Context,
	taskID string,
	dependsOnID string,
	actor taskpkg.ActorContext,
) error {
	if s.RemoveDependencyFn != nil {
		return s.RemoveDependencyFn(ctx, taskID, dependsOnID, actor)
	}
	return taskpkg.ErrTaskNotFound
}
