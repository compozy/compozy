package automation

import (
	"context"
	"errors"
	"fmt"

	"sort"
	"strings"
)

// Register adds a new scheduled job registration.
func (s *Scheduler) Register(ctx context.Context, job Job) (ScheduledJobState, error) {
	if ctx == nil {
		return ScheduledJobState{}, errors.New("automation: scheduler register context is required")
	}
	normalized, err := normalizeScheduledJob(job)
	if err != nil {
		return ScheduledJobState{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureMutable(); err != nil {
		return ScheduledJobState{}, err
	}
	if _, exists := s.registrations[normalized.ID]; exists {
		return ScheduledJobState{}, fmt.Errorf("%w: %s", ErrScheduledJobAlreadyRegistered, normalized.ID)
	}

	return s.registerLocked(ctx, normalized)
}

// Update replaces an existing scheduled job registration.
func (s *Scheduler) Update(ctx context.Context, job Job) (ScheduledJobState, error) {
	if ctx == nil {
		return ScheduledJobState{}, errors.New("automation: scheduler update context is required")
	}
	normalized, err := normalizeScheduledJob(job)
	if err != nil {
		return ScheduledJobState{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureMutable(); err != nil {
		return ScheduledJobState{}, err
	}
	current, exists := s.registrations[normalized.ID]
	if !exists {
		return ScheduledJobState{}, fmt.Errorf("%w: %s", ErrScheduledJobNotFound, normalized.ID)
	}

	if !normalized.Enabled {
		s.unregisterLocked(normalized.ID, current)
		if err := s.deleteSchedulerState(ctx, normalized.ID); err != nil {
			return ScheduledJobState{}, err
		}
		return unregisteredJobState(normalized.ID), nil
	}

	plan, err := s.buildSchedulePlan(normalized)
	if err != nil {
		return ScheduledJobState{}, err
	}
	if !plan.register {
		s.unregisterLocked(normalized.ID, current)
		return unregisteredJobState(normalized.ID), nil
	}

	state, err := s.reconcileSchedulerState(ctx, normalized, plan)
	if err != nil {
		return ScheduledJobState{}, err
	}

	registeredAt := s.now()
	if current.cancel != nil {
		current.cancel()
	}
	s.registrations[normalized.ID] = scheduledRegistration{
		definition:   normalized,
		registeredAt: registeredAt,
		state:        state,
	}
	if s.started {
		s.startJobLoopLocked(normalized.ID)
	}
	s.logger.Info(
		"automation.scheduler.updated",
		"job_id",
		normalized.ID,
		"job_name",
		normalized.Name,
		"schedule_mode",
		normalized.Schedule.Mode,
	)
	return s.snapshotLocked(normalized.ID, s.registrations[normalized.ID]), nil
}

// Unregister removes a scheduled job registration.
func (s *Scheduler) Unregister(ctx context.Context, jobID string) error {
	if ctx == nil {
		return errors.New("automation: scheduler unregister context is required")
	}
	trimmedID := strings.TrimSpace(jobID)
	if trimmedID == "" {
		return errors.New("automation: scheduled job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.registrations[trimmedID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrScheduledJobNotFound, trimmedID)
	}

	s.unregisterLocked(trimmedID, current)
	return s.deleteSchedulerState(ctx, trimmedID)
}

// State returns runtime metadata for one scheduled job.
func (s *Scheduler) State(jobID string) (ScheduledJobState, error) {
	trimmedID := strings.TrimSpace(jobID)
	if trimmedID == "" {
		return ScheduledJobState{}, errors.New("automation: scheduled job id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	current, exists := s.registrations[trimmedID]
	if !exists {
		return ScheduledJobState{}, fmt.Errorf("%w: %s", ErrScheduledJobNotFound, trimmedID)
	}

	return s.snapshotLocked(trimmedID, current), nil
}

// States returns runtime metadata for every registered scheduled job.
func (s *Scheduler) States() []ScheduledJobState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]ScheduledJobState, 0, len(s.registrations))
	for jobID, registration := range s.registrations {
		states = append(states, s.snapshotLocked(jobID, registration))
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].JobID < states[j].JobID
	})
	return states
}

func (s *Scheduler) ensureMutable() error {
	if s.stopped {
		return ErrSchedulerStopped
	}
	return nil
}

func (s *Scheduler) registerLocked(ctx context.Context, job Job) (ScheduledJobState, error) {
	if !job.Enabled {
		if err := s.deleteSchedulerState(ctx, job.ID); err != nil {
			return ScheduledJobState{}, err
		}
		return unregisteredJobState(job.ID), nil
	}

	plan, err := s.buildSchedulePlan(job)
	if err != nil {
		return ScheduledJobState{}, err
	}
	if !plan.register {
		s.logger.Info("automation.scheduler.skipped_past_one_time_job", "job_id", job.ID, "job_name", job.Name)
		state, err := s.reconcileSchedulerState(ctx, job, plan)
		if err != nil {
			return ScheduledJobState{}, err
		}
		if s.store != nil {
			return stateFromDurableState(state, false), nil
		}
		return unregisteredJobState(job.ID), nil
	}

	state, err := s.reconcileSchedulerState(ctx, job, plan)
	if err != nil {
		return ScheduledJobState{}, err
	}
	registeredAt := s.now()
	registration := scheduledRegistration{
		definition:   job,
		registeredAt: registeredAt,
		state:        state,
	}
	s.registrations[job.ID] = registration
	if s.started {
		s.startJobLoopLocked(job.ID)
	}
	s.logger.Info(
		"automation.scheduler.registered",
		"job_id",
		job.ID,
		"job_name",
		job.Name,
		"schedule_mode",
		job.Schedule.Mode,
	)
	return s.snapshotLocked(job.ID, registration), nil
}

func (s *Scheduler) unregisterLocked(jobID string, registration scheduledRegistration) {
	if registration.cancel != nil {
		registration.cancel()
	}
	delete(s.registrations, jobID)
	s.logger.Info("automation.scheduler.unregistered", "job_id", jobID, "job_name", registration.definition.Name)
}

func (s *Scheduler) snapshotLocked(jobID string, registration scheduledRegistration) ScheduledJobState {
	state := stateFromDurableState(registration.state, true)
	if state.JobID == "" {
		state.JobID = jobID
	}
	if state.NextRun == nil {
		predicted := predictNextRun(registration.definition, registration.registeredAt, s.location)
		if !predicted.IsZero() {
			state.NextRun = timePointer(predicted)
		}
	}
	return state
}

func stateFromDurableState(durable SchedulerState, registered bool) ScheduledJobState {
	state := ScheduledJobState{
		JobID:               durable.JobID,
		Registered:          registered,
		NextRun:             durable.NextRunAt,
		LastRun:             durable.LastRunAt,
		LastScheduledAt:     durable.LastScheduledAt,
		LastFireID:          durable.LastFireID,
		CatchUpPolicy:       durable.CatchUpPolicy,
		MisfireGraceSeconds: durable.MisfireGraceSeconds,
		LastMisfireAt:       durable.LastMisfireAt,
		MisfireCount:        durable.MisfireCount,
	}
	if strings.TrimSpace(durable.JobID) != "" {
		state.JobID = durable.JobID
		durableCopy := durable
		state.Durable = &durableCopy
	}
	return state
}
