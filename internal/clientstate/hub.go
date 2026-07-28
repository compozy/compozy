package clientstate

import (
	"log/slog"
	"sync"
)

type domainSelection map[string]struct{}

type subscriptionHub struct {
	workspace WorkspaceID
	logger    *slog.Logger
	metrics   *runtimeMetrics

	mu            sync.Mutex
	subscriptions map[*stateSubscription]domainSelection
}

func newSubscriptionHub(
	workspace WorkspaceID,
	logger *slog.Logger,
	metrics *runtimeMetrics,
) *subscriptionHub {
	return &subscriptionHub{
		workspace:     workspace,
		logger:        logger,
		metrics:       metrics,
		subscriptions: make(map[*stateSubscription]domainSelection),
	}
}

func (h *subscriptionHub) add(
	asOfSeq uint64,
	snapshot []Entry,
	domains domainSelection,
) *stateSubscription {
	subscription := newStateSubscription(asOfSeq, snapshot)
	subscription.setRemove(func() {
		h.remove(subscription)
	})
	h.mu.Lock()
	h.subscriptions[subscription] = domains
	h.metrics.subscriptionOpened()
	h.mu.Unlock()
	h.logger.Debug(
		"clientstate.watch.subscribe",
		"workspace", h.workspace,
		"as_of_seq", asOfSeq,
		"snapshot_entries", len(snapshot),
	)
	return subscription
}

func (h *subscriptionHub) remove(subscription *stateSubscription) {
	h.mu.Lock()
	_, exists := h.subscriptions[subscription]
	if exists {
		delete(h.subscriptions, subscription)
		h.metrics.subscriptionClosed()
	}
	h.mu.Unlock()
	if exists {
		subscription.terminate(nil)
		h.logger.Debug("clientstate.watch.unsubscribe", "workspace", h.workspace)
	}
}

func (h *subscriptionHub) publish(events []Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, event := range events {
		for subscription, domains := range h.subscriptions {
			if len(domains) > 0 {
				if _, ok := domains[event.Entry.Domain]; !ok {
					continue
				}
			}
			select {
			case subscription.events <- cloneEvent(event):
			default:
				delete(h.subscriptions, subscription)
				h.metrics.subscriptionClosed()
				h.metrics.slowConsumerEvicted()
				subscription.terminate(ErrSlowConsumer)
				h.logger.Warn(
					"clientstate.watch.slow_consumer",
					"workspace", h.workspace,
					"queue_depth", SubscriptionBufferSize,
				)
			}
		}
	}
}

func (h *subscriptionHub) closeAll(err error) {
	h.mu.Lock()
	subscriptions := make([]*stateSubscription, 0, len(h.subscriptions))
	for subscription := range h.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}
	clear(h.subscriptions)
	h.metrics.subscriptionsClosed(len(subscriptions))
	h.mu.Unlock()
	for _, subscription := range subscriptions {
		subscription.terminate(err)
	}
}
