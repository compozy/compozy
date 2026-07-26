package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	redactpkg "github.com/compozy/agh/internal/redact"
)

func (b *Broker) createDeliveryLedgerRecord(
	ctx context.Context,
	delivery *activeDelivery,
	createdAt time.Time,
) error {
	if b.ledgerStore == nil || delivery == nil {
		return nil
	}
	record := DeliveryLedgerRecord{
		DeliveryID:       delivery.deliveryID,
		SessionID:        delivery.sessionID,
		TurnID:           delivery.turnID,
		RoutingKey:       delivery.routingKey,
		BridgeInstanceID: delivery.bridgeInstanceID,
		Scope:            delivery.routingKey.Scope,
		WorkspaceID:      delivery.routingKey.WorkspaceID,
		State:            DeliveryLedgerStateActive,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	if err := b.ledgerStore.CreateBridgeDelivery(ctx, record); err != nil {
		return fmt.Errorf("bridges: create durable delivery %q: %w", delivery.deliveryID, err)
	}
	return nil
}

func (b *Broker) checkpointInvalidRegistration(
	ctx context.Context,
	delivery *activeDelivery,
	reason error,
) error {
	if b.ledgerStore == nil || delivery == nil {
		return nil
	}
	message := "invalid delivery registration"
	if reason != nil {
		message = redactpkg.String(reason.Error())
	}
	checkpoint := DeliveryLedgerCheckpoint{
		DeliveryID:    delivery.deliveryID,
		State:         DeliveryLedgerStateTerminalError,
		TerminalError: message,
		UpdatedAt:     b.now(),
	}
	if err := b.ledgerStore.CheckpointBridgeDelivery(ctx, checkpoint); err != nil {
		return fmt.Errorf("bridges: terminalize invalid durable delivery %q: %w", delivery.deliveryID, err)
	}
	return nil
}

func (b *Broker) handleSendSuccess(
	route *routeWorker,
	deliveryID string,
	eventType DeliveryEventType,
	eventSeq int64,
	ack DeliveryAck,
) error {
	normalizedAck := ack.normalize()

	b.mu.Lock()
	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		b.mu.Unlock()
		return nil
	}
	checkpoint, persist := b.ackCheckpointLocked(delivery, eventType, eventSeq, normalizedAck)
	b.mu.Unlock()

	if persist {
		if err := b.persistDeliveryCheckpoint(b.lifecycleCtx, checkpoint); err != nil {
			b.mu.Lock()
			if current := b.deliveries[deliveryID]; current != nil {
				b.recordDeliveryIssueLocked(current.bridgeInstanceID, err.Error())
			}
			b.mu.Unlock()
			return err
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	delivery = b.deliveries[deliveryID]
	if delivery == nil {
		return nil
	}
	b.applyAcknowledgementLocked(route, delivery, eventType, eventSeq, normalizedAck)
	return nil
}

func (b *Broker) persistDeliveryIntent(
	deliveryID string,
	eventType DeliveryEventType,
	eventSeq int64,
) error {
	if b.ledgerStore == nil || !shouldPersistDeliveryAck(eventType) {
		return nil
	}

	b.mu.Lock()
	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		b.mu.Unlock()
		return nil
	}
	checkpoint := DeliveryLedgerCheckpoint{
		DeliveryID:      delivery.deliveryID,
		State:           DeliveryLedgerStateActive,
		LastSentSeq:     max(delivery.lastSentSeq, eventSeq),
		LastAckedSeq:    delivery.lastAckedSeq,
		RemoteMessageID: delivery.remoteMessageID,
		UpdatedAt:       b.now(),
	}
	b.mu.Unlock()

	return b.persistDeliveryCheckpoint(b.lifecycleCtx, checkpoint)
}

func (b *Broker) ackCheckpointLocked(
	delivery *activeDelivery,
	eventType DeliveryEventType,
	eventSeq int64,
	ack DeliveryAck,
) (DeliveryLedgerCheckpoint, bool) {
	normalizedEventType := normalizeDeliveryEventType(eventType)
	remoteMessageID := delivery.remoteMessageID
	if normalizedEventType != DeliveryEventTypeProgress && ack.RemoteMessageID != "" {
		remoteMessageID = ack.RemoteMessageID
	}
	persist := b.ledgerStore != nil && shouldPersistDeliveryAck(normalizedEventType)
	if !persist {
		return DeliveryLedgerCheckpoint{}, false
	}

	lastSentSeq := max(delivery.lastSentSeq, eventSeq)
	lastAckedSeq := max(delivery.lastAckedSeq, eventSeq)
	state := DeliveryLedgerStateActive
	terminalError := ""
	switch normalizedEventType {
	case DeliveryEventTypeError:
		state = DeliveryLedgerStateTerminalError
		terminalError = redactpkg.String(strings.TrimSpace(delivery.errorText))
	case DeliveryEventTypeFinal, DeliveryEventTypeDelete:
		state = DeliveryLedgerStateTerminalOK
	}
	updatedAt := b.now()
	metrics := b.deliveryMetricRecordLocked(delivery, updatedAt)
	metrics.LastSuccessAt = updatedAt.UTC()
	return DeliveryLedgerCheckpoint{
		DeliveryID:      delivery.deliveryID,
		State:           state,
		LastSentSeq:     lastSentSeq,
		LastAckedSeq:    lastAckedSeq,
		RemoteMessageID: remoteMessageID,
		TerminalError:   terminalError,
		UpdatedAt:       updatedAt,
		Metrics:         &metrics,
	}, true
}

func shouldPersistDeliveryAck(eventType DeliveryEventType) bool {
	switch normalizeDeliveryEventType(eventType) {
	case DeliveryEventTypeStart,
		DeliveryEventTypeResume,
		DeliveryEventTypeFinal,
		DeliveryEventTypeError,
		DeliveryEventTypeDelete:
		return true
	default:
		return false
	}
}

func (b *Broker) persistDeliveryCheckpoint(
	ctx context.Context,
	checkpoint DeliveryLedgerCheckpoint,
) error {
	if b.ledgerStore == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("bridges: delivery checkpoint context is required")
	}
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("bridges: persist delivery checkpoint %q: %w", checkpoint.DeliveryID, err)
		}
		if err := b.lifecycleCtx.Err(); err != nil {
			return fmt.Errorf("bridges: persist delivery checkpoint %q: %w", checkpoint.DeliveryID, err)
		}

		checkpointCtx, cancel := context.WithTimeout(ctx, b.requestTimeout)
		stopLifecycleCancel := context.AfterFunc(b.lifecycleCtx, cancel)
		err := b.ledgerStore.CheckpointBridgeDelivery(checkpointCtx, checkpoint)
		stopLifecycleCancel()
		cancel()
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrDeliveryLedgerNotFound) || errors.Is(err, ErrDeliveryLedgerConflict) {
			return fmt.Errorf("bridges: persist delivery checkpoint %q: %w", checkpoint.DeliveryID, err)
		}

		timer := time.NewTimer(b.retryDelay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			stopAndDrainTimer(timer)
			return fmt.Errorf(
				"bridges: persist delivery checkpoint %q: %w",
				checkpoint.DeliveryID,
				errors.Join(err, ctx.Err()),
			)
		case <-b.lifecycleCtx.Done():
			stopAndDrainTimer(timer)
			return fmt.Errorf(
				"bridges: persist delivery checkpoint %q: %w",
				checkpoint.DeliveryID,
				errors.Join(err, b.lifecycleCtx.Err()),
			)
		}
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func (b *Broker) applyAcknowledgementLocked(
	route *routeWorker,
	delivery *activeDelivery,
	eventType DeliveryEventType,
	eventSeq int64,
	ack DeliveryAck,
) {
	if eventSeq > delivery.lastSentSeq {
		delivery.lastSentSeq = eventSeq
	}
	if eventSeq > delivery.lastAckedSeq {
		delivery.lastAckedSeq = eventSeq
	}
	normalizedEventType := normalizeDeliveryEventType(eventType)
	if normalizedEventType != DeliveryEventTypeProgress {
		if ack.RemoteMessageID != "" {
			delivery.remoteMessageID = ack.RemoteMessageID
		}
		if ack.ReplaceRemoteMessageID != "" {
			delivery.replaceRemoteMessageID = ack.ReplaceRemoteMessageID
		}
	}
	if normalizedEventType == DeliveryEventTypeStart || normalizedEventType == DeliveryEventTypeResume {
		if delivery.latestSeq > 0 && delivery.latestEventType != DeliveryEventTypeError {
			delivery.startDelivered = true
		}
	}
	delivery.updatedAt = b.now()
	b.recordDeliverySuccessLocked(delivery.bridgeInstanceID, delivery.updatedAt)

	if delivery.final && !delivery.hasQueuedItems() {
		b.removeDeliveryLocked(route, delivery)
	}
}

func (b *Broker) deliveryMetricRecordLocked(
	delivery *activeDelivery,
	deliveredAt time.Time,
) DeliveryMetricRecord {
	metrics := b.metricsLocked(delivery.bridgeInstanceID)
	droppedReasons := make(map[string]int)
	droppedTotal := 0
	if metrics != nil {
		for reason, count := range metrics.droppedByReason {
			droppedReasons[reason] = count
			droppedTotal += count
		}
	}
	record := DeliveryMetricRecord{
		BridgeDeliveryMetrics: BridgeDeliveryMetrics{
			BridgeInstanceID:        delivery.bridgeInstanceID,
			DeliveryDroppedTotal:    droppedTotal,
			DeliveryDroppedByReason: droppedReasons,
		},
		Scope:       delivery.routingKey.Scope,
		WorkspaceID: delivery.routingKey.WorkspaceID,
		UpdatedAt:   deliveredAt.UTC(),
	}
	if metrics != nil {
		record.DeliveryFailuresTotal = metrics.deliveryFailuresTotal
		record.LastError = metrics.lastError
		record.LastErrorAt = metrics.lastErrorAt
		record.LastSuccessAt = metrics.lastSuccessAt
	}
	return record
}
