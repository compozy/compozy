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
	state := s.initialSchedulerState(job, plan, now, defaultPolicy)
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
		state = s.mergeSchedulerState(existing, job, plan, now, defaultPolicy)
		if state.ScheduleHash != existing.ScheduleHash {
			state, err = s.store.SaveSchedulerState(ctx, state)
			if err != nil {
				return SchedulerState{}, err
			}
		}
	}

	if state.DeferredUntil != nil {
		return state, nil
	}
	if !plan.register {
		state.NextRunAt = nil
		state.LastMisfireAt = new(now)
		state.MisfireCount++
		state.UpdatedAt = now
		return s.store.SaveSchedulerState(ctx, state)
	}
	if state.NextRunAt == nil || state.NextRunAt.IsZero() {
		state.NextRunAt = new(plan.nextRun)
		state.UpdatedAt = now
		return s.store.SaveSchedulerState(ctx, state)
	}
	if !state.NextRunAt.After(now) {
		return s.reconcileMissedSchedulerState(ctx, job, state, now)
	}
	return s.store.SaveSchedulerState(ctx, state)
}

func (s *Scheduler) initialSchedulerState(
	job Job,
	plan schedulePlan,
	now time.Time,
	defaultPolicy SchedulerCatchUpPolicy,
) SchedulerState {
	return SchedulerState{
		JobID:               job.ID,
		NextRunAt:           new(plan.nextRun),
		ScheduleHash:        s.scheduleHash(job.Schedule),
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
	state.LastMisfireAt = new(now)
	state.MisfireCount = 1
	return state
}

func (s *Scheduler) mergeSchedulerState(
	state SchedulerState,
	job Job,
	plan schedulePlan,
	now time.Time,
	defaultPolicy SchedulerCatchUpPolicy,
) SchedulerState {
	desiredScheduleHash := s.scheduleHash(job.Schedule)
	currentHash := strings.TrimSpace(state.ScheduleHash)
	definitionHash := scheduleHash(job.Schedule)
	// Legacy hashes lack a zone. Adopt the current zone without losing a queued fire or catch-up.
	legacy := currentHash == definitionHash
	scheduleChanged := !legacy && currentHash != desiredScheduleHash
	zoneChanged := scheduleChanged && strings.HasPrefix(currentHash, definitionHash+":")
	policyFallback := schedulerCatchUpPolicyOrDefault(state.CatchUpPolicy, defaultPolicy)
	if scheduleChanged && !zoneChanged {
		policyFallback = defaultPolicy
		state.MisfireGraceSeconds = scheduleMisfireGraceSeconds(job.Schedule)
	}
	state.CatchUpPolicy = scheduleCatchUpPolicy(job.Schedule, policyFallback)
	if job.Schedule.CatchUpPolicy != "" || job.Schedule.MisfireGraceSeconds > 0 {
		state.MisfireGraceSeconds = scheduleMisfireGraceSeconds(job.Schedule)
	}
	state.UpdatedAt = now
	if scheduleChanged {
		state.NextRunAt = new(plan.nextRun)
		if !zoneChanged {
			state.DeferredUntil = nil
			state.ConsecutiveResumeFailures = 0
		}
	}
	state.ScheduleHash = desiredScheduleHash
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

	persistCtx1, cancelPersist1 := persistenceContext(ctx)
	defer cancelPersist1()
	result, err := s.store.ClaimScheduledRun(persistCtx1, SchedulerClaim{
		ProfileID:           job.ProfileID,
		JobID:               job.ID,
		RunID:               scheduledRunID(job.ID, missedAt),
		FireID:              scheduledFireID(job.ID, missedAt),
		ScheduledAt:         missedAt,
		NextRunAt:           cloneTimePointer(state.NextRunAt),
		ClaimedAt:           now,
		ScheduleHash:        state.ScheduleHash,
		CatchUpPolicy:       state.CatchUpPolicy,
		MisfireGraceSeconds: state.MisfireGraceSeconds,
		Misfire:             true,
		SkipReason:          skipReason,
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
