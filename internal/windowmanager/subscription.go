package windowmanager

import (
	"context"
	"sync"
)

type eventSubscription struct {
	fence    SubscriptionFence
	updates  chan SubscriptionUpdate
	clientID *ClientID

	mu          sync.RWMutex
	err         error
	remove      func()
	stopContext func() bool
	terminated  bool
	closeOnce   sync.Once
}

var _ Subscription = (*eventSubscription)(nil)

func newEventSubscription(fence SubscriptionFence, capacity int) *eventSubscription {
	var clientID *ClientID
	if fence.Client != nil {
		clientID = clonePointer(&fence.Client.ClientID)
	}
	return &eventSubscription{
		fence:    cloneSubscriptionFence(fence),
		updates:  make(chan SubscriptionUpdate, capacity),
		clientID: clientID,
	}
}

func (s *eventSubscription) Fence() SubscriptionFence           { return cloneSubscriptionFence(s.fence) }
func (s *eventSubscription) Updates() <-chan SubscriptionUpdate { return s.updates }
func (s *eventSubscription) Err() error                         { s.mu.RLock(); defer s.mu.RUnlock(); return s.err }
func (s *eventSubscription) Close() error                       { s.close(); return nil }

func (s *eventSubscription) close() {
	s.mu.RLock()
	remove := s.remove
	s.mu.RUnlock()
	if remove != nil {
		remove()
		return
	}
	s.terminate(nil)
}

func (s *eventSubscription) setRemove(remove func()) { s.mu.Lock(); s.remove = remove; s.mu.Unlock() }

func (s *eventSubscription) watchContext(ctx context.Context) {
	stop := context.AfterFunc(ctx, s.close)
	s.mu.Lock()
	if s.terminated {
		s.mu.Unlock()
		stop()
		return
	}
	s.stopContext = stop
	s.mu.Unlock()
}

func (s *eventSubscription) terminate(err error) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.err = err
		s.terminated = true
		stopContext := s.stopContext
		s.mu.Unlock()
		if stopContext != nil {
			stopContext()
		}
		close(s.updates)
	})
}

type subscriptionHub struct {
	capacity      int
	mu            sync.Mutex
	subscriptions map[*eventSubscription]struct{}
}

func newSubscriptionHub(capacity int) *subscriptionHub {
	return &subscriptionHub{
		capacity:      capacity,
		subscriptions: make(map[*eventSubscription]struct{}),
	}
}

func (h *subscriptionHub) add(ctx context.Context, fence SubscriptionFence) *eventSubscription {
	subscription := newEventSubscription(fence, h.capacity)
	subscription.setRemove(func() { h.remove(subscription) })
	h.mu.Lock()
	h.subscriptions[subscription] = struct{}{}
	h.mu.Unlock()
	subscription.watchContext(ctx)
	return subscription
}

func (h *subscriptionHub) remove(subscription *eventSubscription) {
	h.mu.Lock()
	_, exists := h.subscriptions[subscription]
	if exists {
		delete(h.subscriptions, subscription)
	}
	h.mu.Unlock()
	if exists {
		subscription.terminate(nil)
	}
}

func (h *subscriptionHub) publishEvent(event Event) {
	update := SubscriptionUpdate{Event: &event}
	h.publish(update, func(*eventSubscription) bool { return true })
}

func (h *subscriptionHub) publishClient(client ClientView) {
	update := SubscriptionUpdate{Client: &client}
	h.publish(update, func(subscription *eventSubscription) bool {
		return subscription.clientID != nil && *subscription.clientID == client.ClientID
	})
}

func (h *subscriptionHub) publish(
	update SubscriptionUpdate,
	matches func(*eventSubscription) bool,
) {
	h.mu.Lock()
	evicted := make([]*eventSubscription, 0)
	for subscription := range h.subscriptions {
		if !matches(subscription) {
			continue
		}
		select {
		case subscription.updates <- cloneSubscriptionUpdate(update):
		default:
			delete(h.subscriptions, subscription)
			evicted = append(evicted, subscription)
		}
	}
	h.mu.Unlock()
	for _, subscription := range evicted {
		subscription.terminate(ErrSlowConsumer)
	}
}

func (h *subscriptionHub) closeClient(clientID ClientID, err error) {
	h.mu.Lock()
	subscriptions := make([]*eventSubscription, 0)
	for subscription := range h.subscriptions {
		if subscription.clientID == nil || *subscription.clientID != clientID {
			continue
		}
		delete(h.subscriptions, subscription)
		subscriptions = append(subscriptions, subscription)
	}
	h.mu.Unlock()
	for _, subscription := range subscriptions {
		subscription.terminate(err)
	}
}

func (h *subscriptionHub) closeAll(err error) {
	h.mu.Lock()
	subscriptions := make([]*eventSubscription, 0, len(h.subscriptions))
	for subscription := range h.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}
	clear(h.subscriptions)
	h.mu.Unlock()
	for _, subscription := range subscriptions {
		subscription.terminate(err)
	}
}
