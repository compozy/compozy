package automation

import (
	"context"
	"errors"

	"strings"
)

// Runs returns persisted automation run history.
func (m *Manager) Runs(ctx context.Context, query RunQuery) ([]Run, error) {
	return m.ListRuns(ctx, query)
}

// ListRuns returns persisted automation run history using the supplied
// filters.
func (m *Manager) ListRuns(ctx context.Context, query RunQuery) ([]Run, error) {
	if ctx == nil {
		return nil, errors.New("automation: list runs context is required")
	}
	return m.store.ListRuns(ctx, query)
}

// GetRun returns one persisted automation run by id.
func (m *Manager) GetRun(ctx context.Context, id string) (Run, error) {
	if ctx == nil {
		return Run{}, errors.New("automation: get run context is required")
	}
	return m.store.GetRun(ctx, strings.TrimSpace(id))
}

// Status returns aggregate automation lifecycle and next-fire metadata.
func (m *Manager) Status(ctx context.Context) (ManagerStatus, error) {
	if ctx == nil {
		return ManagerStatus{}, errors.New("automation: status context is required")
	}

	jobs, err := m.loadEffectiveJobs(ctx, JobListQuery{})
	if err != nil {
		return ManagerStatus{}, err
	}
	triggers, err := m.loadEffectiveTriggers(ctx, TriggerListQuery{})
	if err != nil {
		return ManagerStatus{}, err
	}

	status := ManagerStatus{
		Running:  m.isRunning(),
		LastSync: m.lastSyncSnapshot(),
		Jobs: ResourceStatus{
			Total:   len(jobs),
			Enabled: countEnabledJobs(jobs),
		},
		Triggers: ResourceStatus{
			Total:   len(triggers),
			Enabled: countEnabledTriggers(triggers),
		},
	}

	scheduler := m.schedulerSnapshot()
	if scheduler != nil {
		status.SchedulerRunning = status.Running
		status.ScheduledJobs = scheduler.States()
		if nextFire, ok := earliestNextFire(status.ScheduledJobs); ok {
			status.NextFire = &nextFire
		}
		return status, nil
	}

	schedulerStates, err := m.store.ListSchedulerStates(ctx)
	if err != nil {
		return ManagerStatus{}, err
	}
	status.ScheduledJobs = scheduledJobStatesFromDurable(schedulerStates)
	if nextFire, ok := earliestNextFire(status.ScheduledJobs); ok {
		status.NextFire = &nextFire
	}

	return status, nil
}

// SetJobEnabled updates the effective enabled state for one job. Config-backed
// jobs use overlay rows while dynamic jobs mutate their persisted definition.
func (m *Manager) SetJobEnabled(ctx context.Context, id string, enabled bool) (Job, error) {
	if ctx == nil {
		return Job{}, errors.New("automation: set job enabled context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.setJobResourceEnabled(ctx, id, enabled)
	}

	stored, err := m.store.GetJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return Job{}, err
	}
	previous, err := m.effectiveJobFromStored(ctx, stored)
	if err != nil {
		return Job{}, err
	}

	switch {
	case isOverlayManagedSource(stored.Source):
		if err := m.persistJobOverlay(ctx, stored, enabled); err != nil {
			return Job{}, err
		}
	default:
		stored.Enabled = enabled
		if _, err := m.store.UpdateJob(ctx, stored); err != nil {
			return Job{}, err
		}
	}

	current, err := m.effectiveJob(ctx, stored.ID)
	if err != nil {
		return Job{}, err
	}
	if err := m.applyJobToRuntime(ctx, current); err != nil {
		if rollbackErr := m.rollbackJobEnabled(ctx, stored, previous.Enabled); rollbackErr != nil {
			return Job{}, errors.Join(err, rollbackErr)
		}
		return Job{}, err
	}
	return current, nil
}

// SetTriggerEnabled updates the effective enabled state for one trigger.
// Config-backed triggers use overlay rows while dynamic triggers mutate their
// persisted definition.
func (m *Manager) SetTriggerEnabled(ctx context.Context, id string, enabled bool) (Trigger, error) {
	if ctx == nil {
		return Trigger{}, errors.New("automation: set trigger enabled context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.setTriggerResourceEnabled(ctx, id, enabled)
	}

	stored, err := m.store.GetTrigger(ctx, strings.TrimSpace(id))
	if err != nil {
		return Trigger{}, err
	}
	previous, err := m.effectiveTriggerFromStored(ctx, stored)
	if err != nil {
		return Trigger{}, err
	}

	switch {
	case isOverlayManagedSource(stored.Source):
		if err := m.persistTriggerOverlay(ctx, stored, enabled); err != nil {
			return Trigger{}, err
		}
	default:
		stored.Enabled = enabled
		if _, err := m.store.UpdateTrigger(ctx, stored); err != nil {
			return Trigger{}, err
		}
	}

	current, err := m.effectiveTrigger(ctx, stored.ID)
	if err != nil {
		return Trigger{}, err
	}
	if err := m.applyTriggerToRuntime(current); err != nil {
		if rollbackErr := m.rollbackTriggerEnabled(ctx, stored, previous.Enabled); rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		return Trigger{}, err
	}
	return current, nil
}
