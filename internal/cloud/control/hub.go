package control

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const eventBuffer = 32

type StopReason string

const (
	GoingAway StopReason = "going_away"
	Resync    StopReason = "resync"
)

type Subscription struct {
	Events  <-chan store.ControlEvent
	Closing <-chan StopReason

	account  string
	computer remoteproto.ComputerID
	events   chan store.ControlEvent
	closing  chan StopReason
	stopping bool
}

type Hub struct {
	mu            sync.Mutex
	subscriptions map[*Subscription]struct{}
	connected     bool
	draining      bool
	changed       chan struct{}
	drained       chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscriptions: make(map[*Subscription]struct{}),
		changed:       make(chan struct{}),
		drained:       make(chan struct{}),
	}
}

func (h *Hub) Subscribe(accountID string, computerID remoteproto.ComputerID) (*Subscription, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if accountID == "" || h.draining || !h.connected {
		return nil, false
	}
	events := make(chan store.ControlEvent, eventBuffer)
	closing := make(chan StopReason, 1)
	sub := &Subscription{
		Events: events, Closing: closing, account: accountID, computer: computerID, events: events, closing: closing,
	}
	h.subscriptions[sub] = struct{}{}
	return sub, true
}

func (h *Hub) Release(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscriptions[sub]; !ok {
		return
	}
	delete(h.subscriptions, sub)
	close(sub.events)
	close(sub.closing)
	if h.draining && len(h.subscriptions) == 0 {
		close(h.drained)
	}
}

func (h *Hub) publish(event store.ControlEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscriptions {
		if sub.stopping || (event.AccountID != "" && event.AccountID != sub.account) ||
			(event.ComputerID != "" && event.ComputerID != sub.computer) {
			continue
		}
		select {
		case sub.events <- event:
		default:
			// A slow link must reconnect with a snapshot rather than miss an epoch.
			stopSubscription(sub, Resync)
		}
	}
}

func stopSubscription(sub *Subscription, reason StopReason) {
	if !sub.stopping {
		sub.stopping = true
		sub.closing <- reason
	}
}

func (h *Hub) setConnected(connected bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connected == connected {
		return
	}
	h.connected = connected
	close(h.changed)
	h.changed = make(chan struct{})
	if !connected {
		for sub := range h.subscriptions {
			stopSubscription(sub, Resync)
		}
	}
}

func (h *Hub) Ready() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected && !h.draining
}

func (h *Hub) WaitReady(ctx context.Context) error {
	for {
		h.mu.Lock()
		ready, draining, changed := h.connected, h.draining, h.changed
		h.mu.Unlock()
		if draining {
			return context.Canceled
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

// Drain stops subscriptions and signals each existing link before waiting for its release.
func (h *Hub) Drain() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.draining {
		return h.drained
	}
	h.draining = true
	close(h.changed)
	h.changed = make(chan struct{})
	for sub := range h.subscriptions {
		stopSubscription(sub, GoingAway)
	}
	if len(h.subscriptions) == 0 {
		close(h.drained)
	}
	return h.drained
}
