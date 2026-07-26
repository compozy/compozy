package automation

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

func (m *Manager) syncManagedResourceDefinitions(
	ctx context.Context,
	source JobSource,
	desiredJobs []Job,
	desiredTriggers []Trigger,
) (SyncStats, error) {
	actor := m.resourceActorForSource(source)

	jobsSynced, jobsRemoved, err := m.syncJobResourcesForSource(ctx, actor, desiredJobs)
	if err != nil {
		return SyncStats{}, err
	}
	triggersSynced, triggersRemoved, err := m.syncTriggerResourcesForSource(
		ctx,
		actor,
		desiredTriggers,
	)
	if err != nil {
		return SyncStats{}, err
	}

	if err := m.triggerResourceReconcile(ctx, JobResourceKind); err != nil {
		return SyncStats{}, err
	}
	if err := m.triggerResourceReconcile(ctx, TriggerResourceKind); err != nil {
		return SyncStats{}, err
	}

	stats := SyncStats{
		JobsSynced:      jobsSynced,
		TriggersSynced:  triggersSynced,
		JobsRemoved:     jobsRemoved,
		TriggersRemoved: triggersRemoved,
		SyncedAt:        m.now().UTC(),
	}
	m.logger.Info(
		"automation.managed.resource_sync",
		"source", source,
		"jobs_synced", stats.JobsSynced,
		"triggers_synced", stats.TriggersSynced,
		"jobs_removed", stats.JobsRemoved,
		"triggers_removed", stats.TriggersRemoved,
	)
	return stats, nil
}

func (m *Manager) syncJobResourcesForSource(
	ctx context.Context,
	actor resources.MutationActor,
	desired []Job,
) (int, int, error) {
	source := actor.Source
	current, err := m.jobResources.List(ctx, actor, resources.ResourceFilter{
		Kind:   JobResourceKind,
		Source: &source,
	})
	if err != nil {
		return 0, 0, err
	}
	currentByID := make(map[string]resources.Record[Job], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	synced := 0
	for _, job := range desired {
		next := cloneJob(job)
		next.Source = JobSource(strings.TrimSpace(string(next.Source)))
		if strings.TrimSpace(next.ID) == "" {
			return 0, 0, errors.New("automation: managed job id is required")
		}
		if err := next.Validate("job"); err != nil {
			return 0, 0, err
		}
		if err := m.validateJobLoopTarget(ctx, next); err != nil {
			return 0, 0, err
		}
		currentRecord, exists := currentByID[next.ID]
		if exists && currentRecord.Scope == ResourceScopeForAutomation(next.Scope, next.WorkspaceID) &&
			sameJobDefinition(currentRecord.Spec, next) {
			delete(currentByID, next.ID)
			synced++
			continue
		}

		expectedVersion := int64(0)
		if exists {
			expectedVersion = currentRecord.Version
		}
		if _, err := m.jobResources.Put(ctx, actor, resources.Draft[Job]{
			ID:              next.ID,
			Scope:           ResourceScopeForAutomation(next.Scope, next.WorkspaceID),
			ExpectedVersion: expectedVersion,
			Spec:            next,
		}); err != nil {
			return 0, 0, err
		}
		delete(currentByID, next.ID)
		synced++
	}

	removed := 0
	for _, stale := range currentByID {
		if err := m.jobResources.Delete(ctx, actor, stale.ID, stale.Version); err != nil {
			return 0, 0, err
		}
		removed++
	}
	return synced, removed, nil
}

func (m *Manager) syncTriggerResourcesForSource(
	ctx context.Context,
	actor resources.MutationActor,
	desired []Trigger,
) (int, int, error) {
	source := actor.Source
	current, err := m.triggerResources.List(ctx, actor, resources.ResourceFilter{
		Kind:   TriggerResourceKind,
		Source: &source,
	})
	if err != nil {
		return 0, 0, err
	}
	currentByID := make(map[string]resources.Record[Trigger], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	synced := 0
	for _, trigger := range desired {
		next := cloneTrigger(trigger)
		if strings.TrimSpace(next.ID) == "" {
			return 0, 0, errors.New("automation: managed trigger id is required")
		}
		if strings.EqualFold(strings.TrimSpace(next.Event), "webhook") && strings.TrimSpace(next.WebhookID) == "" {
			next.WebhookID = stableConfigID("wbh", next.ID)
		}
		if err := requireWebhookSecretRef(next); err != nil {
			return 0, 0, err
		}
		if err := next.Validate("trigger"); err != nil {
			return 0, 0, err
		}
		if err := m.validateTriggerLoopTarget(ctx, next); err != nil {
			return 0, 0, err
		}

		currentRecord, exists := currentByID[next.ID]
		if exists && currentRecord.Scope == ResourceScopeForAutomation(next.Scope, next.WorkspaceID) &&
			sameTriggerDefinition(currentRecord.Spec, next) {
			delete(currentByID, next.ID)
			synced++
			continue
		}

		expectedVersion := int64(0)
		if exists {
			expectedVersion = currentRecord.Version
		}
		if _, err := m.triggerResources.Put(ctx, actor, resources.Draft[Trigger]{
			ID:              next.ID,
			Scope:           ResourceScopeForAutomation(next.Scope, next.WorkspaceID),
			ExpectedVersion: expectedVersion,
			Spec:            next,
		}); err != nil {
			return 0, 0, err
		}
		delete(currentByID, next.ID)
		synced++
	}

	removed := 0
	for _, stale := range currentByID {
		if err := m.triggerResources.Delete(ctx, actor, stale.ID, stale.Version); err != nil {
			return 0, 0, err
		}
		if err := m.deleteOwnedWebhookSecretIfPresent(ctx, stale.Spec); err != nil {
			return 0, 0, err
		}
		removed++
	}
	return synced, removed, nil
}
