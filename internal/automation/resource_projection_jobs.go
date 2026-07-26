package automation

import (
	"context"
	"errors"

	"strings"

	"github.com/compozy/agh/internal/resources"
)

func (m *Manager) resourceDefinitionsEnabled() bool {
	return m != nil && m.jobResources != nil && m.triggerResources != nil
}

func defaultAutomationResourceActor() resources.MutationActor {
	return resources.MutationActor{
		Kind:     resources.MutationActorKindDaemon,
		ID:       "automation-resource-sync",
		Source:   resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "automation"},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (m *Manager) resourceActorForSource(source JobSource) resources.MutationActor {
	actor := m.resourceActor
	if actor.Kind == "" {
		actor = defaultAutomationResourceActor()
	}
	sourceID := "automation." + strings.TrimSpace(string(source))
	actor.ID = sourceID
	actor.Source = resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: sourceID}
	return actor
}

func (m *Manager) createPreparedJobResource(ctx context.Context, prepared Job) (Job, error) {
	created, err := m.jobResources.Put(ctx, m.resourceActorForSource(JobSourceDynamic), resources.Draft[Job]{
		ID:              prepared.ID,
		Scope:           ResourceScopeForAutomation(prepared.Scope, prepared.WorkspaceID),
		ExpectedVersion: 0,
		Spec:            prepared,
	})
	if err != nil {
		return Job{}, err
	}
	if err := m.applyJobResourcesFromStore(ctx); err != nil {
		if rollbackErr := deleteResourceRecord(
			persistenceContext(ctx),
			m.jobResources,
			m.resourceActor,
			created,
		); rollbackErr != nil {
			return Job{}, errors.Join(err, rollbackErr)
		}
		return Job{}, err
	}
	return m.effectiveJob(ctx, prepared.ID)
}

func (m *Manager) updateJobResource(ctx context.Context, job Job) (Job, error) {
	current, err := m.jobResources.Get(ctx, m.resourceActor, strings.TrimSpace(job.ID))
	if err != nil {
		return Job{}, err
	}
	if current.Spec.Source != JobSourceDynamic {
		return Job{}, ErrDefinitionReadOnly
	}

	next := cloneJob(job)
	next.ID = current.ID
	next.Source = current.Spec.Source
	next.CreatedAt = current.CreatedAt.UTC()
	next.UpdatedAt = m.now().UTC()
	if err := ValidateImmutableJobTarget(current.Spec, next); err != nil {
		return Job{}, err
	}
	if err := m.validateJobDefinition(ctx, next); err != nil {
		return Job{}, err
	}
	updated, err := m.jobResources.Put(ctx, currentResourceActor(current.Source, m.resourceActor), resources.Draft[Job]{
		ID:              current.ID,
		Scope:           ResourceScopeForAutomation(next.Scope, next.WorkspaceID),
		ExpectedVersion: current.Version,
		Spec:            next,
	})
	if err != nil {
		return Job{}, err
	}
	if err := m.applyJobResourcesFromStore(ctx); err != nil {
		if rollbackErr := restoreUpdatedResourceRecord(
			persistenceContext(ctx),
			m.jobResources,
			m.resourceActor,
			current,
			updated,
		); rollbackErr != nil {
			return Job{}, errors.Join(err, rollbackErr)
		}
		return Job{}, err
	}
	return m.effectiveJob(ctx, current.ID)
}

func (m *Manager) deleteJobResource(ctx context.Context, id string) error {
	current, err := m.jobResources.Get(ctx, m.resourceActor, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if current.Spec.Source != JobSourceDynamic {
		return ErrDefinitionReadOnly
	}
	if err := m.jobResources.Delete(
		ctx,
		currentResourceActor(current.Source, m.resourceActor),
		current.ID,
		current.Version,
	); err != nil {
		return err
	}
	if err := m.applyJobResourcesFromStore(ctx); err != nil {
		if rollbackErr := recreateDeletedResourceRecord(
			persistenceContext(ctx),
			m.jobResources,
			m.resourceActor,
			current,
		); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}
