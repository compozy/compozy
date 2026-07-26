package automation

import (
	"context"
	"errors"

	"strings"

	modelpkg "github.com/compozy/agh/internal/automation/model"

	"github.com/compozy/agh/internal/store"
)

// Triggers returns overlay-aware trigger definitions from persistence.
func (m *Manager) Triggers(ctx context.Context) ([]Trigger, error) {
	return m.loadEffectiveTriggers(ctx, TriggerListQuery{})
}

// ListTriggers returns overlay-aware trigger definitions using the supplied
// filters.
func (m *Manager) ListTriggers(ctx context.Context, query TriggerListQuery) (TriggerListPage, error) {
	if ctx == nil {
		return TriggerListPage{}, errors.New("automation: list triggers context is required")
	}
	return m.loadEffectiveTriggerPage(ctx, modelpkg.TriggerListQueryWithDefaultLimit(query))
}

// GetTrigger returns one overlay-aware trigger definition by id.
func (m *Manager) GetTrigger(ctx context.Context, id string) (Trigger, error) {
	if ctx == nil {
		return Trigger{}, errors.New("automation: get trigger context is required")
	}
	return m.effectiveTrigger(ctx, strings.TrimSpace(id))
}

// CreateTrigger stores a new dynamic trigger definition plus its write-only
// webhook secret value when applicable, then registers it into the runtime engine.
func (m *Manager) CreateTrigger(
	ctx context.Context,
	trigger Trigger,
	webhookSecret WebhookSecretWrite,
) (Trigger, error) {
	if ctx == nil {
		return Trigger{}, errors.New("automation: create trigger context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.createTriggerResource(ctx, trigger, webhookSecret)
	}

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
	next = applyWebhookSecretRef(next, nil, &webhookSecret)
	if err := requireWebhookSecretRef(next); err != nil {
		return Trigger{}, err
	}
	if err := m.validateTriggerLoopTarget(ctx, next); err != nil {
		return Trigger{}, err
	}

	created, err := m.store.CreateTrigger(ctx, next)
	if err != nil {
		return Trigger{}, err
	}
	created, err = m.ensureTriggerWebhookID(ctx, created)
	if err != nil {
		return Trigger{}, errors.Join(err, m.cleanupCreatedTrigger(ctx, created.ID))
	}
	if err := m.applyWebhookSecretWrite(ctx, created, webhookSecret); err != nil {
		return Trigger{}, errors.Join(err, m.cleanupCreatedTrigger(ctx, created.ID))
	}

	current, err := m.effectiveTriggerFromStored(ctx, created)
	if err != nil {
		return Trigger{}, errors.Join(err, m.cleanupCreatedTrigger(ctx, created.ID))
	}
	if err := m.applyTriggerToRuntime(current); err != nil {
		return Trigger{}, errors.Join(err, m.cleanupCreatedTrigger(ctx, created.ID))
	}

	return current, nil
}

// UpdateTrigger replaces one existing dynamic trigger definition.
func (m *Manager) UpdateTrigger(
	ctx context.Context,
	trigger Trigger,
	webhookSecret *WebhookSecretWrite,
) (Trigger, error) {
	if ctx == nil {
		return Trigger{}, errors.New("automation: update trigger context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.updateTriggerResource(ctx, trigger, webhookSecret)
	}

	currentStored, err := m.store.GetTrigger(ctx, strings.TrimSpace(trigger.ID))
	if err != nil {
		return Trigger{}, err
	}
	if currentStored.Source != JobSourceDynamic {
		return Trigger{}, ErrDefinitionReadOnly
	}

	previousEffective, err := m.effectiveTriggerFromStored(ctx, currentStored)
	if err != nil {
		return Trigger{}, err
	}
	next := cloneTrigger(trigger)
	next.ID = currentStored.ID
	next.Source = currentStored.Source
	next.CreatedAt = currentStored.CreatedAt
	next = applyWebhookSecretRef(next, &currentStored, webhookSecret)
	if err := ValidateImmutableTriggerTarget(currentStored, next); err != nil {
		return Trigger{}, err
	}
	if err := requireWebhookSecretRef(next); err != nil {
		return Trigger{}, err
	}
	if err := m.validateTriggerLoopTarget(ctx, next); err != nil {
		return Trigger{}, err
	}

	updatedStored, err := m.store.UpdateTrigger(ctx, next)
	if err != nil {
		return Trigger{}, err
	}
	updatedStored, err = m.ensureTriggerWebhookID(ctx, updatedStored)
	if err != nil {
		if _, rollbackErr := m.store.UpdateTrigger(ctx, currentStored); rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		return Trigger{}, err
	}
	if err := m.applyWebhookSecretWritePointer(ctx, updatedStored, webhookSecret); err != nil {
		if _, rollbackErr := m.store.UpdateTrigger(ctx, currentStored); rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		return Trigger{}, err
	}
	if err := m.deleteSupersededOwnedWebhookSecret(ctx, currentStored, updatedStored); err != nil {
		return Trigger{}, err
	}

	currentEffective, err := m.effectiveTriggerFromStored(ctx, updatedStored)
	if err != nil {
		if _, rollbackErr := m.store.UpdateTrigger(ctx, currentStored); rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		return Trigger{}, err
	}
	if err := m.applyTriggerToRuntime(currentEffective); err != nil {
		if _, rollbackErr := m.store.UpdateTrigger(ctx, currentStored); rollbackErr != nil {
			return Trigger{}, errors.Join(err, rollbackErr)
		}
		if runtimeErr := m.applyTriggerToRuntime(previousEffective); runtimeErr != nil {
			return Trigger{}, errors.Join(err, runtimeErr)
		}
		return Trigger{}, err
	}

	return currentEffective, nil
}

// DeleteTrigger removes one dynamic trigger definition and clears any
// persisted webhook secret.
func (m *Manager) DeleteTrigger(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("automation: delete trigger context is required")
	}
	if m.resourceDefinitionsEnabled() {
		return m.deleteTriggerResource(ctx, id)
	}

	currentStored, err := m.store.GetTrigger(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if currentStored.Source != JobSourceDynamic {
		return ErrDefinitionReadOnly
	}

	previousEffective, err := m.effectiveTriggerFromStored(ctx, currentStored)
	if err != nil {
		return err
	}
	engine := m.triggerEngineSnapshot()
	if engine != nil {
		if err := engine.Unregister(currentStored.ID); err != nil && !errors.Is(err, ErrTriggerNotFound) {
			return err
		}
	}

	if err := m.store.DeleteTrigger(ctx, currentStored.ID); err != nil {
		if runtimeErr := m.applyTriggerToRuntime(previousEffective); runtimeErr != nil {
			return errors.Join(err, runtimeErr)
		}
		return err
	}
	if err := m.deleteOwnedWebhookSecretIfPresent(ctx, currentStored); err != nil {
		return err
	}

	return nil
}
