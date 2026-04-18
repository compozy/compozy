package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultBufferSize = 256
	warnEvery         = time.Second
)

// Bus fans out typed events to bounded per-subscriber channels.
type Bus[T any] struct {
	mu      sync.RWMutex
	subs    map[SubID]*subscription[T]
	nextID  SubID
	bufSize int
	closed  atomic.Bool
	steady  atomic.Pointer[subscriberSnapshot[T]]
}

type subscriberSnapshot[T any] struct {
	subs []*subscription[T]
}

type subscription[T any] struct {
	mu           sync.RWMutex
	id           SubID
	ch           chan T
	closed       bool
	dropped      atomic.Uint64
	lastWarnedAt atomic.Int64
}

// New constructs a bus with a bounded channel per subscriber.
func New[T any](bufSize int) *Bus[T] {
	if bufSize <= 0 {
		bufSize = defaultBufferSize
	}
	return &Bus[T]{
		subs:    make(map[SubID]*subscription[T]),
		bufSize: bufSize,
	}
}

// Subscribe registers a subscriber and returns its ID, channel, and unsubscribe function.
func (b *Bus[T]) Subscribe() (SubID, <-chan T, func()) {
	if b.closed.Load() {
		ch := make(chan T)
		close(ch)
		return 0, ch, func() {}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed.Load() {
		ch := make(chan T)
		close(ch)
		return 0, ch, func() {}
	}

	b.nextID++
	id := b.nextID
	sub := &subscription[T]{
		id: id,
		ch: make(chan T, b.bufSize),
	}
	b.subs[id] = sub
	b.refreshSnapshotLocked()
	return id, sub.ch, func() {
		b.unsubscribe(id)
	}
}

// Publish fans out one event to the current subscriber snapshot without blocking.
func (b *Bus[T]) Publish(ctx context.Context, evt T) {
	if b.closed.Load() {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	snapshot := b.snapshot()
	for _, sub := range snapshot {
		if err := ctx.Err(); err != nil {
			return
		}
		if published := sub.publish(ctx, evt); published {
			continue
		}
		dropped := sub.dropped.Add(1)
		sub.warnDrop(ctx, dropped)
	}
}

// Close unsubscribes all subscribers and closes their channels.
func (b *Bus[T]) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		if !b.closed.Load() {
			return fmt.Errorf("close bus: %w", err)
		}
	}
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	b.mu.Lock()
	snapshot := make([]*subscription[T], 0, len(b.subs))
	for _, sub := range b.subs {
		snapshot = append(snapshot, sub)
	}
	b.subs = make(map[SubID]*subscription[T])
	b.refreshSnapshotLocked()
	b.mu.Unlock()

	var closeErr error
	if err := ctx.Err(); err != nil {
		closeErr = fmt.Errorf("close bus: %w", err)
	}
	for _, sub := range snapshot {
		sub.close()
	}

	return closeErr
}

// DroppedFor returns the number of dropped events recorded for a subscriber.
func (b *Bus[T]) DroppedFor(id SubID) uint64 {
	b.mu.RLock()
	sub := b.subs[id]
	b.mu.RUnlock()
	if sub == nil {
		return 0
	}
	return sub.dropped.Load()
}

// SubscriberCount returns the number of active subscribers.
func (b *Bus[T]) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *Bus[T]) snapshot() []*subscription[T] {
	snapshot := b.steady.Load()
	if snapshot == nil {
		return nil
	}
	return snapshot.subs
}

func (b *Bus[T]) unsubscribe(id SubID) {
	b.mu.Lock()
	sub := b.subs[id]
	if sub != nil {
		delete(b.subs, id)
		b.refreshSnapshotLocked()
	}
	b.mu.Unlock()

	if sub != nil {
		sub.close()
	}
}

func (s *subscription[T]) publish(ctx context.Context, evt T) bool {
	if err := ctx.Err(); err != nil {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return true
	}

	select {
	case s.ch <- evt:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (s *subscription[T]) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	close(s.ch)
}

func (s *subscription[T]) warnDrop(ctx context.Context, dropped uint64) {
	now := time.Now().UnixNano()
	last := s.lastWarnedAt.Load()
	if last != 0 && time.Duration(now-last) < warnEvery {
		return
	}
	if !s.lastWarnedAt.CompareAndSwap(last, now) {
		return
	}

	slog.WarnContext(
		ctx,
		"events subscriber dropped messages",
		"subscriber_id",
		uint64(s.id),
		"dropped_total",
		dropped,
	)
}

func (b *Bus[T]) refreshSnapshotLocked() {
	snapshot := &subscriberSnapshot[T]{
		subs: make([]*subscription[T], 0, len(b.subs)),
	}
	for _, sub := range b.subs {
		snapshot.subs = append(snapshot.subs, sub)
	}
	b.steady.Store(snapshot)
}
