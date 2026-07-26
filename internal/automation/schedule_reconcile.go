package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Scheduler) reconcileSchedulerState(
	ctx context.Context,
	job Job,
	plan schedulePlan,
) (SchedulerState, error) {
	now := s.now()
	defaultPolicy, err := s.defaultCatchUpPolicy(ctx, job)
	if err != nil {
		return SchedulerState{}, err
	}
	state := initialSchedulerState(job, plan, now, defaultPolicy)
	if s.store == nil {
		return schedulerStateWithoutStore(state, plan.register, now), nil
	}

	existing, loadErr := s.store.GetSchedulerState(ctx, job.ID)
	if loadErr != nil && !errors.Is(loadErr, ErrSchedulerStateNotFound) {
		return SchedulerState{}, fmt.Errorf(
			"automation: load scheduler state for job %q: %w",
			job.ID,
			loadErr,
		)
	}
	if loadErr == nil {
		state = mergeSchedulerState(existing, job, plan, now, defaultPolicy)
	}

	if !plan.register {
		state.NextRunAt = nil
		state.LastMisfireAt = timePointer(now)
		state.MisfireCount++
		state.UpdatedAt = now
		return s.store.SaveSchedulerState(ctx, state)
	}
	if state.NextRunAt == nil || state.NextRunAt.IsZero() {
		state.NextRunAt = timePointer(plan.nextRun)
		state.UpdatedAt = now
		return s.store.SaveSchedulerState(ctx, state)
	}
	if !state.NextRunAt.After(now) {
		return s.reconcileMissedSchedulerState(ctx, job, state, now)
	}
	return state, nil
}

func initialSchedulerState(
	job Job,
	plan schedulePlan,
	now time.Time,
	defaultPolicy SchedulerCatchUpPolicy,
) SchedulerState {
	return SchedulerState{
		JobID:               job.ID,
		NextRunAt:           timePointer(plan.nextRun),
		ScheduleHash:        scheduleHash(job.Schedule),
		CatchUpPolicy:       scheduleCatchUpPolicy(job.Schedule, defaultPolicy),
		MisfireGraceSeconds: scheduleMisfireGraceSeconds(job.Schedule),
		UpdatedAt:           now,
	}
}

func schedulerStateWithoutStore(state SchedulerState, register bool, now time.Time) SchedulerState {
	if register {
		return state
	}
	state.NextRunAt = nil
	state.LastMisfireAt = timePointer(now)
	state.MisfireCount = 1
	return state
}

func mergeSchedulerState(
	state SchedulerState,
	job Job,
	plan schedulePlan,
	now time.Time,
	defaultPolicy SchedulerCatchUpPolicy,
) SchedulerState {
	desiredScheduleHash := scheduleHash(job.Schedule)
	scheduleChanged := strings.TrimSpace(state.ScheduleHash) != desiredScheduleHash
	policyFallback := schedulerCatchUpPolicyOrDefault(state.CatchUpPolicy, defaultPolicy)
	if scheduleChanged {
		policyFallback = defaultPolicy
		state.MisfireGraceSeconds = scheduleMisfireGraceSeconds(job.Schedule)
	}
	state.CatchUpPolicy = scheduleCatchUpPolicy(job.Schedule, policyFallback)
	if job.Schedule.CatchUpPolicy != "" || job.Schedule.MisfireGraceSeconds > 0 {
		state.MisfireGraceSeconds = scheduleMisfireGraceSeconds(job.Schedule)
	}
	state.UpdatedAt = now
	if scheduleChanged {
		state.NextRunAt = timePointer(plan.nextRun)
		state.ScheduleHash = desiredScheduleHash
		state.ConsecutiveResumeFailures = 0
	}
	return state
}

func (s *Scheduler) reconcileMissedSchedulerState(
	ctx context.Context,
	job Job,
	state SchedulerState,
	now time.Time,
) (SchedulerState, error) {
	missedAt := *state.NextRunAt
	state, skipReason := reconcileMissedSchedulerCursor(job, state, now, s.location)
	if skipReason == "" {
		return s.store.SaveSchedulerState(ctx, state)
	}

	result, err := s.store.ClaimScheduledRun(persistenceContext(ctx), SchedulerClaim{
		JobID:                job.ID,
		RunID:                scheduledRunID(job.ID, missedAt),
		FireID:               scheduledFireID(job.ID, missedAt),
		ScheduledAt:          missedAt,
		NextRunAt:            cloneTimePointer(state.NextRunAt),
		ClaimedAt:            now,
		ScheduleHash:         state.ScheduleHash,
		CatchUpPolicy:        state.CatchUpPolicy,
		MisfireGraceSeconds:  state.MisfireGraceSeconds,
		Misfire:              true,
		SkipReason:           skipReason,
		NetworkParticipation: (DispatchRequest{Job: &job}).networkParticipation(),
	})
	if errors.Is(err, ErrScheduledFireAlreadyClaimed) {
		return s.store.GetSchedulerState(ctx, job.ID)
	}
	if err != nil {
		return SchedulerState{}, fmt.Errorf(
			"automation: record skipped fire for job %q: %w",
			job.ID,
			err,
		)
	}
	return result.State, nil
}
