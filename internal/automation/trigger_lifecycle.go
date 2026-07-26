package automation

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

// Start validates the runtime start contract. Trigger matching is synchronous so no background work begins here.
func (e *TriggerEngine) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: trigger engine start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.stopped {
		return ErrTriggerEngineStopped
	}
	return nil
}

// Shutdown marks the runtime as stopped and clears registered triggers.
func (e *TriggerEngine) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: trigger engine shutdown context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopped = true
	e.registrations = make(map[string]TriggerRegistration)
	e.webhookIndex = make(map[string]string)
	e.deliveries = make(map[string]time.Time)
	return nil
}

// Register adds a new trigger definition to the runtime.
func (e *TriggerEngine) Register(registration TriggerRegistration) error {
	normalized, err := normalizeTriggerRegistration(registration)
	if err != nil {
		return err
	}
	if err := e.validateWebhookRegistration(normalized); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return ErrTriggerEngineStopped
	}
	if _, exists := e.registrations[normalized.Trigger.ID]; exists {
		return fmt.Errorf("%w: %s", ErrTriggerAlreadyRegistered, normalized.Trigger.ID)
	}
	if err := e.ensureUniqueWebhookLocked(normalized, ""); err != nil {
		return err
	}

	e.storeRegistrationLocked(normalized)
	return nil
}

// Update replaces an existing trigger registration.
func (e *TriggerEngine) Update(registration TriggerRegistration) error {
	normalized, err := normalizeTriggerRegistration(registration)
	if err != nil {
		return err
	}
	if err := e.validateWebhookRegistration(normalized); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return ErrTriggerEngineStopped
	}
	if _, exists := e.registrations[normalized.Trigger.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrTriggerNotFound, normalized.Trigger.ID)
	}
	if err := e.ensureUniqueWebhookLocked(normalized, normalized.Trigger.ID); err != nil {
		return err
	}

	e.deleteWebhookIndexLocked(normalized.Trigger.ID)
	e.storeRegistrationLocked(normalized)
	return nil
}

// Unregister removes one trigger registration by id.
func (e *TriggerEngine) Unregister(id string) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return errors.New("automation: trigger id is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return ErrTriggerEngineStopped
	}
	if _, exists := e.registrations[trimmedID]; !exists {
		return fmt.Errorf("%w: %s", ErrTriggerNotFound, trimmedID)
	}

	e.deleteWebhookIndexLocked(trimmedID)
	delete(e.registrations, trimmedID)
	return nil
}

// Fire matches one normalized activation envelope against all registered triggers.
func (e *TriggerEngine) Fire(ctx context.Context, envelope ActivationEnvelope) (TriggerResult, error) {
	if ctx == nil {
		return TriggerResult{}, errors.New("automation: trigger fire context is required")
	}
	if err := envelope.Validate("envelope"); err != nil {
		return TriggerResult{}, err
	}

	registrations, err := e.matchingRegistrations(envelope)
	if err != nil {
		return TriggerResult{}, err
	}
	return e.dispatchPreMatched(ctx, envelope, registrations)
}

// FireSessionCreated normalizes a session-created lifecycle event and routes it through the shared matching path.
func (e *TriggerEngine) FireSessionCreated(ctx context.Context, sess *session.Session) (TriggerResult, error) {
	envelope, err := sessionEnvelope(sessionEventCreated, sess)
	if err != nil {
		return TriggerResult{}, err
	}
	return e.Fire(ctx, envelope)
}

// FireSessionStopped normalizes a session-stopped lifecycle event and routes it through the shared matching path.
func (e *TriggerEngine) FireSessionStopped(ctx context.Context, sess *session.Session) (TriggerResult, error) {
	envelope, err := sessionEnvelope(sessionEventStopped, sess)
	if err != nil {
		return TriggerResult{}, err
	}
	return e.Fire(ctx, envelope)
}

// FireMemoryConsolidated normalizes a dream-consolidation completion into the shared matching path.
func (e *TriggerEngine) FireMemoryConsolidated(
	ctx context.Context,
	event MemoryConsolidatedEvent,
) (TriggerResult, error) {
	envelope, err := memoryConsolidatedEnvelope(event)
	if err != nil {
		return TriggerResult{}, err
	}
	return e.Fire(ctx, envelope)
}

// FireHookCompletion normalizes one hook-completion telemetry record into the shared matching path.
func (e *TriggerEngine) FireHookCompletion(
	ctx context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) (TriggerResult, error) {
	if ctx == nil {
		return TriggerResult{}, errors.New("automation: trigger fire context is required")
	}
	envelope, err := e.hookCompletionEnvelope(ctx, sessionID, record)
	if err != nil {
		return TriggerResult{}, err
	}
	return e.Fire(ctx, envelope)
}

// HandleWebhook authenticates, normalizes, and dispatches one webhook delivery.
func (e *TriggerEngine) HandleWebhook(ctx context.Context, request WebhookRequest) (TriggerResult, error) {
	if ctx == nil {
		return TriggerResult{}, errors.New("automation: webhook context is required")
	}
	if err := request.Validate(triggerEventWebhook); err != nil {
		return TriggerResult{}, err
	}

	parsed, err := ParseWebhookEndpoint(request.Endpoint)
	if err != nil {
		return TriggerResult{}, err
	}
	registration, err := e.webhookRegistration(request.Scope, request.WorkspaceID, parsed)
	if err != nil {
		return TriggerResult{}, err
	}
	if err := ValidateWebhookTimestamp(request.Timestamp, e.now(), e.webhookFreshnessWindow); err != nil {
		return TriggerResult{}, err
	}
	webhookSecret, cleanup, err := e.resolveWebhookSecret(ctx, registration.Trigger)
	if err != nil {
		return TriggerResult{}, err
	}
	defer cleanup()
	if err := ValidateWebhookSignature(
		webhookSecret,
		request.Timestamp,
		request.Payload,
		request.Signature,
	); err != nil {
		return TriggerResult{}, err
	}
	claim, err := e.claimWebhookDelivery(ctx, registration.Trigger, request.DeliveryID)
	if err != nil {
		return TriggerResult{}, err
	}

	envelope, err := webhookEnvelope(request, registration.Trigger)
	if err != nil {
		e.releaseWebhookDelivery(ctx, claim)
		return TriggerResult{}, err
	}
	result, err := e.dispatchAfterFilter(ctx, envelope, []TriggerRegistration{registration}, claim.reservedRun)
	if len(result.Runs) == 0 {
		e.releaseWebhookDelivery(ctx, claim)
	}
	return result, err
}
