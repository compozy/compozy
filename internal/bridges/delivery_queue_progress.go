package bridges

import (
	"errors"
)

var errProgressEventDropped = errors.New("bridges: intermediate progress event dropped")

type progressQueueKey struct {
	toolCallID string
	phase      ToolProgressPhase
}

func progressKeyFor(event DeliveryEvent) (progressQueueKey, error) {
	if event.Progress == nil {
		return progressQueueKey{}, errors.New("bridges: progress queue item requires progress payload")
	}
	progress := event.Progress.normalize()
	if progress.ToolCallID == "" {
		return progressQueueKey{}, errors.New("bridges: progress queue item requires tool call id")
	}
	if err := progress.Phase.Validate(); err != nil {
		return progressQueueKey{}, err
	}
	return progressQueueKey{toolCallID: progress.ToolCallID, phase: progress.Phase}, nil
}

func (b *Broker) enqueueProgressLocked(
	route *routeWorker,
	delivery *activeDelivery,
	event DeliveryEvent,
) error {
	key, err := progressKeyFor(event)
	if err != nil {
		return err
	}
	delivery.ensureProgressQueueState()
	if delivery.queuedProgress[key] {
		cloned := cloneDeliveryEvent(event)
		delivery.pendingProgress[key] = &cloned
		b.recordDeliveryDropLocked(route.bridgeInstanceID, "progress_coalesced")
		return nil
	}

	if !b.makeQueueRoomLocked(route) {
		if key.phase == ToolProgressPhaseStarted {
			b.recordDeliveryDropLocked(route.bridgeInstanceID, "progress_queue_saturated")
			return errProgressEventDropped
		}
		// Completion and failure are lifecycle-closing status. Like delivery
		// terminals, they may temporarily exceed the soft queue bound rather
		// than silently disappear.
	}

	cloned := cloneDeliveryEvent(event)
	delivery.pendingProgress[key] = &cloned
	delivery.queuedProgress[key] = true
	route.queue = append(route.queue, deliveryQueueItem{
		deliveryID: delivery.deliveryID,
		kind:       deliveryQueueKindProgress,
		progress:   key,
	})
	return nil
}

func (b *Broker) makeQueueRoomLocked(route *routeWorker) bool {
	if route == nil || len(route.queue) < b.queueCapacity {
		return true
	}
	if b.dropQueuedDeltaLocked(route) {
		return true
	}
	return b.dropQueuedIntermediateProgressLocked(route)
}

func (b *Broker) dropQueuedIntermediateProgressLocked(route *routeWorker) bool {
	if route == nil {
		return false
	}
	for index, item := range route.queue {
		if item.kind != deliveryQueueKindProgress || item.progress.phase != ToolProgressPhaseStarted {
			continue
		}
		delivery := b.deliveries[item.deliveryID]
		if delivery != nil {
			delete(delivery.pendingProgress, item.progress)
			delete(delivery.queuedProgress, item.progress)
		}
		route.queue = append(route.queue[:index], route.queue[index+1:]...)
		b.recordDeliveryDropLocked(route.bridgeInstanceID, "progress_evicted")
		return true
	}
	return false
}

func (d *activeDelivery) ensureProgressQueueState() {
	if d.pendingProgress == nil {
		d.pendingProgress = make(map[progressQueueKey]*DeliveryEvent)
	}
	if d.queuedProgress == nil {
		d.queuedProgress = make(map[progressQueueKey]bool)
	}
}

func (d *activeDelivery) pendingProgressEvent(key progressQueueKey) (DeliveryEvent, bool) {
	if d == nil || d.pendingProgress == nil {
		return DeliveryEvent{}, false
	}
	event := d.pendingProgress[key]
	if event == nil {
		return DeliveryEvent{}, false
	}
	delete(d.pendingProgress, key)
	delete(d.queuedProgress, key)
	return cloneDeliveryEvent(*event), true
}

func (b *Broker) removeQueuedProgressLocked(route *routeWorker, deliveryID string) {
	if route == nil {
		return
	}
	next := route.queue[:0]
	for _, item := range route.queue {
		if item.deliveryID == deliveryID && item.kind == deliveryQueueKindProgress {
			continue
		}
		next = append(next, item)
	}
	route.queue = next
	if delivery := b.deliveries[deliveryID]; delivery != nil {
		clear(delivery.pendingProgress)
		clear(delivery.queuedProgress)
	}
}
