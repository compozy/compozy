package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"time"

	gocron "github.com/go-co-op/gocron/v2"
)

func (s *Scheduler) buildSchedulePlan(job Job) (schedulePlan, error) {
	now := s.now()
	if job.Schedule == nil {
		return schedulePlan{}, errors.New("automation: job schedule is required")
	}

	switch job.Schedule.Mode {
	case ScheduleModeCron:
		cronImpl := gocron.NewDefaultCron(false)
		expr := strings.TrimSpace(job.Schedule.Expr)
		if err := cronImpl.IsValid(expr, s.location, now); err != nil {
			return schedulePlan{}, fmt.Errorf("automation: validate cron schedule for job %q: %w", job.ID, err)
		}
		return schedulePlan{
			register: true,
			nextRun:  cronImpl.Next(now),
		}, nil
	case ScheduleModeEvery:
		interval, err := time.ParseDuration(strings.TrimSpace(job.Schedule.Interval))
		if err != nil {
			return schedulePlan{}, fmt.Errorf("automation: parse interval schedule for job %q: %w", job.ID, err)
		}
		return schedulePlan{
			register: true,
			nextRun:  now.Add(interval),
		}, nil
	case ScheduleModeAt:
		atTime, err := time.Parse(time.RFC3339, strings.TrimSpace(job.Schedule.Time))
		if err != nil {
			return schedulePlan{}, fmt.Errorf("automation: parse one-time schedule for job %q: %w", job.ID, err)
		}
		if !atTime.After(now) {
			return schedulePlan{register: false}, nil
		}
		return schedulePlan{
			register: true,
			nextRun:  atTime,
		}, nil
	default:
		return schedulePlan{}, fmt.Errorf("automation: unsupported schedule mode %q", job.Schedule.Mode)
	}
}

func (s *Scheduler) startJobLoopLocked(jobID string) {
	registration, exists := s.registrations[jobID]
	if !exists || s.runtime == nil {
		return
	}
	if registration.cancel != nil {
		registration.cancel()
	}

	runtime := s.runtime
	jobCtx, cancel := context.WithCancel(runtime.ctx)
	registration.cancel = cancel
	s.registrations[jobID] = registration
	runtime.workers.Go(func() {
		s.runJobLoop(jobCtx, cancel, jobID)
	})
}

func (s *Scheduler) runJobLoop(ctx context.Context, cancel context.CancelFunc, jobID string) {
	defer cancel()

	for {
		registration, ok := s.registrationSnapshot(jobID)
		if !ok {
			return
		}
		if schedulerDueAt(registration.state) == nil {
			return
		}

		delay := max(schedulerDueAt(registration.state).Sub(s.now()), 0)
		var capacityAvailable <-chan struct{}
		if registration.state.DeferredUntil != nil && registration.capacityWaiting {
			if notifier, ok := s.dispatcher.(interface{ CapacityAvailable() <-chan struct{} }); ok {
				capacityAvailable = notifier.CapacityAvailable()
			}
		}
		timer := s.clock.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-capacityAvailable:
			timer.Stop()
		case <-timer.Chan():
		}

		if err := s.executeScheduledJob(ctx, jobID); err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("automation.scheduler.dispatch_failed", "job_id", jobID, "error", err)
		}
	}
}

func (s *Scheduler) registrationSnapshot(jobID string) (scheduledRegistration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	registration, ok := s.registrations[jobID]
	return registration, ok
}

func (s *Scheduler) executeScheduledJob(ctx context.Context, jobID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	registration, ok := s.registrationSnapshot(jobID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrScheduledJobNotFound, jobID)
	}
	if schedulerDueAt(registration.state) == nil {
		return nil
	}

	job := registration.definition
	claimed, err := s.claimScheduledJob(ctx, registration)
	if err != nil {
		if errors.Is(err, ErrScheduledFireAlreadyClaimed) {
			s.updateRegistrationState(job.ID, registration.state)
			return nil
		}
		return err
	}
	if claimed.skipped {
		s.updateRegistrationState(job.ID, claimed.state)
		s.logScheduledSkip(job, claimed.claim, claimed.skipReason)
		return nil
	}

	s.logScheduledFire(job, claimed.claim.FireID, claimed.claim.ScheduledAt)

	run, err := s.dispatcher.Dispatch(ctx, DispatchRequest{
		Kind:          DispatchKindSchedule,
		Job:           &job,
		ReservedRun:   claimed.reservedRun,
		ScheduledAt:   timePointer(claimed.claim.ScheduledAt),
		CatchUp:       claimed.claim.CatchUp,
		CatchUpPolicy: claimed.state.CatchUpPolicy,
	})
	if errors.Is(err, ErrConcurrencyLimitReached) || errors.Is(err, errScheduledAdmissionDeferred) {
		s.setCapacityWaiting(job.ID, claimed.state.ScheduleHash, errors.Is(err, ErrConcurrencyLimitReached))
		return s.setScheduledDeferral(ctx, claimed, timePointer(s.now().Add(time.Second)))
	}
	if fireLimitErr, ok := errors.AsType[*FireLimitError](err); ok {
		if adjustErr := s.deferAfterFireLimit(ctx, job.ID, claimed.state, fireLimitErr); adjustErr != nil {
			return errors.Join(err, adjustErr)
		}
		return nil
	}
	if claimed.state.DeferredUntil != nil {
		if clearErr := s.setScheduledDeferral(ctx, claimed, nil); clearErr != nil {
			return errors.Join(err, clearErr)
		}
	}
	if claimed.state.DeferredUntil == nil {
		s.updateRegistrationState(job.ID, claimed.state)
	}
	if err != nil && s.store != nil {
		runID := strings.TrimSpace(claimed.claim.RunID)
		if run != nil && strings.TrimSpace(run.ID) != "" {
			runID = run.ID
		}
		persistCtx1, cancelPersist1 := persistenceContext(ctx)
		defer cancelPersist1()
		if _, recordErr := s.store.RecordRunDeliveryError(persistCtx1, runID, err); recordErr != nil {
			err = errors.Join(err, recordErr)
		}
	}
	return err
}
