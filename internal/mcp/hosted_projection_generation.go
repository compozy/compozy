package mcp

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/internal/tools"
)

// ProjectionEpoch is the daemon-owned monotonic invalidation source for hosted tool projections.
type ProjectionEpoch struct {
	value   atomic.Uint64
	changes changeSignal
}

// NewProjectionEpoch creates an initialized projection epoch.
func NewProjectionEpoch() *ProjectionEpoch {
	epoch := &ProjectionEpoch{}
	epoch.value.Store(1)
	return epoch
}

// Snapshot returns the current authoritative daemon projection epoch.
func (e *ProjectionEpoch) Snapshot(_ context.Context, _ tools.Scope) (string, bool) {
	if e == nil {
		return "", false
	}
	return strconv.FormatUint(e.value.Load(), 10), true
}

// Current returns the current epoch without changing it.
func (e *ProjectionEpoch) Current() uint64 {
	if e == nil {
		return 0
	}
	return e.value.Load()
}

// Changes returns a channel closed by the next Advance. Capture it before reading projection
// state so an advance racing that read still wakes the waiter. A nil epoch never signals.
func (e *ProjectionEpoch) Changes() <-chan struct{} {
	if e == nil {
		return nil
	}
	return e.changes.wait()
}

// Advance invalidates projections built against an earlier daemon state and wakes waiters.
func (e *ProjectionEpoch) Advance() uint64 {
	if e == nil {
		return 0
	}
	var next uint64
	e.changes.notify(func() { next = e.value.Add(1) })
	return next
}

// changeSignal broadcasts "something changed" by closing and replacing one channel.
type changeSignal struct {
	mu sync.Mutex
	ch chan struct{}
}

// wait returns the channel closed by the next notify.
func (s *changeSignal) wait() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == nil {
		s.ch = make(chan struct{})
	}
	return s.ch
}

// notify applies mutate, when present, atomically with waking every current waiter.
func (s *changeSignal) notify(mutate func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mutate != nil {
		mutate()
	}
	if s.ch != nil {
		close(s.ch)
		s.ch = nil
	}
}
