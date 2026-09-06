package acp

import (
	"encoding/json"
	"sync"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

// BroadcastStats measures only post-append delivery, never ingest admission.
type BroadcastStats struct {
	Subscribers int    `json:"subscribers"`
	Depth       int    `json:"depth"`
	Shed        uint64 `json:"shed"`
}

// LiveBroadcast distributes persisted events. A shed subscriber receives one
// replay marker and closes; its owner queries durable history after its cursor.
type LiveBroadcast struct {
	mu       sync.Mutex
	capacity int
	subs     map[*liveSubscriber]struct{}
	shed     uint64
}

type liveSubscriber struct {
	after    uint64
	wakeOnly bool
	events   chan store.SessionEvent
}

func NewLiveBroadcast(capacity int) *LiveBroadcast {
	if capacity <= 0 {
		capacity = 64
	}
	return &LiveBroadcast{capacity: capacity, subs: make(map[*liveSubscriber]struct{})}
}

func (b *LiveBroadcast) Subscribe(after uint64, wakeOnly bool) (<-chan store.SessionEvent, func()) {
	// One reserved slot guarantees the overflow marker cannot block publishing.
	capacity := b.capacity + 1
	if wakeOnly {
		capacity = 1
	}
	sub := &liveSubscriber{after: after, wakeOnly: wakeOnly, events: make(chan store.SessionEvent, capacity)}
	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub.events, sync.OnceFunc(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, exists := b.subs[sub]; exists {
			delete(b.subs, sub)
			close(sub.events)
		}
	})
}

// Publish must be called after event's durable append. Sequence is its store
// cursor; the broadcast neither allocates cursors nor acknowledges ingestion.
func (b *LiveBroadcast) Publish(sequence uint64, event store.SessionEvent) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	shed := false
	for sub := range b.subs {
		if sub.wakeOnly {
			// A projection wake carries the latest durable watermark, not every token.
			// The consumer reconstructs named events and deltas from its committed cursor.
			select {
			case sub.events <- event:
			default:
				select {
				case <-sub.events:
				default:
				}
				sub.events <- event
			}
			continue
		}
		if !sub.wakeOnly && sequence <= sub.after {
			continue
		}
		if len(sub.events) >= b.capacity {
			marker := broadcastDegradeMarker(event, sub.after, sequence)
			sub.events <- marker
			close(sub.events)
			delete(b.subs, sub)
			b.shed++
			shed = true
			continue
		}
		sub.events <- event
		if !sub.wakeOnly {
			sub.after = sequence
		}
	}
	return shed
}

func (b *LiveBroadcast) Stats() BroadcastStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	stats := BroadcastStats{Subscribers: len(b.subs), Shed: b.shed}
	for sub := range b.subs {
		stats.Depth += len(sub.events)
	}
	return stats
}

func broadcastDegradeMarker(event store.SessionEvent, after, through uint64) store.SessionEvent {
	// All fields are JSON scalar values; this shape cannot fail encoding.
	content, _ := json.Marshal(struct { //nolint:errcheck // Fixed scalar fields cannot fail JSON encoding.
		Type          string `json:"type"`
		SessionID     string `json:"session_id"`
		AfterSequence uint64 `json:"after_sequence"`
		Through       uint64 `json:"through_sequence"`
		Refresh       bool   `json:"refresh"`
	}{events.StreamConsumerDegraded, event.SessionID, after, through, true})
	return store.SessionEvent{
		SessionID: event.SessionID, Type: events.StreamConsumerDegraded,
		Timestamp: event.Timestamp, Content: string(content),
	}
}
