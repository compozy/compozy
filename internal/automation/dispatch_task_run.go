package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"time"

	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (d *Dispatcher) dispatchTaskBackedAttempt(
	ctx context.Context,
	req DispatchRequest,
	scheduledRun *Run,
	attempt int,
) (*Run, error) {
	if d.tasks == nil {
		return d.finishRun(
			ctx,
			scheduledRun,
			RunFailed,
			errors.New("automation: task-backed job requires task service"),
		)
	}

	preFirePrompt := strings.TrimSpace(req.Prompt)
	if preFirePrompt == "" && req.Job != nil {
		preFirePrompt = strings.TrimSpace(req.Job.Prompt)
	}
	preFirePrompt, canceled, hookErr := d.dispatchPreFireHook(ctx, req, preFirePrompt, attempt)
	if hookErr != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, hookErr)
	}
	if canceled {
		return d.finishRun(ctx, scheduledRun, RunCancelled, nil)
	}

	actor, err := directTaskActorContext(req.Job, scheduledRun.ID)
	if err != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, err)
	}

	taskRecord, err := d.tasks.CreateTask(ctx, directTaskSpec(req.Job, preFirePrompt), actor)
	if err != nil {
		return d.finishRun(ctx, scheduledRun, classifyDispatchError(err), err)
	}
	if taskRecord == nil || strings.TrimSpace(taskRecord.ID) == "" {
		return d.finishRun(ctx, scheduledRun, RunFailed, errors.New("automation: task service returned empty task"))
	}

	taskRun, err := d.tasks.EnqueueRun(ctx, taskpkg.EnqueueRun{
		TaskID:                     taskRecord.ID,
		IdempotencyKey:             automationTaskRunIdempotencyKey(scheduledRun.ID),
		NetworkParticipation:       cloneParticipationRequest(req.Job.Task.NetworkParticipation),
		NetworkParticipationSource: participation.SourceAutomationJob,
	}, actor)
	if err != nil {
		return d.finishRun(ctx, scheduledRun, classifyDispatchError(err), err)
	}
	if taskRun == nil || strings.TrimSpace(taskRun.ID) == "" {
		return d.finishRun(ctx, scheduledRun, RunFailed, errors.New("automation: task service returned empty task run"))
	}

	delegatedRun, err := d.delegateRun(ctx, scheduledRun, taskRecord.ID, taskRun.ID)
	if err != nil {
		return delegatedRun, err
	}
	d.dispatchPostFireHook(ctx, req, *delegatedRun)
	return delegatedRun, nil
}

func (d *Dispatcher) transitionRun(
	ctx context.Context,
	current *Run,
	mutate func(run *Run, now time.Time),
) (*Run, error) {
	if current == nil {
		return nil, errors.New("automation: run is required")
	}

	next := *current
	mutate(&next, d.now())

	updated, err := d.runs.UpdateRun(persistenceContext(ctx), next)
	if err != nil {
		return cloneRun(current), fmt.Errorf("automation: update run %q: %w", current.ID, err)
	}
	return &updated, nil
}

func (d *Dispatcher) delegateRun(ctx context.Context, current *Run, taskID string, taskRunID string) (*Run, error) {
	updatedRun, updateErr := d.transitionRun(ctx, current, func(run *Run, now time.Time) {
		run.TaskID = strings.TrimSpace(taskID)
		run.TaskRunID = strings.TrimSpace(taskRunID)
		run.Status = RunDelegated
		run.EndedAt = timePointer(now)
		run.Error = ""
	})
	if updateErr != nil {
		return updatedRun, updateErr
	}

	d.logger.Info(
		"automation.dispatch.delegated",
		"run_id", updatedRun.ID,
		"job_id", strings.TrimSpace(updatedRun.JobID),
		"trigger_id", strings.TrimSpace(updatedRun.TriggerID),
		"task_id", strings.TrimSpace(updatedRun.TaskID),
		"task_run_id", strings.TrimSpace(updatedRun.TaskRunID),
		"attempt", updatedRun.Attempt,
	)
	return updatedRun, nil
}

func (d *Dispatcher) finishRun(ctx context.Context, current *Run, status RunStatus, runErr error) (*Run, error) {
	updatedRun, updateErr := d.transitionRun(ctx, current, func(run *Run, now time.Time) {
		run.Status = status
		run.EndedAt = timePointer(now)
		if runErr != nil {
			run.Error = runErr.Error()
			return
		}
		run.Error = ""
	})
	if updateErr != nil {
		if runErr == nil {
			return updatedRun, updateErr
		}
		return updatedRun, errors.Join(runErr, updateErr)
	}

	if runErr == nil && status == RunCompleted {
		d.logger.Info(
			"automation.dispatch.completed",
			"run_id", updatedRun.ID,
			"job_id", strings.TrimSpace(updatedRun.JobID),
			"trigger_id", strings.TrimSpace(updatedRun.TriggerID),
			"session_id", strings.TrimSpace(updatedRun.SessionID),
			"attempt", updatedRun.Attempt,
		)
		return updatedRun, nil
	}
	if runErr == nil {
		d.logger.Info(
			"automation.dispatch.finished",
			"run_id", updatedRun.ID,
			"job_id", strings.TrimSpace(updatedRun.JobID),
			"trigger_id", strings.TrimSpace(updatedRun.TriggerID),
			"session_id", strings.TrimSpace(updatedRun.SessionID),
			"attempt", updatedRun.Attempt,
			"status", updatedRun.Status,
		)
		return updatedRun, nil
	}

	level := d.logger.Warn
	if status == RunCancelled {
		level = d.logger.Info
	}
	level(
		"automation.dispatch.failed",
		"run_id", updatedRun.ID,
		"job_id", strings.TrimSpace(updatedRun.JobID),
		"trigger_id", strings.TrimSpace(updatedRun.TriggerID),
		"session_id", strings.TrimSpace(updatedRun.SessionID),
		"attempt", updatedRun.Attempt,
		"status", updatedRun.Status,
		"error", runErr,
	)
	return updatedRun, runErr
}

func (d *Dispatcher) finishRunAfterSessionStop(
	ctx context.Context,
	current *Run,
	sessionID string,
	status RunStatus,
	runErr error,
) (*Run, error) {
	stopErr := d.stopAutomationSession(ctx, sessionID, status, runErr)
	if stopErr == nil {
		d.deleteAutomationSessionTaskActor(sessionID)
	}
	if stopErr != nil {
		wrappedStopErr := fmt.Errorf("automation: stop session %q: %w", strings.TrimSpace(sessionID), stopErr)
		if runErr == nil {
			status = RunFailed
			runErr = wrappedStopErr
		} else {
			runErr = errors.Join(runErr, wrappedStopErr)
		}
	}

	return d.finishRun(ctx, current, status, runErr)
}

func (d *Dispatcher) stopAutomationSession(
	ctx context.Context,
	sessionID string,
	status RunStatus,
	runErr error,
) error {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if d == nil || trimmedSessionID == "" {
		return nil
	}

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.sessionStopTimeout)
	defer cancel()

	cause, detail := dispatchStopCause(status, runErr)
	return d.sessions.StopWithCause(stopCtx, trimmedSessionID, cause, detail)
}
