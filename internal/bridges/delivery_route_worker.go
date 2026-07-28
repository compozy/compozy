package bridges

import (
	"context"
	"strings"
	"time"
)

func (b *Broker) ensureRouteLocked(hash string, bridgeInstanceID string, extensionName string) *routeWorker {
	if route := b.routes[hash]; route != nil {
		return route
	}

	route := &routeWorker{
		hash:             hash,
		bridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
		extensionName:    strings.TrimSpace(extensionName),
		wakeCh:           make(chan struct{}, 1),
		stopCh:           make(chan struct{}),
	}
	b.routes[hash] = route
	if b.bridgeRoutes == nil {
		b.bridgeRoutes = make(map[string]map[string]*routeWorker)
	}
	indexedRoutes := b.bridgeRoutes[route.bridgeInstanceID]
	if indexedRoutes == nil {
		indexedRoutes = make(map[string]*routeWorker)
		b.bridgeRoutes[route.bridgeInstanceID] = indexedRoutes
	}
	indexedRoutes[hash] = route

	b.wg.Add(1)
	go b.runRouteWorker(route)
	return route
}

func (b *Broker) runRouteWorker(route *routeWorker) {
	defer b.wg.Done()

	for {
		item, ok := b.popQueueItem(route)
		if !ok {
			select {
			case <-route.wakeCh:
				continue
			case <-route.stopCh:
				return
			case <-b.lifecycleCtx.Done():
				return
			}
		}

		retry := b.processQueueItem(route, item)
		if retry {
			timer := time.NewTimer(b.retryDelay)
			select {
			case <-timer.C:
			case <-route.stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-b.lifecycleCtx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}
}

func (b *Broker) processQueueItem(route *routeWorker, item deliveryQueueItem) bool {
	req, eventType, eventSeq, deliveryID, ok := b.prepareRequest(route, item)
	if !ok {
		return false
	}

	transport := b.currentTransport()
	if transport == nil {
		if eventType == DeliveryEventTypeProgress {
			b.handleProgressSendFailure(deliveryID, ErrDeliveryTransportUnavailable)
			return false
		}
		b.handleSendFailure(route, deliveryID, ErrDeliveryTransportUnavailable)
		return true
	}
	if err := b.persistDeliveryIntent(deliveryID, eventType, eventSeq); err != nil {
		b.handleSendFailure(route, deliveryID, err)
		return true
	}

	callCtx, cancel := context.WithTimeout(b.lifecycleCtx, b.requestTimeout)
	ack, err := transport.DeliverBridge(callCtx, route.extensionName, req)
	cancel()
	if err != nil {
		if eventType == DeliveryEventTypeProgress {
			b.handleProgressSendFailure(deliveryID, err)
			return false
		}
		b.handleSendFailure(route, deliveryID, err)
		return true
	}
	if err := ack.ValidateFor(req.Event); err != nil {
		if eventType == DeliveryEventTypeProgress {
			b.handleProgressIndeterminate(deliveryID, err.Error())
			return false
		}
		b.handleIndeterminateTextDelivery(route, deliveryID, eventSeq, err.Error())
		return false
	}
	if ack.Outcome.Normalize() == DeliveryAckOutcomeCommittedResultUnavailable {
		if eventType == DeliveryEventTypeProgress {
			b.handleProgressCommittedResultUnavailable(deliveryID, ack)
			return false
		}
		b.handleCommittedResultUnavailable(route, deliveryID, eventSeq, ack)
		return false
	}

	if err := b.handleSendSuccess(route, deliveryID, eventType, eventSeq, ack); err != nil {
		return false
	}
	return false
}

func (b *Broker) handleProgressSendFailure(deliveryID string, reason error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		return
	}
	b.recordDeliveryDropLocked(delivery.bridgeInstanceID, "progress_delivery_failed")
	if reason != nil {
		b.recordDeliveryIssueLocked(delivery.bridgeInstanceID, reason.Error())
	}
	delivery.updatedAt = b.now()
}

func (b *Broker) prepareRequest(
	_ *routeWorker,
	item deliveryQueueItem,
) (DeliveryRequest, DeliveryEventType, int64, string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delivery := b.deliveries[item.deliveryID]
	if delivery == nil {
		return DeliveryRequest{}, "", 0, "", false
	}
	if item.kind == deliveryQueueKindResume {
		return b.prepareResumeRequestLocked(delivery)
	}
	event, ok := delivery.takePendingEvent(item)
	if !ok {
		return DeliveryRequest{}, "", 0, "", false
	}
	if event.Seq > delivery.lastSentSeq {
		delivery.lastSentSeq = event.Seq
	}
	delivery.updatedAt = b.now()
	return DeliveryRequest{Event: event}, event.EventType, event.Seq, delivery.deliveryID, true
}

func (d *activeDelivery) takePendingEvent(item deliveryQueueItem) (DeliveryEvent, bool) {
	switch item.kind {
	case deliveryQueueKindStart:
		if d.pendingStart == nil {
			return DeliveryEvent{}, false
		}
		event := cloneDeliveryEvent(*d.pendingStart)
		d.pendingStart = nil
		d.queuedStart = false
		return event, true
	case deliveryQueueKindDelta:
		if d.pendingDelta == nil {
			return DeliveryEvent{}, false
		}
		event := cloneDeliveryEvent(*d.pendingDelta)
		d.pendingDelta = nil
		d.queuedDelta = false
		return event, true
	case deliveryQueueKindProgress:
		return d.pendingProgressEvent(item.progress)
	case deliveryQueueKindTerminal:
		if d.pendingTerminal == nil {
			return DeliveryEvent{}, false
		}
		event := cloneDeliveryEvent(*d.pendingTerminal)
		d.pendingTerminal = nil
		d.queuedTerminal = false
		return event, true
	default:
		return DeliveryEvent{}, false
	}
}

func (b *Broker) prepareResumeRequestLocked(
	delivery *activeDelivery,
) (DeliveryRequest, DeliveryEventType, int64, string, bool) {
	delivery.queuedResume = false
	snapshot := cloneDeliverySnapshot(b.snapshotLocked(delivery))
	event := DeliveryEvent{
		DeliveryID:       delivery.deliveryID,
		BridgeInstanceID: delivery.bridgeInstanceID,
		RoutingKey:       delivery.routingKey,
		DeliveryTarget:   delivery.target,
		Seq:              delivery.latestSeq,
		EventType:        DeliveryEventTypeResume,
		Content:          delivery.currentContent,
		Operation:        delivery.operation,
		Reference:        cloneDeliveryReference(delivery.reference),
		Resume: &DeliveryResumeState{
			LatestEventType: delivery.latestEventType,
		},
		Final:            delivery.final,
		ProviderMetadata: cloneRawJSON(delivery.providerMetadata),
	}
	if event.Seq > delivery.lastSentSeq {
		delivery.lastSentSeq = event.Seq
	}
	delivery.updatedAt = b.now()
	return DeliveryRequest{
		Event:    event,
		Snapshot: &snapshot,
	}, event.EventType, event.Seq, delivery.deliveryID, true
}

func (b *Broker) handleSendFailure(route *routeWorker, deliveryID string, reason error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		return
	}
	b.removeQueuedSlotLocked(route, deliveryID, deliveryQueueKindStart)
	b.removeQueuedSlotLocked(route, deliveryID, deliveryQueueKindDelta)
	b.removeQueuedSlotLocked(route, deliveryID, deliveryQueueKindTerminal)
	b.removeQueuedProgressLocked(route, deliveryID)
	delivery.pendingStart = nil
	delivery.pendingDelta = nil
	delivery.pendingTerminal = nil

	if delivery.latestEventType == DeliveryEventTypeDelete {
		deleteEvent := DeliveryEvent{
			DeliveryID:       delivery.deliveryID,
			BridgeInstanceID: delivery.bridgeInstanceID,
			RoutingKey:       delivery.routingKey,
			DeliveryTarget:   delivery.target,
			Seq:              delivery.latestSeq,
			EventType:        DeliveryEventTypeDelete,
			Final:            true,
			Operation:        DeliveryOperationDelete,
			Reference:        cloneDeliveryReference(delivery.reference),
			ProviderMetadata: cloneRawJSON(delivery.providerMetadata),
		}
		delivery.pendingTerminal = &deleteEvent
		route.queue = append([]deliveryQueueItem{{
			deliveryID: deliveryID,
			kind:       deliveryQueueKindTerminal,
		}}, route.queue...)
		delivery.queuedTerminal = true
		delivery.updatedAt = b.now()
		if reason != nil &&
			(delivery.latestEventType != DeliveryEventTypeError || strings.TrimSpace(delivery.errorText) == "") {
			b.recordDeliveryIssueLocked(delivery.bridgeInstanceID, reason.Error())
		}
		return
	}

	if !delivery.queuedResume {
		route.queue = append([]deliveryQueueItem{{
			deliveryID: deliveryID,
			kind:       deliveryQueueKindResume,
		}}, route.queue...)
		delivery.queuedResume = true
	}
	delivery.updatedAt = b.now()
	if reason != nil {
		// Preserve terminal delivery errors as the operator-visible failure
		// signal; resume retries may still fail, but they should not replace a
		// concrete delivery error with a generic transport issue.
		if delivery.latestEventType == DeliveryEventTypeError && strings.TrimSpace(delivery.errorText) != "" {
			return
		}
		b.recordDeliveryIssueLocked(delivery.bridgeInstanceID, reason.Error())
	}
}

func (b *Broker) popQueueItem(route *routeWorker) (deliveryQueueItem, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	current := b.routes[route.hash]
	if current == nil || len(current.queue) == 0 {
		return deliveryQueueItem{}, false
	}

	item := current.queue[0]
	current.queue = current.queue[1:]
	return item, true
}

func (b *Broker) currentTransport() DeliveryTransport {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.transport
}

func (b *Broker) retireIdleRouteLocked(route *routeWorker, routeHash string) {
	if route == nil {
		route = b.routes[routeHash]
	}
	if route == nil {
		return
	}
	if b.routes[route.hash] != route {
		return
	}
	if len(route.queue) > 0 || b.routeHasActiveDeliveriesLocked(route.hash) {
		return
	}
	delete(b.routes, route.hash)
	if indexedRoutes := b.bridgeRoutes[route.bridgeInstanceID]; indexedRoutes != nil {
		delete(indexedRoutes, route.hash)
		if len(indexedRoutes) == 0 {
			delete(b.bridgeRoutes, route.bridgeInstanceID)
		}
	}
	close(route.stopCh)
}

func (b *Broker) routeHasActiveDeliveriesLocked(routeHash string) bool {
	for _, delivery := range b.deliveries {
		if delivery != nil && delivery.routeHash == routeHash {
			return true
		}
	}
	return false
}

func (b *Broker) signalRoute(route *routeWorker) {
	if route == nil {
		return
	}
	select {
	case route.wakeCh <- struct{}{}:
	default:
	}
}
