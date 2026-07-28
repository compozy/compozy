package automation

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"time"
)

type webhookDeliveryClaim struct {
	triggerID   string
	deliveryID  string
	reservedRun *Run
}

func (e *TriggerEngine) claimWebhookDelivery(
	ctx context.Context,
	trigger Trigger,
	deliveryID string,
) (webhookDeliveryClaim, error) {
	claim := webhookDeliveryClaim{
		triggerID:  strings.TrimSpace(trigger.ID),
		deliveryID: strings.TrimSpace(deliveryID),
	}
	if e.webhookDeliveries != nil {
		reserved, err := e.claimPersistentWebhookDelivery(ctx, trigger, deliveryID)
		if err != nil {
			return webhookDeliveryClaim{}, err
		}
		claim.reservedRun = reserved
		return claim, nil
	}
	return claim, e.claimInMemoryWebhookDelivery(trigger.ID, deliveryID)
}

func (e *TriggerEngine) claimInMemoryWebhookDelivery(triggerID string, deliveryID string) error {
	now := e.now()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return ErrTriggerEngineStopped
	}

	e.purgeDeliveriesLocked(now)

	key := webhookDeliveryKey(triggerID, deliveryID)
	if expiresAt, exists := e.deliveries[key]; exists && expiresAt.After(now) {
		return ErrWebhookReplayDetected
	}

	e.deliveries[key] = now.Add(e.webhookFreshnessWindow)
	return nil
}

func (e *TriggerEngine) claimPersistentWebhookDelivery(
	ctx context.Context,
	trigger Trigger,
	deliveryID string,
) (*Run, error) {
	if ctx == nil {
		return nil, errors.New("automation: webhook delivery claim context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	e.mu.RLock()
	stopped := e.stopped
	e.mu.RUnlock()
	if stopped {
		return nil, ErrTriggerEngineStopped
	}

	now := e.now()
	runID := webhookDeliveryRunID(trigger.ID, deliveryID)
	run := Run{
		ID:                   runID,
		TriggerID:            strings.TrimSpace(trigger.ID),
		FireID:               webhookDeliveryFireID(trigger.ID, deliveryID),
		Status:               RunScheduled,
		Attempt:              1,
		StartedAt:            timePointer(now),
		NetworkParticipation: (DispatchRequest{Trigger: &trigger}).networkParticipation(),
	}
	created, err := e.webhookDeliveries.CreateRun(ctx, run)
	if err == nil {
		return &created, nil
	}
	if !errors.Is(err, ErrRunAlreadyExists) {
		return nil, fmt.Errorf("automation: claim webhook delivery: %w", err)
	}

	existing, getErr := e.webhookDeliveries.GetRun(ctx, runID)
	if getErr != nil && !errors.Is(getErr, ErrRunNotFound) {
		return nil, fmt.Errorf("automation: inspect webhook delivery claim: %w", getErr)
	}
	if getErr == nil && webhookDeliveryClaimActive(existing, now, e.webhookFreshnessWindow) {
		return nil, ErrWebhookReplayDetected
	}
	if getErr == nil {
		if err := e.webhookDeliveries.DeleteRun(ctx, runID); err != nil && !errors.Is(err, ErrRunNotFound) {
			return nil, fmt.Errorf("automation: expire webhook delivery claim: %w", err)
		}
	}

	created, err = e.webhookDeliveries.CreateRun(ctx, run)
	if err != nil {
		if errors.Is(err, ErrRunAlreadyExists) {
			return nil, ErrWebhookReplayDetected
		}
		return nil, fmt.Errorf("automation: reclaim webhook delivery: %w", err)
	}
	return &created, nil
}

func (e *TriggerEngine) releaseWebhookDelivery(ctx context.Context, claim webhookDeliveryClaim) {
	if claim.reservedRun != nil && e.webhookDeliveries != nil {
		if err := e.webhookDeliveries.DeleteRun(
			ctx,
			claim.reservedRun.ID,
		); err != nil &&
			!errors.Is(err, ErrRunNotFound) {
			e.logger.Warn(
				"automation.trigger.webhook_delivery_release_failed",
				"run_id", strings.TrimSpace(claim.reservedRun.ID),
				"error", err,
			)
		}
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.deliveries, webhookDeliveryKey(claim.triggerID, claim.deliveryID))
}

func (e *TriggerEngine) purgeDeliveriesLocked(now time.Time) {
	for key, expiresAt := range e.deliveries {
		if !expiresAt.After(now) {
			delete(e.deliveries, key)
		}
	}
}

func webhookDeliveryKey(triggerID string, deliveryID string) string {
	return strings.TrimSpace(triggerID) + "\x00" + strings.TrimSpace(deliveryID)
}
