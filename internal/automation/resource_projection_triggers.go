package automation

import (
	"context"
	"errors"

	"strings"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
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
		generatedID, err := store.NewID("trg")
		if err != nil {
			return Trigger{}, errors.Join(errors.New("automation: generate trigger resource id"), err)
		}
		next.ID = generatedID
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
	if err := ValidateTriggerAgentName(next, "trigger"); err != nil {
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
		persistCtx1, cancelPersist1 := persistenceContext(ctx)
		defer cancelPersist1()
		return Trigger{}, errors.Join(err, restoreWebhookSecretStates(persistCtx1, m, secretState))
	}
	if err := m.applyTriggerResourcesFromStore(ctx); err != nil {
		persistCtx2, cancelPersist2 := persistenceContext(ctx)
		defer cancelPersist2()
		rollbackErr := errors.Join(
			deleteResourceRecord(persistCtx2, m.triggerResources, m.resourceActor, created),
			restoreWebhookSecretStates(persistCtx2, m, secretState),
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
		persistCtx3, cancelPersist3 := persistenceContext(ctx)
		defer cancelPersist3()
		return Trigger{}, triggerMutationError(
			err,
			restoreWebhookSecretStates(persistCtx3, m, nextSecretState),
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
	persistCtx4, cancelPersist4 := persistenceContext(ctx)
	defer cancelPersist4()
	return errors.Join(
		restoreUpdatedResourceRecord(persistCtx4, m.triggerResources, m.resourceActor, current, updated),
		restoreWebhookSecretStates(persistCtx4, m, secretStates...),
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
	if err := ValidateTriggerAgentName(next, "trigger"); err != nil {
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
		persistCtx5, cancelPersist5 := persistenceContext(ctx)
		defer cancelPersist5()
		if rollbackErr := recreateDeletedResourceRecord(
			persistCtx5,
			m.triggerResources,
			m.resourceActor,
			current,
		); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if err := m.deleteOwnedWebhookSecretIfPresent(ctx, current.Spec); err != nil {
		persistCtx6, cancelPersist6 := persistenceContext(ctx)
		defer cancelPersist6()
		rollbackErr := errors.Join(
			recreateDeletedResourceRecord(
				persistCtx6,
				m.triggerResources,
				m.resourceActor,
				current,
			),
			restoreWebhookSecretStates(persistCtx6, m, secretState),
		)
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if err := m.applyTriggerResourcesFromStore(ctx); err != nil {
		persistCtx7, cancelPersist7 := persistenceContext(ctx)
		defer cancelPersist7()
		rollbackErr := errors.Join(
			recreateDeletedResourceRecord(
				persistCtx7,
				m.triggerResources,
				m.resourceActor,
				current,
			),
			restoreWebhookSecretStates(persistCtx7, m, secretState),
		)
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}
