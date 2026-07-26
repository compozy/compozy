package situation

import (
	"context"

	"errors"
	"fmt"

	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Service) BundleForActiveLease(
	ctx context.Context,
	req taskpkg.ContextRequest,
) (taskpkg.ContextBundle, error) {
	if err := checkContext(ctx); err != nil {
		return taskpkg.ContextBundle{}, err
	}
	store := s.taskStoreValue()
	if store == nil {
		return taskpkg.ContextBundle{}, nil
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return taskpkg.ContextBundle{}, fmt.Errorf("%w: session_id is required", taskpkg.ErrPermissionDenied)
	}
	run, ok, err := s.activeRunForSession(ctx, sessionID)
	if err != nil || !ok {
		return taskpkg.ContextBundle{}, err
	}
	if runID := strings.TrimSpace(req.RunID); runID != "" && runID != strings.TrimSpace(run.ID) {
		return taskpkg.ContextBundle{}, fmt.Errorf(
			"%w: run %q is not owned by session %q",
			taskpkg.ErrPermissionDenied,
			runID,
			sessionID,
		)
	}
	taskRecord, err := store.GetTask(ctx, run.TaskID)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	workspaceSnapshot, err := s.resolveWorkspace(ctx, taskRecord.WorkspaceID, "", nil)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	return s.bundleForRun(ctx, taskRecord, run, workspaceSnapshot, nil)
}

func (s *Service) BundleForOperatorTask(
	ctx context.Context,
	req taskpkg.OperatorTaskContextRequest,
) (taskpkg.ContextBundle, error) {
	if err := checkContext(ctx); err != nil {
		return taskpkg.ContextBundle{}, err
	}
	store := s.taskStoreValue()
	if store == nil {
		return taskpkg.ContextBundle{}, nil
	}
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return taskpkg.ContextBundle{}, fmt.Errorf("%w: task_id is required", taskpkg.ErrValidation)
	}
	taskRecord, err := store.GetTask(ctx, taskID)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	workspaceSnapshot, err := s.resolveWorkspace(ctx, taskRecord.WorkspaceID, "", nil)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	runs, err := store.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskID})
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	run, ok := selectTaskContextRun(taskRecord, runs)
	if !ok {
		return s.bundleForTaskOnly(ctx, taskRecord, workspaceSnapshot)
	}
	return s.bundleForRun(ctx, taskRecord, run, workspaceSnapshot, nil)
}

func (s *Service) TaskRunPromptOverlay(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	profile *taskpkg.ExecutionProfile,
) (string, error) {
	if s == nil {
		return "", nil
	}
	workspaceSnapshot, err := s.resolveWorkspace(ctx, taskRecord.WorkspaceID, "", nil)
	if err != nil {
		return "", err
	}
	bundle, err := s.bundleForRun(ctx, taskRecord, run, workspaceSnapshot, profile)
	if err != nil {
		return "", err
	}
	return renderTaskBundlePrompt(bundle)
}

func (s *Service) TaskRunPromptOverlayByID(ctx context.Context, taskID string, runID string) (string, error) {
	if s == nil {
		return "", nil
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	store := s.taskStoreValue()
	if store == nil {
		return "", nil
	}
	taskRecord, err := store.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return "", err
	}
	run, err := store.GetTaskRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(run.TaskID) != strings.TrimSpace(taskRecord.ID) {
		return "", fmt.Errorf(
			"%w: run %q belongs to task %q, not task %q",
			taskpkg.ErrValidation,
			run.ID,
			run.TaskID,
			taskRecord.ID,
		)
	}
	return s.TaskRunPromptOverlay(ctx, taskRecord, run, nil)
}

func (s *Service) activeRunForSession(
	ctx context.Context,
	sessionID string,
) (taskpkg.Run, bool, error) {
	store := s.taskStoreValue()
	if store == nil {
		return taskpkg.Run{}, false, nil
	}
	runs, err := store.ListTaskRuns(ctx, taskpkg.RunQuery{SessionID: strings.TrimSpace(sessionID)})
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	run, ok := selectActiveRun(runs)
	return run, ok, nil
}

func (s *Service) bundleForTaskOnly(
	ctx context.Context,
	taskRecord taskpkg.Task,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (taskpkg.ContextBundle, error) {
	cfg := taskContextConfig(workspaceSnapshot)
	profile, err := s.loadTaskExecutionProfile(ctx, taskRecord.ID)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	bundle := taskpkg.ContextBundle{
		Task:             taskReference(taskRecord),
		LatestEventSeq:   taskRecord.LatestEventSeq,
		Limits:           taskContextRuntimeLimits(cfg),
		ExecutionProfile: &profile,
		PriorAttempts:    []taskpkg.RunSummary{},
		RecentEvents:     []taskpkg.TimelineItem{},
		ReviewHistory:    []taskpkg.RunReviewSummary{},
	}
	return s.enforceTaskContextBudget(taskpkg.NormalizeContextBundle(bundle), cfg.ContextBodyMaxBytes)
}

func (s *Service) bundleForRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
	profileOverride *taskpkg.ExecutionProfile,
) (taskpkg.ContextBundle, error) {
	cfg := taskContextConfig(workspaceSnapshot)
	profile, err := s.loadTaskExecutionProfile(ctx, taskRecord.ID)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	if profileOverride != nil {
		profile = *profileOverride
	}
	priorAttempts, err := s.priorRunSummaries(ctx, taskRecord, run, cfg.ContextPriorAttempts)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	recentEvents, err := s.recentTaskEvents(ctx, taskRecord, cfg.ContextRecentEvents)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	reviewHistory, err := s.reviewHistory(ctx, taskRecord, cfg)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}
	reviewContinuation, err := s.reviewContinuation(ctx, run, cfg)
	if err != nil {
		return taskpkg.ContextBundle{}, err
	}

	currentRun := runSummaryFromTaskRun(run, taskRecord.MaxAttempts)
	bundle := taskpkg.ContextBundle{
		Task:               taskReference(taskRecord),
		LatestEventSeq:     taskRecord.LatestEventSeq,
		CurrentRun:         &currentRun,
		PriorAttempts:      priorAttempts,
		RecentEvents:       recentEvents,
		HandoffSummary:     handoffSummary(run, reviewContinuation, cfg.ContextBodyMaxBytes),
		Limits:             taskContextRuntimeLimits(cfg),
		ExecutionProfile:   &profile,
		ReviewContinuation: reviewContinuation,
		ReviewHistory:      reviewHistory,
	}
	return s.enforceTaskContextBudget(taskpkg.NormalizeContextBundle(bundle), cfg.ContextBodyMaxBytes)
}

func (s *Service) loadTaskExecutionProfile(ctx context.Context, taskID string) (taskpkg.ExecutionProfile, error) {
	store := s.taskStoreValue()
	if store == nil {
		return taskpkg.DefaultExecutionProfile(taskID), nil
	}
	profile, err := store.GetExecutionProfile(ctx, taskID)
	switch {
	case errors.Is(err, taskpkg.ErrExecutionProfileNotFound):
		return taskpkg.DefaultExecutionProfile(taskID), nil
	case err != nil:
		return taskpkg.ExecutionProfile{}, err
	default:
		return profile, nil
	}
}
