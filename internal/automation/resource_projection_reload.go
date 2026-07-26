package automation

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/resources"
)

func (m *Manager) setJobResourceEnabled(ctx context.Context, id string, enabled bool) (Job, error) {
	current, err := m.projectedJobDefinition(id)
	if err != nil {
		return Job{}, err
	}
	if isOverlayManagedSource(current.Source) {
		if err := m.persistJobOverlay(ctx, current, enabled); err != nil {
			return Job{}, err
		}
		currentEffective, err := m.effectiveJob(ctx, current.ID)
		if err != nil {
			return Job{}, err
		}
		if err := m.applyJobToRuntime(ctx, currentEffective); err != nil {
			return Job{}, err
		}
		return currentEffective, nil
	}

	current.Enabled = enabled
	return m.updateJobResource(ctx, current)
}

func (m *Manager) setTriggerResourceEnabled(ctx context.Context, id string, enabled bool) (Trigger, error) {
	current, err := m.projectedTriggerDefinition(id)
	if err != nil {
		return Trigger{}, err
	}
	if isOverlayManagedSource(current.Source) {
		if err := m.persistTriggerOverlay(ctx, current, enabled); err != nil {
			return Trigger{}, err
		}
		currentEffective, err := m.effectiveTrigger(ctx, current.ID)
		if err != nil {
			return Trigger{}, err
		}
		if err := m.applyTriggerToRuntime(currentEffective); err != nil {
			return Trigger{}, err
		}
		return currentEffective, nil
	}

	current.Enabled = enabled
	return m.updateTriggerResource(ctx, current, nil)
}

func (m *Manager) applyJobResourcesFromStore(ctx context.Context) error {
	records, err := m.jobResources.List(ctx, m.resourceActor, resources.ResourceFilter{Kind: JobResourceKind})
	if err != nil {
		return err
	}
	plan, err := m.BuildJobResourceState(ctx, records)
	if err != nil {
		return err
	}
	return m.ApplyJobResourceState(ctx, plan)
}

func (m *Manager) applyTriggerResourcesFromStore(ctx context.Context) error {
	records, err := m.triggerResources.List(ctx, m.resourceActor, resources.ResourceFilter{Kind: TriggerResourceKind})
	if err != nil {
		return err
	}
	plan, err := m.BuildTriggerResourceState(ctx, records)
	if err != nil {
		return err
	}
	return m.ApplyTriggerResourceState(ctx, plan)
}

func (m *Manager) loadProjectedJobDefinitionsFromStore(ctx context.Context) ([]Job, int64, error) {
	records, err := m.jobResources.List(ctx, m.resourceActor, resources.ResourceFilter{Kind: JobResourceKind})
	if err != nil {
		return nil, 0, err
	}

	jobs := make([]Job, 0, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		job := cloneJob(record.Spec)
		job.ID = strings.TrimSpace(record.ID)
		job.CreatedAt = record.CreatedAt.UTC()
		job.UpdatedAt = record.UpdatedAt.UTC()
		jobs = append(jobs, job)
	}
	sortJobs(jobs)
	return jobs, revision, nil
}

func (m *Manager) loadProjectedTriggerDefinitionsFromStore(ctx context.Context) ([]Trigger, int64, error) {
	records, err := m.triggerResources.List(ctx, m.resourceActor, resources.ResourceFilter{Kind: TriggerResourceKind})
	if err != nil {
		return nil, 0, err
	}

	triggers := make([]Trigger, 0, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		trigger := cloneTrigger(record.Spec)
		trigger.ID = strings.TrimSpace(record.ID)
		trigger.CreatedAt = record.CreatedAt.UTC()
		trigger.UpdatedAt = record.UpdatedAt.UTC()
		if strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") &&
			strings.TrimSpace(trigger.WebhookID) == "" {
			trigger.WebhookID = stableConfigID("wbh", trigger.ID)
		}
		triggers = append(triggers, trigger)
	}
	sortTriggers(triggers)
	return triggers, revision, nil
}

func (m *Manager) triggerResourceReconcile(ctx context.Context, kind resources.ResourceKind) error {
	if m == nil || m.resourceTrigger == nil {
		return nil
	}
	return m.resourceTrigger(ctx, kind, resources.ReconcileReasonWrite)
}
