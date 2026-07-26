package automation

import (
	"context"

	"errors"
	"fmt"

	"sort"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"

	"github.com/compozy/agh/internal/vault"
)

func (e *TriggerEngine) matchingRegistrations(envelope ActivationEnvelope) ([]TriggerRegistration, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.stopped {
		return nil, ErrTriggerEngineStopped
	}

	matches := make([]TriggerRegistration, 0)
	for _, registration := range e.registrations {
		if registrationMatchesEnvelope(registration, envelope) {
			matches = append(matches, cloneTriggerRegistration(registration))
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Trigger.ID < matches[j].Trigger.ID
	})
	return matches, nil
}

func (e *TriggerEngine) dispatchPreMatched(
	ctx context.Context,
	envelope ActivationEnvelope,
	registrations []TriggerRegistration,
) (TriggerResult, error) {
	return e.dispatchMatches(ctx, envelope, registrations, false, nil)
}

func (e *TriggerEngine) dispatchAfterFilter(
	ctx context.Context,
	envelope ActivationEnvelope,
	registrations []TriggerRegistration,
	reservedRun *Run,
) (TriggerResult, error) {
	return e.dispatchMatches(ctx, envelope, registrations, true, reservedRun)
}

func (e *TriggerEngine) dispatchMatches(
	ctx context.Context,
	envelope ActivationEnvelope,
	registrations []TriggerRegistration,
	filterRegistrations bool,
	reservedRun *Run,
) (TriggerResult, error) {
	result := TriggerResult{
		Runs: make([]Run, 0, len(registrations)),
	}
	var errs []error
	dispatchKind := DispatchKindTrigger
	if envelope.Source == ActivationSourceExtension {
		dispatchKind = DispatchKindExtension
	}
	for _, registration := range registrations {
		if filterRegistrations && !registrationMatchesEnvelope(registration, envelope) {
			continue
		}

		result.Matched++
		trigger := registration.Trigger
		run, err := e.dispatcher.Dispatch(ctx, DispatchRequest{
			Kind:        dispatchKind,
			Trigger:     &trigger,
			Envelope:    pointerToActivationEnvelope(envelope),
			ReservedRun: cloneRun(reservedRun),
		})
		if run != nil {
			result.Runs = append(result.Runs, *cloneRun(run))
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if result.Runs == nil {
		result.Runs = []Run{}
	}
	return result, errors.Join(errs...)
}

func (e *TriggerEngine) webhookRegistration(
	scope Scope,
	workspaceID string,
	endpoint ParsedWebhookEndpoint,
) (TriggerRegistration, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.stopped {
		return TriggerRegistration{}, ErrTriggerEngineStopped
	}

	triggerID, ok := e.webhookIndex[strings.TrimSpace(endpoint.WebhookID)]
	if !ok {
		return TriggerRegistration{}, ErrWebhookTriggerNotRegistered
	}
	registration, ok := e.registrations[triggerID]
	if !ok {
		return TriggerRegistration{}, ErrWebhookTriggerNotRegistered
	}
	if registration.Trigger.Scope != scope ||
		strings.TrimSpace(registration.Trigger.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return TriggerRegistration{}, ErrWebhookTriggerNotRegistered
	}
	if strings.TrimSpace(registration.Trigger.EndpointSlug) != strings.TrimSpace(endpoint.EndpointSlug) ||
		strings.TrimSpace(registration.Trigger.WebhookID) != strings.TrimSpace(endpoint.WebhookID) {
		return TriggerRegistration{}, ErrWebhookTriggerNotRegistered
	}
	return cloneTriggerRegistration(registration), nil
}

func (e *TriggerEngine) resolveWebhookSecret(ctx context.Context, trigger Trigger) (string, func(), error) {
	ref := strings.TrimSpace(trigger.WebhookSecretRef)
	if ref == "" {
		return "", func() {}, ErrWebhookSecretRequired
	}
	if err := vault.ValidateRefNamespace(ref, "automation"); err != nil {
		return "", func() {}, fmt.Errorf("%w: %w", ErrWebhookSecretRequired, err)
	}
	if e.webhookSecrets == nil {
		return "", func() {}, ErrWebhookSecretRequired
	}
	value, err := e.webhookSecrets.ResolveRef(ctx, ref)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) {
			return "", func() {}, ErrWebhookSecretRequired
		}
		return "", func() {}, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", func() {}, ErrWebhookSecretRequired
	}
	return value, diagnostics.RegisterDynamicSecret(value), nil
}

func (e *TriggerEngine) validateWebhookRegistration(registration TriggerRegistration) error {
	if !strings.EqualFold(strings.TrimSpace(registration.Trigger.Event), triggerEventWebhook) {
		return nil
	}
	ref := strings.TrimSpace(registration.Trigger.WebhookSecretRef)
	if ref == "" {
		return ErrWebhookSecretRequired
	}
	if err := vault.ValidateRefNamespace(ref, "automation"); err != nil {
		return fmt.Errorf("%w: %w", ErrWebhookSecretRequired, err)
	}
	if e.webhookSecrets == nil {
		return ErrWebhookSecretRequired
	}
	return nil
}

func (e *TriggerEngine) ensureUniqueWebhookLocked(registration TriggerRegistration, allowTriggerID string) error {
	webhookID := strings.TrimSpace(registration.Trigger.WebhookID)
	if webhookID == "" {
		return nil
	}

	existingTriggerID, exists := e.webhookIndex[webhookID]
	if exists && existingTriggerID != strings.TrimSpace(allowTriggerID) {
		return ErrTriggerWebhookIDTaken
	}
	return nil
}

func (e *TriggerEngine) storeRegistrationLocked(registration TriggerRegistration) {
	e.registrations[registration.Trigger.ID] = cloneTriggerRegistration(registration)
	if webhookID := strings.TrimSpace(registration.Trigger.WebhookID); webhookID != "" {
		e.webhookIndex[webhookID] = registration.Trigger.ID
	}
}

func (e *TriggerEngine) deleteWebhookIndexLocked(triggerID string) {
	registration, ok := e.registrations[strings.TrimSpace(triggerID)]
	if !ok {
		return
	}
	if webhookID := strings.TrimSpace(registration.Trigger.WebhookID); webhookID != "" {
		delete(e.webhookIndex, webhookID)
	}
}
