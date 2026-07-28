package bridges

import "fmt"

func (b *Broker) enqueueEventLocked(route *routeWorker, delivery *activeDelivery, event DeliveryEvent) error {
	switch normalizeDeliveryEventType(event.EventType) {
	case DeliveryEventTypeStart:
		if delivery.queuedStart {
			start := cloneDeliveryEvent(*delivery.pendingStart)
			start.Content = event.Content
			start.Seq = event.Seq
			start.ProviderMetadata = cloneRawJSON(event.ProviderMetadata)
			delivery.pendingStart = &start
			return nil
		}
		if !b.makeQueueRoomLocked(route) {
			b.recordDeliveryDropLocked(route.bridgeInstanceID, "queue_saturated")
			return ErrDeliveryQueueSaturated
		}
		cloned := cloneDeliveryEvent(event)
		delivery.pendingStart = &cloned
		delivery.queuedStart = true
		route.queue = append(
			route.queue,
			deliveryQueueItem{deliveryID: delivery.deliveryID, kind: deliveryQueueKindStart},
		)
		return nil
	case DeliveryEventTypeDelta:
		if !delivery.startDelivered && delivery.queuedStart && delivery.pendingStart != nil {
			if !b.makeQueueRoomLocked(route) {
				start := cloneDeliveryEvent(event)
				start.EventType = DeliveryEventTypeStart
				start.Final = false
				delivery.pendingStart = &start
				return nil
			}
		}
		if delivery.queuedDelta {
			cloned := cloneDeliveryEvent(event)
			delivery.pendingDelta = &cloned
			return nil
		}
		if !b.makeQueueRoomLocked(route) {
			b.recordDeliveryDropLocked(route.bridgeInstanceID, "queue_saturated")
			return ErrDeliveryQueueSaturated
		}
		cloned := cloneDeliveryEvent(event)
		delivery.pendingDelta = &cloned
		delivery.queuedDelta = true
		route.queue = append(
			route.queue,
			deliveryQueueItem{deliveryID: delivery.deliveryID, kind: deliveryQueueKindDelta},
		)
		return nil
	case DeliveryEventTypeProgress:
		return b.enqueueProgressLocked(route, delivery, event)
	case DeliveryEventTypeFinal, DeliveryEventTypeError, DeliveryEventTypeDelete:
		b.removeQueuedSlotLocked(route, delivery.deliveryID, deliveryQueueKindDelta)
		delivery.pendingDelta = nil
		if delivery.queuedTerminal {
			cloned := cloneDeliveryEvent(event)
			delivery.pendingTerminal = &cloned
			return nil
		}
		b.makeQueueRoomLocked(route)
		cloned := cloneDeliveryEvent(event)
		delivery.pendingTerminal = &cloned
		delivery.queuedTerminal = true
		route.queue = append(
			route.queue,
			deliveryQueueItem{deliveryID: delivery.deliveryID, kind: deliveryQueueKindTerminal},
		)
		return nil
	default:
		return fmt.Errorf("bridges: unsupported projected delivery event type %q", event.EventType)
	}
}

func (b *Broker) dropQueuedDeltaLocked(route *routeWorker) bool {
	if route == nil {
		return false
	}
	for idx, item := range route.queue {
		if item.kind != deliveryQueueKindDelta {
			continue
		}
		delivery := b.deliveries[item.deliveryID]
		if delivery != nil {
			delivery.queuedDelta = false
			delivery.pendingDelta = nil
		}
		route.queue = append(route.queue[:idx], route.queue[idx+1:]...)
		b.recordDeliveryDropLocked(route.bridgeInstanceID, "coalesced")
		return true
	}
	return false
}

func (b *Broker) removeQueuedSlotLocked(route *routeWorker, deliveryID string, kind deliveryQueueKind) {
	if route == nil {
		return
	}
	next := route.queue[:0]
	for _, item := range route.queue {
		if item.deliveryID == deliveryID && item.kind == kind {
			continue
		}
		next = append(next, item)
	}
	route.queue = next

	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		return
	}
	switch kind {
	case deliveryQueueKindStart:
		delivery.queuedStart = false
	case deliveryQueueKindDelta:
		delivery.queuedDelta = false
	case deliveryQueueKindTerminal:
		delivery.queuedTerminal = false
	case deliveryQueueKindResume:
		delivery.queuedResume = false
	}
}
