package acp

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

var (
	ErrIngestSaturated = errors.New("acp: prompt ingestion capacity exhausted")
	ErrIngestClosed    = errors.New("acp: prompt ingestion is closed")
)

const (
	defaultIngestCapacity = 1024
	defaultIngestMaxBytes = 8 << 20
	maxIngestTextBytes    = 4096
)

// IngestStats describes accepted pre-append work, including coalesced chunks.
type IngestStats struct {
	Depth     int    `json:"depth"`
	Bytes     int    `json:"bytes"`
	Coalesced uint64 `json:"coalesced"`
}

// IngestGate accepts bounded, lossless work without waiting for its consumer.
// Close retains accepted events for Next, then returns the closing error.
type IngestGate struct {
	mu        sync.Mutex
	queue     []*ingestEntry
	head      int
	depth     int
	bytes     int
	maxBytes  int
	coalesced uint64
	closed    bool
	reason    error
	changed   chan struct{}
	done      chan struct{}
}

type ingestEntry struct {
	event  AgentEvent
	text   strings.Builder
	rawKey string
	bytes  int
}

func NewIngestGate(capacity, maxBytes int) *IngestGate {
	if capacity <= 0 {
		capacity = defaultIngestCapacity
	}
	if maxBytes <= 0 {
		maxBytes = defaultIngestMaxBytes
	}
	return &IngestGate{
		queue:    make([]*ingestEntry, capacity),
		maxBytes: maxBytes,
		changed:  make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

func (g *IngestGate) Admit(event AgentEvent) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return ErrIngestClosed
	}
	key, mergeable := ingestTextKey(event)
	if g.depth > 0 && mergeable {
		tail := g.queue[(g.head+g.depth-1)%len(g.queue)]
		if tail.text.Len()+len(event.Text) <= maxIngestTextBytes &&
			tail.rawKey == key && equivalentIngestText(tail.event, event) {
			if len(event.Text) > g.maxBytes-g.bytes {
				return ErrIngestSaturated
			}
			tail.text.WriteString(event.Text)
			tail.bytes += len(event.Text)
			g.bytes += len(event.Text)
			g.coalesced++
			return nil
		}
	}
	cost := ingestEventBytes(event)
	if g.depth == len(g.queue) || cost > g.maxBytes-g.bytes {
		return ErrIngestSaturated
	}
	entry := &ingestEntry{event: event, rawKey: key, bytes: cost}
	entry.text.WriteString(event.Text)
	g.queue[(g.head+g.depth)%len(g.queue)] = entry
	g.depth++
	g.bytes += cost
	g.signal()
	return nil
}

// Next drains accepted work in order. Its wait is exclusively consumer-owned.
func (g *IngestGate) Next(ctx context.Context) (AgentEvent, error) {
	for {
		g.mu.Lock()
		if g.depth > 0 {
			entry := g.queue[g.head]
			g.queue[g.head] = nil
			g.head = (g.head + 1) % len(g.queue)
			g.depth--
			g.bytes -= entry.bytes
			g.mu.Unlock()
			return materializeIngestText(entry)
		}
		closed, reason := g.closed, g.reason
		g.mu.Unlock()
		if closed {
			if reason == nil {
				reason = io.EOF
			}
			return AgentEvent{}, reason
		}
		select {
		case <-ctx.Done():
			return AgentEvent{}, ctx.Err()
		case <-g.changed:
		case <-g.done:
		}
	}
}

func (g *IngestGate) Close(reason error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed, g.reason = true, reason
	close(g.done)
}

func (g *IngestGate) Stats() IngestStats {
	g.mu.Lock()
	defer g.mu.Unlock()
	return IngestStats{Depth: g.depth, Bytes: g.bytes, Coalesced: g.coalesced}
}

func (g *IngestGate) signal() {
	select {
	case g.changed <- struct{}{}:
	default:
	}
}
