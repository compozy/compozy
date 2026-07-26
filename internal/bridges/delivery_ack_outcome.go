package bridges

import (
	"strings"

	redactpkg "github.com/compozy/agh/internal/redact"
)

const progressDeliveryIndeterminateReason = "progress_delivery_indeterminate"

func (b *Broker) handleCommittedResultUnavailable(
	route *routeWorker,
	deliveryID string,
	eventSeq int64,
	ack DeliveryAck,
) {
	b.handleIndeterminateTextDelivery(route, deliveryID, eventSeq, ack.Error.Message)
}

func (b *Broker) handleIndeterminateTextDelivery(
	route *routeWorker,
	deliveryID string,
	eventSeq int64,
	reason string,
) {
	message := redactpkg.String(strings.TrimSpace(reason))

	b.mu.Lock()
	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		b.mu.Unlock()
		return
	}
	b.recordDeliveryFailureLocked(delivery.bridgeInstanceID, message)
	updatedAt := b.now()
	delivery.updatedAt = updatedAt
	metrics := b.deliveryMetricRecordLocked(delivery, updatedAt)
	checkpoint := DeliveryLedgerCheckpoint{
		DeliveryID:      delivery.deliveryID,
		State:           DeliveryLedgerStateTerminalError,
		LastSentSeq:     max(delivery.lastSentSeq, eventSeq),
		LastAckedSeq:    delivery.lastAckedSeq,
		RemoteMessageID: delivery.remoteMessageID,
		TerminalError:   message,
		UpdatedAt:       updatedAt,
		Metrics:         &metrics,
	}
	persist := b.ledgerStore != nil
	b.mu.Unlock()

	var persistErr error
	if persist {
		persistErr = b.persistDeliveryCheckpoint(b.lifecycleCtx, checkpoint)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	delivery = b.deliveries[deliveryID]
	if delivery == nil {
		return
	}
	if persistErr != nil {
		b.recordDeliveryIssueLocked(delivery.bridgeInstanceID, persistErr.Error())
	}
	b.removeDeliveryLocked(route, delivery)
}

func (b *Broker) handleProgressCommittedResultUnavailable(deliveryID string, ack DeliveryAck) {
	b.handleProgressIndeterminate(deliveryID, ack.Error.Message)
}

func (b *Broker) handleProgressIndeterminate(deliveryID string, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		return
	}
	b.recordDeliveryDropLocked(delivery.bridgeInstanceID, progressDeliveryIndeterminateReason)
	b.recordDeliveryIssueLocked(delivery.bridgeInstanceID, reason)
	delivery.updatedAt = b.now()
}
