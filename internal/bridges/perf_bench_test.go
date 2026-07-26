package bridges

import (
	"testing"
	"time"
)

func BenchmarkBrokerProjectEventTurnLookup(b *testing.B) {
	b.ReportAllocs()

	broker := NewBroker(nil)
	b.Cleanup(broker.Close)

	lookupKey := newTurnIndexKey("sess-bench", "turn-bench")
	broker.turnIndex[lookupKey] = "delivery-bench"

	for b.Loop() {
		deliveryID, ok := broker.turnIndex[lookupKey]
		if !ok || deliveryID != "delivery-bench" {
			b.Fatalf("turnIndex lookup = %q, %t", deliveryID, ok)
		}
	}
}

func BenchmarkBrokerEnqueueEventLockedDelta(b *testing.B) {
	b.ReportAllocs()

	broker := &Broker{
		now:           func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
		queueCapacity: 8,
		deliveries:    make(map[string]*activeDelivery),
		metrics:       make(map[string]*instanceDeliveryMetrics),
	}
	route := &routeWorker{
		hash:             "route-bench",
		bridgeInstanceID: "brg-bench",
		queue:            make([]deliveryQueueItem, 0, broker.queueCapacity),
	}
	delivery := &activeDelivery{
		deliveryID:       "delivery-bench",
		bridgeInstanceID: "brg-bench",
		routingKey:       benchmarkRoutingKey("brg-bench", "peer-bench"),
		target: DeliveryTarget{
			BridgeInstanceID: "brg-bench",
			PeerID:           "peer-bench",
			Mode:             DeliveryModeReply,
		},
		seen: make(map[string]struct{}),
	}
	broker.deliveries[delivery.deliveryID] = delivery
	event := DeliveryEvent{
		DeliveryID:       delivery.deliveryID,
		BridgeInstanceID: delivery.bridgeInstanceID,
		RoutingKey:       delivery.routingKey,
		DeliveryTarget:   delivery.target,
		Seq:              1,
		EventType:        DeliveryEventTypeDelta,
		Content:          MessageContent{Text: "benchmark delta"},
		Final:            false,
	}

	for b.Loop() {
		route.queue = route.queue[:0]
		delivery.pendingDelta = nil
		delivery.queuedDelta = false

		if err := broker.enqueueEventLocked(route, delivery, event); err != nil {
			b.Fatalf("enqueueEventLocked() error: %v", err)
		}
		if len(route.queue) != 1 {
			b.Fatalf("queue length = %d, want 1", len(route.queue))
		}
	}
}

func BenchmarkBrokerPrepareRequestDelta(b *testing.B) {
	b.ReportAllocs()

	broker := &Broker{
		now:        func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
		deliveries: make(map[string]*activeDelivery),
	}
	route := &routeWorker{hash: "route-bench"}
	baseEvent := DeliveryEvent{
		DeliveryID:       "delivery-bench",
		BridgeInstanceID: "brg-bench",
		RoutingKey:       benchmarkRoutingKey("brg-bench", "peer-bench"),
		DeliveryTarget: DeliveryTarget{
			BridgeInstanceID: "brg-bench",
			PeerID:           "peer-bench",
			Mode:             DeliveryModeReply,
		},
		Seq:       42,
		EventType: DeliveryEventTypeDelta,
		Content:   MessageContent{Text: "benchmark delta"},
	}

	for b.Loop() {
		delivery := &activeDelivery{
			deliveryID:       baseEvent.DeliveryID,
			bridgeInstanceID: baseEvent.BridgeInstanceID,
			pendingDelta:     &baseEvent,
			queuedDelta:      true,
			updatedAt:        broker.now(),
		}
		broker.deliveries[delivery.deliveryID] = delivery

		req, eventType, seq, deliveryID, ok := broker.prepareRequest(route, deliveryQueueItem{
			deliveryID: delivery.deliveryID,
			kind:       deliveryQueueKindDelta,
		})
		if !ok {
			b.Fatal("prepareRequest() = not ok, want ok")
		}
		if deliveryID != delivery.deliveryID || eventType != DeliveryEventTypeDelta || seq != baseEvent.Seq {
			b.Fatalf("prepareRequest() returned %q %q %d", deliveryID, eventType, seq)
		}
		if req.Event.DeliveryID != delivery.deliveryID {
			b.Fatalf("request delivery id = %q, want %q", req.Event.DeliveryID, delivery.deliveryID)
		}
	}
}

func BenchmarkBrokerDeliveryMetricsSnapshot(b *testing.B) {
	b.ReportAllocs()

	broker := NewBroker(nil)
	b.Cleanup(broker.Close)

	for idx := range 64 {
		bridgeInstanceID := "brg-metrics-" + string(rune('a'+(idx%26)))
		broker.metrics[bridgeInstanceID] = &instanceDeliveryMetrics{
			droppedByReason: map[string]int{
				"coalesced":       idx + 1,
				"queue_saturated": idx + 2,
			},
			deliveryFailuresTotal: idx,
			lastError:             "timeout",
			lastErrorAt:           time.Unix(int64(idx), 0).UTC(),
			lastSuccessAt:         time.Unix(int64(idx+1), 0).UTC(),
		}
		broker.routes[string(rune('A'+(idx%26)))+time.Unix(int64(idx), 0).UTC().Format(time.RFC3339)] = &routeWorker{
			hash:             "route-bench",
			bridgeInstanceID: bridgeInstanceID,
			queue: []deliveryQueueItem{
				{deliveryID: "delivery-a", kind: deliveryQueueKindStart},
				{deliveryID: "delivery-b", kind: deliveryQueueKindDelta},
			},
			wakeCh: make(chan struct{}, 1),
		}
	}

	for b.Loop() {
		snapshot := broker.DeliveryMetrics()
		if len(snapshot) == 0 {
			b.Fatal("DeliveryMetrics() returned empty snapshot")
		}
	}
}

func benchmarkRoutingKey(bridgeInstanceID string, peerID string) RoutingKey {
	return RoutingKey{
		Scope:            ScopeGlobal,
		BridgeInstanceID: bridgeInstanceID,
		PeerID:           peerID,
	}
}
