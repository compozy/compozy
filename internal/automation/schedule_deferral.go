package automation

import (
	"context"
	"errors"
	"time"
)

func schedulerDueAt(state SchedulerState) *time.Time {
	if state.DeferredUntil != nil {
		return state.DeferredUntil
	}
	if state.NextRunAt == nil || state.NextRunAt.IsZero() {
		return nil
	}
	return state.NextRunAt
}

func (s *Scheduler) setScheduledDeferral(
	ctx context.Context,
	claimed scheduledJobClaimResult,
	retryAt *time.Time,
) error {
	state := claimed.state
	state.DeferredUntil = cloneTimePointer(retryAt)
	if s.store != nil {
		persistCtx, cancel := persistenceContext(ctx)
		defer cancel()
		saved, err := s.store.SetScheduledDeferral(persistCtx, claimed.claim, retryAt)
		if errors.Is(err, ErrScheduledFireAlreadyClaimed) || errors.Is(err, ErrSchedulerStateNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		state = saved
	}
	if retryAt == nil && state.NextRunAt != nil && !state.NextRunAt.After(s.now()) && s.store != nil {
		registration, ok := s.registrationSnapshot(claimed.claim.JobID)
		if ok && registration.state.ScheduleHash == state.ScheduleHash {
			reconciled, err := s.reconcileMissedSchedulerState(ctx, registration.definition, state, s.now())
			if err != nil {
				return err
			}
			state = reconciled
		}
	}
	s.updateRegistrationState(claimed.claim.JobID, state)
	return nil
}

func (s *Scheduler) setCapacityWaiting(jobID, hash string, waiting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	registration, exists := s.registrations[jobID]
	if !exists || registration.state.ScheduleHash != hash {
		return
	}
	registration.capacityWaiting = waiting
	s.registrations[jobID] = registration
}
