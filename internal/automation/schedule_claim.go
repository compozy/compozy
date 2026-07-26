package automation

import (
	"context"

	"fmt"

	"time"
)

type scheduledJobClaimResult struct {
	claim       SchedulerClaim
	state       SchedulerState
	reservedRun *Run
	skipped     bool
	skipReason  SchedulerSkipReason
}

func (s *Scheduler) claimScheduledJob(
	ctx context.Context,
	registration scheduledRegistration,
) (scheduledJobClaimResult, error) {
	job := registration.definition
	scheduledAt := *registration.state.NextRunAt
	claimedAt := s.now()
	nextRun := nextRunAfter(job, scheduledAt, s.location)
	claim := SchedulerClaim{
		JobID:                job.ID,
		RunID:                scheduledRunID(job.ID, scheduledAt),
		FireID:               scheduledFireID(job.ID, scheduledAt),
		ScheduledAt:          scheduledAt,
		NextRunAt:            cloneTimePointer(nextRun),
		ClaimedAt:            claimedAt,
		ScheduleHash:         scheduleHash(job.Schedule),
		CatchUpPolicy:        registration.state.CatchUpPolicy,
		MisfireGraceSeconds:  registration.state.MisfireGraceSeconds,
		CatchUp:              scheduledFireIsCatchUp(scheduledAt, claimedAt, registration.state),
		NetworkParticipation: (DispatchRequest{Job: &job}).networkParticipation(),
	}
	if s.store != nil {
		result, err := s.store.ClaimScheduledRun(persistenceContext(ctx), claim)
		if err != nil {
			return scheduledJobClaimResult{}, fmt.Errorf("automation: claim scheduled job %q: %w", job.ID, err)
		}
		s.updateRegistrationState(job.ID, result.State)
		return scheduledJobClaimResult{
			claim:       claim,
			state:       result.State,
			reservedRun: &result.Run,
			skipped:     result.Skipped,
			skipReason:  result.SkipReason,
		}, nil
	}
	state := schedulerStateAfterInMemoryClaim(registration.state, claim, nextRun)
	s.updateRegistrationState(job.ID, state)
	return scheduledJobClaimResult{claim: claim, state: state}, nil
}

func schedulerStateAfterInMemoryClaim(
	current SchedulerState,
	claim SchedulerClaim,
	nextRun *time.Time,
) SchedulerState {
	state := current
	state.NextRunAt = cloneTimePointer(nextRun)
	state.LastRunAt = timePointer(claim.ClaimedAt)
	state.LastScheduledAt = timePointer(claim.ScheduledAt)
	state.LastFireID = claim.FireID
	state.ScheduleHash = claim.ScheduleHash
	state.CatchUpPolicy = schedulerCatchUpPolicyOrDefault(claim.CatchUpPolicy, current.CatchUpPolicy)
	state.MisfireGraceSeconds = claim.MisfireGraceSeconds
	if claim.Misfire {
		state.LastMisfireAt = timePointer(claim.ClaimedAt)
		state.MisfireCount++
	} else if !claim.CatchUp {
		state.LastMisfireAt = nil
		state.MisfireCount = 0
	}
	state.UpdatedAt = claim.ClaimedAt
	return state
}

func (s *Scheduler) logScheduledFire(job Job, fireID string, scheduledAt time.Time) {
	s.logger.Info(
		"automation.scheduler.job_fired",
		"job_id", job.ID,
		"job_name", job.Name,
		"agent", job.AgentName,
		"schedule_mode", job.Schedule.Mode,
		"fire_id", fireID,
		"scheduled_at", scheduledAt.Format(time.RFC3339Nano),
	)
}

func (s *Scheduler) deferAfterFireLimit(
	ctx context.Context,
	jobID string,
	state SchedulerState,
	fireLimitErr *FireLimitError,
) error {
	if fireLimitErr == nil || fireLimitErr.RetryAt.IsZero() {
		return nil
	}

	target := fireLimitErr.RetryAt.In(s.location)
	if state.NextRunAt != nil && state.NextRunAt.After(target) {
		target = *state.NextRunAt
	}
	state.NextRunAt = timePointer(target)
	state.UpdatedAt = s.now()

	if s.store != nil {
		saved, err := s.store.SaveSchedulerState(persistenceContext(ctx), state)
		if err != nil {
			return fmt.Errorf("automation: defer scheduler after fire limit for job %q: %w", jobID, err)
		}
		state = saved
	}
	s.updateRegistrationState(jobID, state)
	s.logger.Info(
		"automation.scheduler.fire_limit_deferred",
		"job_id", jobID,
		"retry_at", target.Format(time.RFC3339Nano),
		"fires", fireLimitErr.Count,
		"limit", fireLimitErr.Limit,
		"window", fireLimitErr.Window.String(),
	)
	return nil
}

func (s *Scheduler) updateRegistrationState(jobID string, state SchedulerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	registration, exists := s.registrations[jobID]
	if !exists {
		return
	}
	registration.state = state
	s.registrations[jobID] = registration
	if state.NextRunAt == nil || state.NextRunAt.IsZero() {
		if registration.cancel != nil {
			registration.cancel()
		}
		delete(s.registrations, jobID)
	}
}

func (s *Scheduler) defaultCatchUpPolicy(ctx context.Context, job Job) (SchedulerCatchUpPolicy, error) {
	if s.policyForJob == nil {
		return SchedulerCatchUpPolicySkipMissed, nil
	}
	policy, err := s.policyForJob(ctx, job)
	if err != nil {
		return "", fmt.Errorf("automation: resolve scheduler catch-up policy for job %q: %w", job.ID, err)
	}
	return schedulerCatchUpPolicyOrDefault(policy, SchedulerCatchUpPolicySkipMissed), nil
}

func (s *Scheduler) deleteSchedulerState(ctx context.Context, jobID string) error {
	if s.store == nil {
		return nil
	}
	return s.store.DeleteSchedulerState(ctx, jobID)
}

func (s *Scheduler) now() time.Time {
	return s.clock.Now().In(s.location)
}
