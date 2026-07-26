package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	modelpkg "github.com/compozy/agh/internal/automation/model"
)

// Jobs returns overlay-aware job definitions from persistence.
func (m *Manager) Jobs(ctx context.Context) ([]Job, error) {
	return m.loadEffectiveJobs(ctx, JobListQuery{})
}

// ListJobs returns overlay-aware job definitions using the supplied filters.
func (m *Manager) ListJobs(ctx context.Context, query JobListQuery) (JobListPage, error) {
	if ctx == nil {
		return JobListPage{}, errors.New("automation: list jobs context is required")
	}
	return m.loadEffectiveJobPage(ctx, modelpkg.JobListQueryWithDefaultLimit(query))
}

// GetJob returns one overlay-aware job definition by id.
func (m *Manager) GetJob(ctx context.Context, id string) (Job, error) {
	if ctx == nil {
		return Job{}, errors.New("automation: get job context is required")
	}
	return m.effectiveJob(ctx, strings.TrimSpace(id))
}

// UpdateJob replaces one existing dynamic automation job definition.
func (m *Manager) UpdateJob(ctx context.Context, job Job) (Job, error) {
	if ctx == nil {
		return Job{}, errors.New("automation: update job context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.updateJobResource(ctx, job)
	}

	currentStored, err := m.store.GetJob(ctx, strings.TrimSpace(job.ID))
	if err != nil {
		return Job{}, err
	}
	if currentStored.Source != JobSourceDynamic {
		return Job{}, ErrDefinitionReadOnly
	}

	previousEffective, err := m.effectiveJobFromStored(ctx, currentStored)
	if err != nil {
		return Job{}, err
	}

	next := cloneJob(job)
	next.ID = currentStored.ID
	next.Source = currentStored.Source
	next.CreatedAt = currentStored.CreatedAt
	if err := ValidateImmutableJobTarget(currentStored, next); err != nil {
		return Job{}, err
	}
	if err := m.validateJobDefinition(ctx, next); err != nil {
		return Job{}, err
	}

	updatedStored, err := m.store.UpdateJob(ctx, next)
	if err != nil {
		return Job{}, err
	}

	currentEffective, err := m.effectiveJobFromStored(ctx, updatedStored)
	if err != nil {
		if _, rollbackErr := m.store.UpdateJob(ctx, currentStored); rollbackErr != nil {
			return Job{}, errors.Join(
				err,
				fmt.Errorf("automation: restore job %q after load failure: %w", currentStored.ID, rollbackErr),
			)
		}
		return Job{}, err
	}
	if err := m.applyJobToRuntime(ctx, currentEffective); err != nil {
		if _, rollbackErr := m.store.UpdateJob(ctx, currentStored); rollbackErr != nil {
			return Job{}, errors.Join(err, rollbackErr)
		}
		if restoreErr := m.applyJobToRuntime(ctx, previousEffective); restoreErr != nil {
			return Job{}, errors.Join(err, restoreErr)
		}
		return Job{}, err
	}

	return currentEffective, nil
}

// DeleteJob removes one dynamic automation job definition and unregisters it
// from the runtime scheduler when needed.
func (m *Manager) DeleteJob(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("automation: delete job context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.deleteJobResource(ctx, id)
	}

	currentStored, err := m.store.GetJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if currentStored.Source != JobSourceDynamic {
		return ErrDefinitionReadOnly
	}

	previousEffective, err := m.effectiveJobFromStored(ctx, currentStored)
	if err != nil {
		return err
	}

	scheduler := m.schedulerSnapshot()
	if scheduler != nil {
		if err := scheduler.Unregister(ctx, currentStored.ID); err != nil && !errors.Is(err, ErrScheduledJobNotFound) {
			return err
		}
	}

	if err := m.store.DeleteJob(ctx, currentStored.ID); err != nil {
		if restoreErr := m.applyJobToRuntime(ctx, previousEffective); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}

	return nil
}

// TriggerJob forces one immediate manual execution through the shared
// dispatcher path.
func (m *Manager) TriggerJob(ctx context.Context, id string) (Run, error) {
	return m.triggerJob(ctx, id, nil)
}

// TriggerJobWithPayload forces one immediate manual execution and exposes the
// caller-supplied payload to job lifecycle hooks.
func (m *Manager) TriggerJobWithPayload(ctx context.Context, id string, payload map[string]any) (Run, error) {
	return m.triggerJob(ctx, id, payload)
}

func (m *Manager) triggerJob(ctx context.Context, id string, payload map[string]any) (Run, error) {
	if ctx == nil {
		return Run{}, errors.New("automation: trigger job context is required")
	}

	job, err := m.effectiveJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return Run{}, err
	}

	dispatchCtx := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		dispatchCtx, cancel = context.WithDeadline(dispatchCtx, deadline)
		defer cancel()
	}

	run, err := m.dispatcher.Dispatch(dispatchCtx, DispatchRequest{
		Kind:    DispatchKindManual,
		Job:     &job,
		Payload: cloneJSONMap(payload),
	})
	if err != nil {
		if run != nil {
			return *run, err
		}
		return Run{}, err
	}
	if run == nil {
		return Run{}, errors.New("automation: manual job dispatch returned no run")
	}
	return *run, nil
}
