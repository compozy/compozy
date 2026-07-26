package automation

import (
	"context"
	"errors"

	"strings"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) createTriggerResource(
	ctx context.Context,
	trigger Trigger,
	webhookSecret WebhookSecretWrite,
) (Trigger, error) {
	next := cloneTrigger(trigger)
	if next.Source == "" {
		next.Source = JobSourceDynamic
	}
	if next.Source != JobSourceDynamic {
		return Trigger{}, ErrDefinitionReadOnly
	}
	if strings.TrimSpace(next.ID) == "" {
		next.ID = store.NewID("trg")
	}
	if strings.EqualFold(strings.TrimSpace(next.Event), "webhook") &&
		strings.TrimSpace(next.WebhookID) == "" {
		next.WebhookID = stableConfigID("wbh", next.ID)
	}
	next = applyWebhookSecretRef(next, nil, &webhookSecret)
	next.CreatedAt = m.now().UTC()
	next.UpdatedAt = next.CreatedAt
	if err := requireWebhookSecretRef(next); err != nil {
		return Trigger{}, err
	}
	if err := next.Validate("trigger"); err != nil {
		return Trigger{}, err
	}
	if err := m.validateTriggerLoopTarget(ctx, next); err != nil {
		return Trigger{}, err
	}
	secretState, err := captureWebhookSecretState(ctx, m, next, webhookSecret.Value != nil)
	if err != nil {
		return Trigger{}, err
	}
	if err := m.applyWebhookSecretWrite(ctx, next, webhookSecret); err != nil {
		return Trigger{}, err
	}
	draft := resources.Draft[Trigger]{
		ID:              next.ID,
		Scope:           ResourceScopeForAutomation(next.Scope, next.WorkspaceID),
		ExpectedVersion: 0,
		Spec:            next,
	}
	created, err := m.triggerResources.Put(ctx, m.resourceActorForSource(JobSourceDynamic), draft)
	if err != nil {
		return Trigger{}, errors.Join(err, restoreWebhookSecretStates(persistenceContext(ctx), m, secretState))
	}
	if err := m.applyTriggerResourcesFromStore(ctx); err != nil {
		rollbackErr := errors.Join(
			deleteResourceRecord(persistenceContext(ctx), m.triggerResources, m.resourceActor, created),
			restoreWebhookSecretStates(persistenceContext(ctx), m, secretState),
		)
		if rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		return Trigger{}, err
	}
	return m.effectiveTrigger(ctx, next.ID)
}

func (m *Manager) updateTriggerResource(
	ctx context.Context,
	trigger Trigger,
	webhookSecret *WebhookSecretWrite,
) (Trigger, error) {
	current, err := m.triggerResources.Get(ctx, m.resourceActor, strings.TrimSpace(trigger.ID))
	if err != nil {
		return Trigger{}, err
	}
	if current.Spec.Source != JobSourceDynamic {
		return Trigger{}, ErrDefinitionReadOnly
	}
	next, err := m.nextUpdatedTriggerSpec(ctx, current.Spec, trigger, webhookSecret)
	if err != nil {
		return Trigger{}, err
	}
	previousSecretState, err := captureWebhookSecretState(
		ctx,
		m,
		current.Spec,
		strings.TrimSpace(current.Spec.WebhookSecretRef) != "",
	)
	if err != nil {
		return Trigger{}, err
	}
	nextSecretState, err := captureWebhookSecretState(ctx, m, next, webhookSecret != nil && webhookSecret.Value != nil)
	if err != nil {
		return Trigger{}, err
	}
	if err := m.applyWebhookSecretWritePointer(ctx, next, webhookSecret); err != nil {
		return Trigger{}, err
	}
	updated, err := m.triggerResources.Put(
		ctx,
		currentResourceActor(current.Source, m.resourceActor),
		resources.Draft[Trigger]{
			ID:              current.ID,
			Scope:           ResourceScopeForAutomation(next.Scope, next.WorkspaceID),
			ExpectedVersion: current.Version,
			Spec:            next,
		},
	)
	if err != nil {
		return Trigger{}, triggerMutationError(
			err,
			restoreWebhookSecretStates(persistenceContext(ctx), m, nextSecretState),
		)
	}
	if err := m.deleteSupersededOwnedWebhookSecret(ctx, current.Spec, next); err != nil {
		return Trigger{}, triggerMutationError(
			err,
			m.restoreUpdatedTriggerResource(ctx, current, updated, nextSecretState, previousSecretState),
		)
	}
	if err := m.applyTriggerResourcesFromStore(ctx); err != nil {
		return Trigger{}, triggerMutationError(
			err,
			m.restoreUpdatedTriggerResource(ctx, current, updated, nextSecretState, previousSecretState),
		)
	}
	return m.effectiveTrigger(ctx, current.ID)
}

func triggerMutationError(err error, rollbackErr error) error {
	if rollbackErr != nil {
		return errors.Join(err, rollbackErr)
	}
	return err
}

func (m *Manager) restoreUpdatedTriggerResource(
	ctx context.Context,
	current resources.Record[Trigger],
	updated resources.Record[Trigger],
	secretStates ...webhookSecretState,
) error {
	return errors.Join(
		restoreUpdatedResourceRecord(persistenceContext(ctx), m.triggerResources, m.resourceActor, current, updated),
		restoreWebhookSecretStates(persistenceContext(ctx), m, secretStates...),
	)
}

func (m *Manager) nextUpdatedTriggerSpec(
	ctx context.Context,
	current Trigger,
	trigger Trigger,
	webhookSecret *WebhookSecretWrite,
) (Trigger, error) {
	next := cloneTrigger(trigger)
	next.ID = current.ID
	next.Source = current.Source
	next.CreatedAt = current.CreatedAt.UTC()
	next.UpdatedAt = m.now().UTC()
	next = applyWebhookSecretRef(next, &current, webhookSecret)
	if err := ValidateImmutableTriggerTarget(current, next); err != nil {
		return Trigger{}, err
	}
	if strings.EqualFold(strings.TrimSpace(next.Event), "webhook") &&
		strings.TrimSpace(next.WebhookID) == "" {
		next.WebhookID = stableConfigID("wbh", next.ID)
	}
	if err := requireWebhookSecretRef(next); err != nil {
		return Trigger{}, err
	}
	if err := next.Validate("trigger"); err != nil {
		return Trigger{}, err
	}
	if err := m.validateTriggerLoopTarget(ctx, next); err != nil {
		return Trigger{}, err
	}
	return next, nil
}

func (m *Manager) deleteTriggerResource(ctx context.Context, id string) error {
	current, err := m.triggerResources.Get(ctx, m.resourceActor, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if current.Spec.Source != JobSourceDynamic {
		return ErrDefinitionReadOnly
	}
	if err := m.triggerResources.Delete(
		ctx,
		currentResourceActor(current.Source, m.resourceActor),
		current.ID,
		current.Version,
	); err != nil {
		return err
	}
	secretState, err := captureWebhookSecretState(
		ctx,
		m,
		current.Spec,
		strings.TrimSpace(current.Spec.WebhookSecretRef) != "",
	)
	if err != nil {
		if rollbackErr := recreateDeletedResourceRecord(
			persistenceContext(ctx),
			m.triggerResources,
			m.resourceActor,
			current,
		); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if err := m.deleteOwnedWebhookSecretIfPresent(ctx, current.Spec); err != nil {
		rollbackErr := errors.Join(
			recreateDeletedResourceRecord(
				persistenceContext(ctx),
				m.triggerResources,
				m.resourceActor,
				current,
			),
			restoreWebhookSecretStates(persistenceContext(ctx), m, secretState),
		)
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if err := m.applyTriggerResourcesFromStore(ctx); err != nil {
		rollbackErr := errors.Join(
			recreateDeletedResourceRecord(
				persistenceContext(ctx),
				m.triggerResources,
				m.resourceActor,
				current,
			),
			restoreWebhookSecretStates(persistenceContext(ctx), m, secretState),
		)
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}
