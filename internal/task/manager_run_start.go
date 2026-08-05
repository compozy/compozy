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
	if strings.TrimSpace(run.SessionID) == "" {
		if err := m.requireSessionExecutor("start run"); err != nil {
			return Run{}, nil, err
		}
	}

	lifecycleCtx := taskRunLifecycleContext(ctx)
	startingRun, startingTask, err := m.persistClaimedRunStarting(lifecycleCtx, run, actor)
	if err != nil {
		return Run{}, nil, err
	}
	if strings.TrimSpace(startingRun.SessionID) != "" {
		return startingRun, nil, nil
	}
	return m.startAndBindClaimedRunSession(lifecycleCtx, taskRecord, startingTask, startingRun, actor)
}

func (m *Service) persistClaimedRunStarting(
	ctx context.Context,
	run Run,
	actor ActorContext,
) (Run, Task, error) {
	if err := m.preflightRunTransition(run, taskEventRunStarting, TaskRunStatusStarting, actor); err != nil {
		return Run{}, Task{}, err
	}
	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"transition task run starting",
		taskEventRunStarting,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.TransitionRunStarting(ctx, NewRunStartingMutation(run))
		},
		transitionRunPayload,
	)
	if err != nil {
		return Run{}, Task{}, err
	}
	return settlement.mutation.Run, settlement.task, nil
}

func (m *Service) startAndBindClaimedRunSession(
	ctx context.Context,
	taskRecord Task,
	startingTask Task,
	run Run,
	actor ActorContext,
) (Run, *Run, error) {
	sessionID, failedRun, err := m.startRunSession(ctx, taskRecord, startingTask, run, actor)
	if err != nil {
		return Run{}, failedRun, err
	}
	candidate := run
	candidate.SessionID = sessionID
	if err := m.preflightRunTransition(candidate, taskEventRunSessionBound, candidate.Status, actor); err != nil {
		stopErr := m.stopUnboundStartedTaskSession(ctx, sessionID)
		return Run{}, nil, errorsJoin(err, stopErr)
	}
	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"bind task run session",
		taskEventRunSessionBound,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.BindRunSession(ctx, NewRunSessionBindingMutation(run, sessionID))
		},
		transitionRunPayload,
	)
	if err != nil {
		stopErr := m.stopUnboundStartedTaskSession(ctx, sessionID)
		return Run{}, nil, errorsJoin(err, stopErr)
	}
	return settlement.mutation.Run, nil, nil
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
		publicErr := redactTaskError(err)
		message := fmt.Sprintf("load execution profile: %v", publicErr)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return "", failedRun, fmt.Errorf("task: load execution profile for run %q: %w", run.ID, publicErr)
	}
	sessionRef, err := m.sessions.StartTaskSession(ctx, &StartTaskSession{
		Task:             startingTask,
		Run:              run,
		ExecutionProfile: &profile,
		Actor:            actor,
	})
	if err != nil {
		publicErr := redactTaskError(err)
		message := fmt.Sprintf("start task session: %v", publicErr)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return "", failedRun, fmt.Errorf("task: start task session for run %q: %w", run.ID, publicErr)
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
		publicErr := redactTaskError(err)
		message := fmt.Sprintf("start task session: %v", publicErr)
		failedRun, failErr := m.failRunAfterSessionStartError(ctx, taskRecord, run, actor, message)
		if failErr != nil {
			return "", nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return "", failedRun, publicErr
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
