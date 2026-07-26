package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/vault"
)

func (m *Manager) rollbackJobEnabled(ctx context.Context, definition Job, enabled bool) error {
	switch {
	case isOverlayManagedSource(definition.Source):
		return m.persistJobOverlay(ctx, definition, enabled)
	default:
		definition.Enabled = enabled
		_, err := m.store.UpdateJob(ctx, definition)
		return err
	}
}

func (m *Manager) rollbackTriggerEnabled(ctx context.Context, definition Trigger, enabled bool) error {
	switch {
	case isOverlayManagedSource(definition.Source):
		return m.persistTriggerOverlay(ctx, definition, enabled)
	default:
		definition.Enabled = enabled
		_, err := m.store.UpdateTrigger(ctx, definition)
		return err
	}
}

func isOverlayManagedSource(source JobSource) bool {
	switch source {
	case JobSourceConfig, JobSourcePackage:
		return true
	default:
		return false
	}
}

func applyWebhookSecretRef(current Trigger, previous *Trigger, write *WebhookSecretWrite) Trigger {
	next := cloneTrigger(current)
	if !strings.EqualFold(strings.TrimSpace(next.Event), "webhook") {
		next.WebhookSecretRef = ""
		return next
	}
	ref := strings.TrimSpace(next.WebhookSecretRef)
	if write != nil && strings.TrimSpace(write.Ref) != "" {
		ref = strings.TrimSpace(write.Ref)
	}
	if ref == "" && previous != nil {
		ref = strings.TrimSpace(previous.WebhookSecretRef)
	}
	if ref == "" && write != nil && write.Value != nil {
		ref = defaultAutomationWebhookSecretRef(next.ID)
	}
	next.WebhookSecretRef = ref
	return next
}

func requireWebhookSecretRef(trigger Trigger) error {
	if !strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") {
		return nil
	}
	if strings.TrimSpace(trigger.WebhookSecretRef) == "" {
		return ErrWebhookSecretRequired
	}
	return nil
}

func defaultAutomationWebhookSecretRef(triggerID string) string {
	return "vault:automation/triggers/" + strings.TrimSpace(triggerID) + "/webhook-secret"
}

func ownedAutomationWebhookSecretRef(trigger Trigger) string {
	if strings.TrimSpace(trigger.ID) == "" {
		return ""
	}
	return defaultAutomationWebhookSecretRef(trigger.ID)
}

func isOwnedAutomationWebhookSecretRef(trigger Trigger) bool {
	return strings.TrimSpace(trigger.WebhookSecretRef) == ownedAutomationWebhookSecretRef(trigger)
}

func (m *Manager) applyWebhookSecretWritePointer(
	ctx context.Context,
	trigger Trigger,
	write *WebhookSecretWrite,
) error {
	if write == nil {
		return nil
	}
	return m.applyWebhookSecretWrite(ctx, trigger, *write)
}

func (m *Manager) applyWebhookSecretWrite(ctx context.Context, trigger Trigger, write WebhookSecretWrite) error {
	if !strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") {
		if strings.TrimSpace(write.Ref) != "" || write.Value != nil {
			return errors.New("automation: webhook secret write is only valid for webhook triggers")
		}
		return m.deleteOwnedWebhookSecretIfPresent(ctx, trigger)
	}
	if write.Value == nil {
		return nil
	}
	ref := strings.TrimSpace(write.Ref)
	if ref == "" {
		ref = strings.TrimSpace(trigger.WebhookSecretRef)
	}
	if ref == "" {
		ref = defaultAutomationWebhookSecretRef(trigger.ID)
	}
	if err := vault.ValidateSecretRefNamespace(ref, "automation"); err != nil {
		return fmt.Errorf("automation: webhook secret value requires vault:automation/... ref: %w", err)
	}
	if strings.TrimSpace(*write.Value) == "" {
		return ErrWebhookSecretRequired
	}
	if m.webhookSecrets == nil {
		return errors.New("automation: webhook secret store is required")
	}
	if _, err := m.webhookSecrets.PutSecret(ctx, ref, "webhook_secret", *write.Value); err != nil {
		return fmt.Errorf("automation: store webhook secret %q: %w", ref, err)
	}
	return nil
}

func (m *Manager) deleteSupersededOwnedWebhookSecret(ctx context.Context, previous Trigger, current Trigger) error {
	if !isOwnedAutomationWebhookSecretRef(previous) {
		return nil
	}
	if strings.TrimSpace(previous.WebhookSecretRef) == strings.TrimSpace(current.WebhookSecretRef) {
		return nil
	}
	return m.deleteOwnedWebhookSecretIfPresent(ctx, previous)
}

func (m *Manager) deleteOwnedWebhookSecretIfPresent(ctx context.Context, trigger Trigger) error {
	if !isOwnedAutomationWebhookSecretRef(trigger) {
		return nil
	}
	if m.webhookSecrets == nil {
		return nil
	}
	if err := m.webhookSecrets.DeleteSecret(ctx, strings.TrimSpace(trigger.WebhookSecretRef)); err != nil &&
		!errors.Is(err, vault.ErrSecretNotFound) {
		return err
	}
	return nil
}

func (m *Manager) ensureTriggerWebhookID(ctx context.Context, trigger Trigger) (Trigger, error) {
	if !strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") || strings.TrimSpace(trigger.WebhookID) != "" {
		return trigger, nil
	}

	next := cloneTrigger(trigger)
	next.WebhookID = stableConfigID("wbh", next.ID)
	return m.store.UpdateTrigger(ctx, next)
}

func (m *Manager) currentWebhookSecret(ctx context.Context, trigger Trigger) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") || strings.TrimSpace(trigger.ID) == "" {
		return "", nil
	}
	if strings.TrimSpace(trigger.WebhookSecretRef) == "" || m.webhookSecrets == nil {
		return "", nil
	}
	secret, err := m.webhookSecrets.ResolveRef(ctx, trigger.WebhookSecretRef)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(secret), nil
}

func (m *Manager) restoreWebhookSecret(ctx context.Context, trigger Trigger, secret string) error {
	if !strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") {
		return m.deleteOwnedWebhookSecretIfPresent(ctx, trigger)
	}
	if strings.TrimSpace(secret) == "" {
		return m.deleteOwnedWebhookSecretIfPresent(ctx, trigger)
	}
	return m.applyWebhookSecretWrite(ctx, trigger, WebhookSecretWrite{
		Ref:   trigger.WebhookSecretRef,
		Value: &secret,
	})
}
