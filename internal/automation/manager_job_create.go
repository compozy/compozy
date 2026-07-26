package automation

import (
	"context"
	"errors"

	"github.com/compozy/agh/internal/store"
)

// CreateJob stores a new dynamic automation job and registers it into the
// runtime when the scheduler is active.
func (m *Manager) CreateJob(ctx context.Context, job Job) (Job, error) {
	prepared, err := m.prepareJobForCreate(ctx, job)
	if err != nil {
		return Job{}, err
	}
	return m.createPreparedJob(ctx, prepared)
}

func (m *Manager) prepareJobForCreate(ctx context.Context, job Job) (Job, error) {
	if ctx == nil {
		return Job{}, errors.New("automation: create job context is required")
	}
	next := cloneJob(job)
	if next.Source == "" {
		next.Source = JobSourceDynamic
	}
	if next.Source != JobSourceDynamic {
		return Job{}, ErrDefinitionReadOnly
	}
	if next.ID == "" {
		next.ID = store.NewID("job")
	}
	next.CreatedAt = m.now().UTC()
	next.UpdatedAt = next.CreatedAt
	if err := m.validateJobDefinition(ctx, next); err != nil {
		return Job{}, err
	}
	return next, nil
}

func (m *Manager) createPreparedJob(ctx context.Context, prepared Job) (Job, error) {
	if m.resourceDefinitionsEnabled() {
		return m.createPreparedJobResource(ctx, prepared)
	}

	created, err := m.store.CreateJob(ctx, prepared)
	if err != nil {
		return Job{}, err
	}

	current, err := m.effectiveJobFromStored(ctx, created)
	if err != nil {
		return Job{}, errors.Join(err, m.cleanupCreatedJob(ctx, created.ID))
	}
	if err := m.applyJobToRuntime(ctx, current); err != nil {
		return Job{}, errors.Join(err, m.cleanupCreatedJob(ctx, created.ID))
	}

	return current, nil
}
