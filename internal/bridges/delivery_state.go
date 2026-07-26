package bridges

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func (b *Broker) snapshotLocked(delivery *activeDelivery) DeliverySnapshot {
	return DeliverySnapshot{
		DeliveryID:             delivery.deliveryID,
		SessionID:              delivery.sessionID,
		TurnID:                 delivery.turnID,
		BridgeInstanceID:       delivery.bridgeInstanceID,
		RoutingKey:             delivery.routingKey,
		DeliveryTarget:         delivery.target,
		LatestSeq:              delivery.latestSeq,
		LatestEventType:        delivery.latestEventType,
		CurrentContent:         delivery.currentContent,
		Operation:              delivery.operation,
		Reference:              cloneDeliveryReference(delivery.reference),
		ProviderMetadata:       cloneRawJSON(delivery.providerMetadata),
		LastSentSeq:            delivery.lastSentSeq,
		LastAckedSeq:           delivery.lastAckedSeq,
		RemoteMessageID:        delivery.remoteMessageID,
		ReplaceRemoteMessageID: delivery.replaceRemoteMessageID,
		Final:                  delivery.final,
		Error:                  delivery.errorText,
		UpdatedAt:              delivery.updatedAt,
	}
}

func (b *Broker) removeDeliveryLocked(route *routeWorker, delivery *activeDelivery) {
	if delivery == nil {
		return
	}
	delete(b.deliveries, delivery.deliveryID)
	delete(b.turnIndex, newTurnIndexKey(delivery.sessionID, delivery.turnID))
	if deliverySet := b.sessionIndex[delivery.sessionID]; deliverySet != nil {
		delete(deliverySet, delivery.deliveryID)
		if len(deliverySet) == 0 {
			delete(b.sessionIndex, delivery.sessionID)
		}
	}
	if route != nil {
		b.removeQueuedSlotLocked(route, delivery.deliveryID, deliveryQueueKindStart)
		b.removeQueuedSlotLocked(route, delivery.deliveryID, deliveryQueueKindDelta)
		b.removeQueuedSlotLocked(route, delivery.deliveryID, deliveryQueueKindTerminal)
		b.removeQueuedSlotLocked(route, delivery.deliveryID, deliveryQueueKindResume)
		b.removeQueuedProgressLocked(route, delivery.deliveryID)
	}
	b.retireIdleRouteLocked(route, delivery.routeHash)
}

func (b *Broker) sessionDeliveriesLocked(sessionID string) []string {
	set := b.sessionIndex[sessionID]
	if len(set) == 0 {
		return nil
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

func (d *activeDelivery) hasQueuedItems() bool {
	return d.queuedStart || d.queuedDelta || d.queuedTerminal || d.queuedResume || len(d.queuedProgress) > 0
}

func newTurnIndexKey(sessionID string, turnID string) turnIndexKey {
	return turnIndexKey{
		sessionID: sessionID,
		turnID:    turnID,
	}
}

func newDeliveryID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("del-%d", time.Now().UTC().UnixNano())
	}
	return "del-" + hex.EncodeToString(random[:])
}

func cloneDeliveryReference(reference *DeliveryMessageReference) *DeliveryMessageReference {
	if reference == nil {
		return nil
	}
	cloned := reference.normalize()
	return &cloned
}
