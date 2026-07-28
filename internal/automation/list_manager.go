package automation

import "context"

func (m *Manager) loadEffectiveJobs(ctx context.Context, query JobListQuery) ([]Job, error) {
	if m.resourceDefinitionsEnabled() {
		return m.applyJobOverlays(ctx, m.projectedJobDefinitions())
	}
	jobs, err := m.store.ListJobDefinitions(ctx, query.Source)
	if err != nil {
		return nil, err
	}
	return m.applyJobOverlays(ctx, jobs)
}

func (m *Manager) loadEffectiveTriggers(ctx context.Context, query TriggerListQuery) ([]Trigger, error) {
	if m.resourceDefinitionsEnabled() {
		return m.applyTriggerOverlays(ctx, m.projectedTriggerDefinitions())
	}
	triggers, err := m.store.ListTriggerDefinitions(ctx, query.Source)
	if err != nil {
		return nil, err
	}
	return m.applyTriggerOverlays(ctx, triggers)
}

func (m *Manager) loadEffectiveJobPage(ctx context.Context, query JobListQuery) (JobListPage, error) {
	if m.resourceDefinitionsEnabled() {
		return m.listProjectedJobCatalog(ctx, query)
	}
	return m.store.ListJobs(ctx, query)
}

func (m *Manager) loadEffectiveTriggerPage(
	ctx context.Context,
	query TriggerListQuery,
) (TriggerListPage, error) {
	if m.resourceDefinitionsEnabled() {
		return m.listProjectedTriggerCatalog(ctx, query)
	}
	return m.store.ListTriggers(ctx, query)
}

func (m *Manager) applyJobOverlays(ctx context.Context, jobs []Job) ([]Job, error) {
	overlayByID, err := m.jobEnabledOverrides(ctx)
	if err != nil {
		return nil, err
	}

	effective := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		next := cloneJob(job)
		if isOverlayManagedSource(next.Source) {
			if enabled, ok := overlayByID[next.ID]; ok {
				next.Enabled = enabled
			}
		}
		effective = append(effective, next)
	}
	return effective, nil
}

func (m *Manager) applyTriggerOverlays(ctx context.Context, triggers []Trigger) ([]Trigger, error) {
	overlayByID, err := m.triggerEnabledOverrides(ctx)
	if err != nil {
		return nil, err
	}

	effective := make([]Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		next := cloneTrigger(trigger)
		if isOverlayManagedSource(next.Source) {
			if enabled, ok := overlayByID[next.ID]; ok {
				next.Enabled = enabled
			}
		}
		effective = append(effective, next)
	}
	return effective, nil
}
