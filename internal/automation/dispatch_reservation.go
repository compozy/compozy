package automation

import (
	"context"
	"errors"
	"fmt"

	"reflect"
	"strings"

	"time"
)

func (d *Dispatcher) dispatchAttempt(ctx context.Context, req DispatchRequest, attempt int) (*Run, error) {
	if !d.tryAcquire() {
		return nil, fmt.Errorf(
			"%w: active=%d limit=%d",
			ErrConcurrencyLimitReached,
			len(d.gate),
			cap(d.gate),
		)
	}
	defer d.release()

	scheduledRun, err := d.reserveRun(ctx, req, attempt)
	if err != nil {
		return scheduledRun, err
	}
	if req.Job != nil && req.Job.Task != nil {
		return d.dispatchTaskBackedAttempt(ctx, req, scheduledRun, attempt)
	}
	if req.loopTarget() != nil {
		return d.dispatchLoopBackedAttempt(ctx, req, scheduledRun, attempt)
	}

	prompt, promptErr := req.prompt()
	if promptErr != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, promptErr)
	}
	prompt, canceled, hookErr := d.dispatchPreFireHook(ctx, req, prompt, attempt)
	if hookErr != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, hookErr)
	}
	if canceled {
		return d.finishRun(ctx, scheduledRun, RunCancelled, nil)
	}

	createOpts := d.createOpts(req)
	createdSession, createErr := d.sessions.Create(ctx, createOpts)
	if createErr != nil {
		return d.finishRun(ctx, scheduledRun, classifyDispatchError(createErr), createErr)
	}
	if createdSession == nil || strings.TrimSpace(createdSession.ID) == "" {
		return d.finishRun(
			ctx,
			scheduledRun,
			RunFailed,
			errors.New("automation: session creator returned empty session"),
		)
	}

	runningRun, err := d.transitionRun(ctx, scheduledRun, func(run *Run, _ time.Time) {
		run.Status = RunRunning
		run.SessionID = strings.TrimSpace(createdSession.ID)
	})
	if err != nil {
		return cloneRun(scheduledRun), err
	}
	if err := d.recordAutomationSessionTaskActor(
		createdSession.ID,
		createdSession.WorkspaceID,
		runningRun,
	); err != nil {
		return d.finishRunAfterSessionStop(ctx, runningRun, createdSession.ID, RunFailed, err)
	}

	events, promptErr := d.sessions.Prompt(ctx, createdSession.ID, prompt)
	if promptErr != nil {
		return d.finishRunAfterSessionStop(
			ctx,
			runningRun,
			createdSession.ID,
			classifyDispatchError(promptErr),
			promptErr,
		)
	}
	d.dispatchPostFireHook(ctx, req, *runningRun)

	runErr := collectPromptError(ctx, events)
	if runErr != nil {
		return d.finishRunAfterSessionStop(ctx, runningRun, createdSession.ID, classifyDispatchError(runErr), runErr)
	}

	return d.finishRunAfterSessionStop(ctx, runningRun, createdSession.ID, RunCompleted, nil)
}

func (d *Dispatcher) reserveRun(ctx context.Context, req DispatchRequest, attempt int) (*Run, error) {
	if req.ReservedRun != nil {
		return d.reserveExistingRun(ctx, req, attempt)
	}

	now := d.now()
	if fireLimitErr, err := d.evaluateFireLimit(ctx, req, "", now); err != nil {
		return nil, err
	} else if fireLimitErr != nil {
		return nil, fireLimitErr
	}

	run := Run{
		Status:               RunScheduled,
		Attempt:              attempt,
		StartedAt:            timePointer(now),
		NetworkParticipation: req.networkParticipation(),
	}
	if req.Job != nil {
		run.JobID = req.Job.ID
	} else {
		run.TriggerID = req.Trigger.ID
	}

	created, err := d.runs.CreateRun(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("automation: create scheduled run: %w", err)
	}
	return &created, nil
}

func (d *Dispatcher) reserveExistingRun(ctx context.Context, req DispatchRequest, attempt int) (*Run, error) {
	reserved := cloneRun(req.ReservedRun)
	if reserved == nil {
		return nil, errors.New("automation: reserved run is required")
	}
	if strings.TrimSpace(reserved.ID) == "" {
		return nil, errors.New("automation: reserved run id is required")
	}
	if reserved.Attempt < attempt {
		reserved.Attempt = attempt
	}
	if req.Job != nil && strings.TrimSpace(reserved.JobID) != strings.TrimSpace(req.Job.ID) {
		return nil, errors.New("automation: reserved run job_id does not match dispatch job")
	}
	if req.Trigger != nil && strings.TrimSpace(reserved.TriggerID) != strings.TrimSpace(req.Trigger.ID) {
		return nil, errors.New("automation: reserved run trigger_id does not match dispatch trigger")
	}
	requestedParticipation := req.networkParticipation()
	if !reflect.DeepEqual(reserved.NetworkParticipation, requestedParticipation) {
		return reserved, errors.New("automation: reserved run network participation does not match definition")
	}

	now := d.now()
	fireLimitErr, err := d.evaluateFireLimit(ctx, req, reserved.ID, now)
	if err != nil {
		return reserved, err
	}
	if fireLimitErr != nil {
		return d.finishRun(ctx, reserved, fireLimitRunStatus(req.Kind), fireLimitErr)
	}
	return reserved, nil
}

func fireLimitRunStatus(kind DispatchKind) RunStatus {
	if kind == DispatchKindSchedule {
		return RunCancelled
	}
	return RunFailed
}

func (d *Dispatcher) evaluateFireLimit(
	ctx context.Context,
	req DispatchRequest,
	excludeID string,
	now time.Time,
) (*FireLimitError, error) {
	fireLimit := req.fireLimitConfig()
	window, err := time.ParseDuration(fireLimit.Window)
	if err != nil {
		return nil, fmt.Errorf("automation: parse fire-limit window: %w", err)
	}

	query := RunQuery{
		Since:     now.Add(-window),
		Until:     now,
		ExcludeID: strings.TrimSpace(excludeID),
	}
	if req.Job != nil {
		query.JobID = req.Job.ID
	} else {
		query.TriggerID = req.Trigger.ID
	}

	d.fireLimitMu.Lock()
	defer d.fireLimitMu.Unlock()

	runs, err := d.runs.ListRuns(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("automation: evaluate fire limit: %w", err)
	}

	var (
		count   int64
		retryAt time.Time
	)
	for _, run := range runs {
		if !countsTowardFireLimit(run) {
			continue
		}
		count++
		if run.StartedAt == nil || run.StartedAt.IsZero() {
			continue
		}
		candidate := run.StartedAt.Add(window)
		if retryAt.IsZero() || candidate.Before(retryAt) {
			retryAt = candidate
		}
	}
	if count < int64(fireLimit.Max) {
		return nil, nil
	}
	if retryAt.IsZero() {
		retryAt = now
	}
	if retryAt.Before(now) {
		retryAt = now
	}

	return &FireLimitError{
		Count:   count,
		Limit:   fireLimit.Max,
		Window:  window,
		RetryAt: retryAt,
	}, nil
}

func countsTowardFireLimit(run Run) bool {
	return run.Status != RunCancelled
}
