package task

import (
	"context"

	"errors"
	"fmt"

	"strings"
)

func (m *Service) transitionClaimedRunToStarting(
	ctx context.Context,
	taskRecord Task,
	run Run,
	actor ActorContext,
) (Run, *Run, error) {
	if err := m.requireSessionExecutor("start run"); err != nil {
		return Run{}, nil, err
	}

	run.Status = TaskRunStatusStarting
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return Run{}, nil, err
	}

	lifecycleCtx := taskRunLifecycleContext(ctx)
	startingTask, err := m.reconcileTaskCascade(lifecycleCtx, run.TaskID, actor)
	if err != nil {
		return Run{}, nil, err
	}
	if err := m.recordTaskEvent(lifecycleCtx, run.TaskID, run.ID, taskEventRunStarting, actor, runTransitionPayload{
		Status:     run.Status,
		TaskStatus: startingTask.Status,
		SessionID:  run.SessionID,
	}); err != nil {
		return Run{}, nil, err
	}

	sessionID, failedRun, err := m.startRunSession(
		lifecycleCtx,
		taskRecord,
		startingTask,
		run,
		actor,
	)
	if err != nil {
		return Run{}, failedRun, err
	}
	run.SessionID = sessionID
	if err := m.store.UpdateTaskRun(lifecycleCtx, run); err != nil {
		stopErr := m.stopUnboundStartedTaskSession(lifecycleCtx, sessionID)
		return Run{}, nil, errorsJoin(err, stopErr)
	}

	boundTask, err := m.reconcileTaskCascade(lifecycleCtx, run.TaskID, actor)
	if err != nil {
		return Run{}, nil, err
	}
	if err := m.recordTaskEvent(lifecycleCtx, run.TaskID, run.ID, taskEventRunSessionBound, actor, runTransitionPayload{
		Status:     run.Status,
		TaskStatus: boundTask.Status,
		SessionID:  run.SessionID,
	}); err != nil {
		return Run{}, nil, err
	}
	return run, nil, nil
}

func (m *Service) startRunSession(
	ctx context.Context,
	taskRecord Task,
	startingTask Task,
	run Run,
	actor ActorContext,
) (string, *Run, error) {
	profile, err := m.startTaskExecutionProfile(ctx, startingTask.ID)
	if err != nil {
		message := fmt.Sprintf("load execution profile: %v", err)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(err, failErr)
		}
		return "", failedRun, fmt.Errorf("task: load execution profile for run %q: %w", run.ID, err)
	}
	sessionRef, err := m.sessions.StartTaskSession(ctx, &StartTaskSession{
		Task:             startingTask,
		Run:              run,
		ExecutionProfile: &profile,
		Actor:            actor,
	})
	if err != nil {
		message := fmt.Sprintf("start task session: %v", err)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(err, failErr)
		}
		return "", failedRun, fmt.Errorf("task: start task session for run %q: %w", run.ID, err)
	}
	if sessionRef == nil {
		failedRun, failErr := m.failRunAfterSessionStartError(
			ctx,
			taskRecord,
			run,
			actor,
			"start task session: nil session reference",
		)
		if failErr != nil {
			return "", nil, failErr
		}
		return "", failedRun, fmt.Errorf(
			"%w: start_task_session returned nil session reference",
			ErrValidation,
		)
	}
	if err := sessionRef.Validate(); err != nil {
		message := fmt.Sprintf("start task session: %v", err)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(err, failErr)
		}
		return "", failedRun, err
	}
	return strings.TrimSpace(sessionRef.SessionID), nil, nil
}

func (m *Service) startTaskExecutionProfile(
	ctx context.Context,
	taskID string,
) (ExecutionProfile, error) {
	profile, err := m.store.GetExecutionProfile(ctx, taskID)
	switch {
	case errors.Is(err, ErrExecutionProfileNotFound):
		return defaultExecutionProfile(taskID), nil
	case err != nil:
		return ExecutionProfile{}, fmt.Errorf(
			"task: load execution profile for session start: %w",
			err,
		)
	default:
		return profile, nil
	}
}

func (m *Service) stopUnboundStartedTaskSession(ctx context.Context, sessionID string) error {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" || m.sessions == nil {
		return nil
	}
	requestErr := m.sessions.RequestTaskStop(ctx, trimmedSessionID, StopReasonFailed)
	forceErr := m.sessions.ForceTaskStop(ctx, trimmedSessionID, StopReasonFailed)
	if requestErr != nil {
		requestErr = fmt.Errorf(
			"task: request stop for unbound session %q: %w",
			trimmedSessionID,
			requestErr,
		)
	}
	if forceErr != nil {
		forceErr = fmt.Errorf("task: force stop unbound session %q: %w", trimmedSessionID, forceErr)
	}
	return errorsJoin(requestErr, forceErr)
}

func (m *Service) failRunAfterSessionStartError(
	ctx context.Context,
	taskRecord Task,
	run Run,
	actor ActorContext,
	message string,
) (*Run, error) {
	return m.failRunRecord(ctx, taskRecord, run, RunFailure{Error: message}, actor)
}

func validateRunningSessionBinding(run Run) error {
	if strings.TrimSpace(run.SessionID) == "" {
		return fmt.Errorf(
			"%w: task run %q cannot transition from %q to %q without a session binding",
			ErrInvalidStatusTransition,
			run.ID,
			run.Status,
			TaskRunStatusRunning,
		)
	}
	return nil
}
