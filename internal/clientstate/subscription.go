package clientstate

import "sync"

type stateSubscription struct {
	asOfSeq  uint64
	snapshot []Entry
	events   chan Event
	done     chan struct{}

	mu        sync.RWMutex
	err       error
	remove    func()
	closeOnce sync.Once
}

var _ Subscription = (*stateSubscription)(nil)

func newStateSubscription(asOfSeq uint64, snapshot []Entry) *stateSubscription {
	return &stateSubscription{
		asOfSeq:  asOfSeq,
		snapshot: cloneEntries(snapshot),
		events:   make(chan Event, SubscriptionBufferSize),
		done:     make(chan struct{}),
	}
}

func (s *stateSubscription) AsOfSeq() uint64 {
	return s.asOfSeq
}

func (s *stateSubscription) Snapshot() []Entry {
	return cloneEntries(s.snapshot)
}

func (s *stateSubscription) Events() <-chan Event {
	return s.events
}

func (s *stateSubscription) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *stateSubscription) Close() error {
	s.close()
	return nil
}

func (s *stateSubscription) close() {
	s.mu.RLock()
	remove := s.remove
	s.mu.RUnlock()
	if remove != nil {
		remove()
	} else {
		s.terminate(nil)
	}
}

func (s *stateSubscription) setRemove(remove func()) {
	s.mu.Lock()
	s.remove = remove
	s.mu.Unlock()
}

func (s *stateSubscription) terminate(err error) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.events)
		close(s.done)
	})
}
