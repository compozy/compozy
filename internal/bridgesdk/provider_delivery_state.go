package bridgesdk

import (
	"strings"
	"sync"
	"time"
)

type deliveryStateEntry[S any] struct {
	state     S
	touchedAt time.Time
	sequence  uint64
}

// DeliveryStateStoreOption configures bounded provider delivery-state retention.
type DeliveryStateStoreOption[S any] func(*DeliveryStateStore[S])

// DeliveryStateStore owns provider-specific delivery snapshots without deciding their lifecycle.
type DeliveryStateStore[S any] struct {
	mu         sync.Mutex
	states     map[string]deliveryStateEntry[S]
	maxEntries int
	ttl        time.Duration
	now        func() time.Time
	sequence   uint64
}

// NewDeliveryStateStore creates an empty typed delivery-state store.
func NewDeliveryStateStore[S any](options ...DeliveryStateStoreOption[S]) *DeliveryStateStore[S] {
	store := &DeliveryStateStore[S]{
		states: make(map[string]deliveryStateEntry[S]),
		now:    func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// WithDeliveryStateRetention bounds retained state by least-recent use and age.
func WithDeliveryStateRetention[S any](
	maxEntries int,
	ttl time.Duration,
	now func() time.Time,
) DeliveryStateStoreOption[S] {
	return func(store *DeliveryStateStore[S]) {
		if maxEntries > 0 {
			store.maxEntries = maxEntries
		}
		if ttl > 0 {
			store.ttl = ttl
		}
		if now != nil {
			store.now = now
		}
	}
}

// Load returns one delivery state.
func (s *DeliveryStateStore[S]) Load(deliveryID string) (S, bool) {
	var zero S
	if s == nil {
		return zero, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveryID = strings.TrimSpace(deliveryID)
	now := s.now()
	s.evictExpiredLocked(now)
	entry, ok := s.states[deliveryID]
	if !ok {
		return zero, false
	}
	s.touchLocked(deliveryID, &entry, now)
	return entry.state, true
}

// Store replaces one delivery state.
func (s *DeliveryStateStore[S]) Store(deliveryID string, state S) {
	if s == nil || strings.TrimSpace(deliveryID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveryID = strings.TrimSpace(deliveryID)
	now := s.now()
	s.evictExpiredLocked(now)
	entry := deliveryStateEntry[S]{state: state}
	s.touchLocked(deliveryID, &entry, now)
	s.evictOverCapacityLocked()
}

// Update mutates one delivery state atomically and reports whether it existed.
func (s *DeliveryStateStore[S]) Update(deliveryID string, update func(S) S) bool {
	if s == nil || update == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveryID = strings.TrimSpace(deliveryID)
	now := s.now()
	s.evictExpiredLocked(now)
	entry, ok := s.states[deliveryID]
	if !ok {
		return false
	}
	entry.state = update(entry.state)
	s.touchLocked(deliveryID, &entry, now)
	return true
}

// UpdateOrCreate atomically resolves one delivery state and reports whether it already existed.
func (s *DeliveryStateStore[S]) UpdateOrCreate(
	deliveryID string,
	resolve func(S, bool) S,
) (S, bool) {
	var zero S
	if s == nil || resolve == nil {
		return zero, false
	}
	deliveryID = strings.TrimSpace(deliveryID)
	if deliveryID == "" {
		return zero, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.evictExpiredLocked(now)
	entry, existed := s.states[deliveryID]
	entry.state = resolve(entry.state, existed)
	s.touchLocked(deliveryID, &entry, now)
	s.evictOverCapacityLocked()
	return entry.state, existed
}

// Delete removes and returns one delivery state.
func (s *DeliveryStateStore[S]) Delete(deliveryID string) (S, bool) {
	var zero S
	if s == nil {
		return zero, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveryID = strings.TrimSpace(deliveryID)
	s.evictExpiredLocked(s.now())
	entry, ok := s.states[deliveryID]
	if !ok {
		return zero, false
	}
	delete(s.states, deliveryID)
	return entry.state, true
}

// Drain removes and returns every delivery state.
func (s *DeliveryStateStore[S]) Drain() map[string]S {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked(s.now())
	drained := make(map[string]S, len(s.states))
	for key, entry := range s.states {
		drained[key] = entry.state
	}
	s.states = make(map[string]deliveryStateEntry[S])
	return drained
}

// TransformAll atomically replaces every retained state and returns the prior snapshot.
func (s *DeliveryStateStore[S]) TransformAll(transform func(string, S) S) map[string]S {
	if s == nil || transform == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.evictExpiredLocked(now)
	previous := make(map[string]S, len(s.states))
	for key, entry := range s.states {
		previous[key] = entry.state
		entry.state = transform(key, entry.state)
		s.touchLocked(key, &entry, now)
	}
	return previous
}

func (s *DeliveryStateStore[S]) touchLocked(
	deliveryID string,
	entry *deliveryStateEntry[S],
	now time.Time,
) {
	s.sequence++
	entry.touchedAt = now
	entry.sequence = s.sequence
	s.states[deliveryID] = *entry
}

func (s *DeliveryStateStore[S]) evictExpiredLocked(now time.Time) {
	if s.ttl <= 0 {
		return
	}
	for deliveryID, entry := range s.states {
		if !entry.touchedAt.Add(s.ttl).After(now) {
			delete(s.states, deliveryID)
		}
	}
}

func (s *DeliveryStateStore[S]) evictOverCapacityLocked() {
	for s.maxEntries > 0 && len(s.states) > s.maxEntries {
		oldestID := ""
		var oldestSequence uint64
		for deliveryID, entry := range s.states {
			if oldestID == "" || entry.sequence < oldestSequence {
				oldestID = deliveryID
				oldestSequence = entry.sequence
			}
		}
		delete(s.states, oldestID)
	}
}
