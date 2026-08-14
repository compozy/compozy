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
	claimToken string,
	actor ActorContext,
) (Run, *Run, error) {
	if strings.TrimSpace(run.SessionID) == "" {
		if err := m.requireSessionExecutor("start run"); err != nil {
			return Run{}, nil, err
		}
	}

	lifecycleCtx := taskRunLifecycleContext(ctx)
	startingRun, startingTask, err := m.persistClaimedRunStarting(
		lifecycleCtx,
		run,
		claimToken,
		actor,
	)
	if err != nil {
		return Run{}, nil, err
	}
	if strings.TrimSpace(startingRun.SessionID) != "" {
		switch startingRun.ResolvedWorktreeModeValue().Normalize() {
		case WorktreeModePerRun:
			// A per-run checkout does not exist at claim time. Transfer the
			// claimed run to a dedicated session only after materialization.
		case WorktreeModeRef:
			bound, bindErr := m.attachAndPersistRunSession(
				lifecycleCtx,
				startingRun,
				startingRun.SessionID,
				claimToken,
				actor,
			)
			if bindErr == nil {
				return *bound, nil, nil
			}
			if !errors.Is(bindErr, ErrSessionAttachNotAllowed) {
				return Run{}, nil, bindErr
			}
		default:
			return startingRun, nil, nil
		}
	}
	return m.startAndBindClaimedRunSession(
		lifecycleCtx,
		taskRecord,
		startingTask,
		startingRun,
		claimToken,
		actor,
	)
}

func (m *Service) persistClaimedRunStarting(
	ctx context.Context,
	run Run,
	claimToken string,
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
			return store.TransitionRunStarting(
				ctx,
				NewRunStartingMutation(run, claimToken, commandAt),
			)
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
	claimToken string,
	actor ActorContext,
) (Run, *Run, error) {
	sessionRef, failedRun, err := m.startRunSession(
		ctx,
		taskRecord,
		startingTask,
		run,
		claimToken,
		actor,
	)
	if err != nil {
		return Run{}, failedRun, err
	}
	candidate := run
	candidate.SessionID = sessionRef.SessionID
	candidate.SetWorktreeID(sessionRef.WorktreeID)
	if err := m.preflightRunTransition(candidate, taskEventRunSessionBound, candidate.Status, actor); err != nil {
		stopErr := m.stopUnboundStartedTaskSession(ctx, run, sessionRef)
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
			return store.BindRunSession(
				ctx,
				NewRunSessionBindingMutation(
					run,
					claimToken,
					sessionRef.SessionID,
					sessionRef.WorktreeID,
					commandAt,
				),
			)
		},
		transitionRunPayload,
	)
	if err != nil {
		stopErr := m.stopUnboundStartedTaskSession(ctx, run, sessionRef)
		return Run{}, nil, errorsJoin(err, stopErr)
	}
	return settlement.mutation.Run, nil, nil
}

func (m *Service) startRunSession(
	ctx context.Context,
	taskRecord Task,
	startingTask Task,
	run Run,
	claimToken string,
	actor ActorContext,
) (*SessionRef, *Run, error) {
	profile, err := m.startTaskExecutionProfile(ctx, startingTask.ID)
	if err != nil {
		publicErr := redactTaskError(err)
		message := fmt.Sprintf("load execution profile: %v", publicErr)
		failedRun, failErr := m.failRunAfterSessionStartError(
			ctx,
			taskRecord,
			run,
			claimToken,
			actor,
			message,
		)
		if failErr != nil {
			return nil, nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return nil, failedRun, fmt.Errorf("task: load execution profile for run %q: %w", run.ID, publicErr)
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
		failedRun, failErr := m.failRunAfterSessionStartError(
			ctx,
			taskRecord,
			run,
			claimToken,
			actor,
			message,
		)
		if failErr != nil {
			return nil, nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return nil, failedRun, fmt.Errorf("task: start task session for run %q: %w", run.ID, publicErr)
	}
	if sessionRef == nil {
		failedRun, failErr := m.failRunAfterSessionStartError(
			ctx,
			taskRecord,
			run,
			claimToken,
			actor,
			"start task session: nil session reference",
		)
		if failErr != nil {
			return nil, nil, failErr
		}
		return nil, failedRun, fmt.Errorf(
			"%w: start_task_session returned nil session reference",
			ErrValidation,
		)
	}
	if err := sessionRef.Validate(); err != nil {
		publicErr := redactTaskError(err)
		message := fmt.Sprintf("start task session: %v", publicErr)
		failedRun, failErr := m.failRunAfterSessionStartError(
			ctx,
			taskRecord,
			run,
			claimToken,
			actor,
			message,
		)
		if failErr != nil {
			return nil, nil, errorsJoin(publicErr, redactTaskError(failErr))
		}
		return nil, failedRun, publicErr
	}
	return sessionRef, nil, nil
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

func (m *Service) stopUnboundStartedTaskSession(ctx context.Context, run Run, ref *SessionRef) error {
	if ref == nil {
		return nil
	}
	trimmedSessionID := strings.TrimSpace(ref.SessionID)
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
	var cleanupErr error
	if cleaner, ok := m.sessions.(UnboundTaskSessionCleaner); ok {
		cleanupErr = cleaner.CleanupUnboundTaskSession(ctx, run, *ref)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("task: clean up unbound session resources: %w", cleanupErr)
		}
	}
	return errorsJoin(requestErr, forceErr, cleanupErr)
}

func (m *Service) failRunAfterSessionStartError(
	ctx context.Context,
	taskRecord Task,
	run Run,
	claimToken string,
	actor ActorContext,
	message string,
) (*Run, error) {
	if strings.TrimSpace(run.ClaimTokenHash) != "" {
		return m.FailRunLease(ctx, LeaseFailure{
			RunID:      run.ID,
			ClaimToken: claimToken,
			Failure:    RunFailure{Error: message},
			Now:        m.now().UTC(),
		}, actor)
	}
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
