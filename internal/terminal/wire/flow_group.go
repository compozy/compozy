package wire

import (
	"context"
	"sync"
)

const (
	AckGrainBytes    = 16 << 10
	AckHighWatermark = 256 << 10
	AckLowWatermark  = 64 << 10
	AckPendingLimit  = 1 << 20
	DropQueueLimit   = 64 << 10
)

type Flow string

const (
	FlowAck  Flow = "ack"
	FlowDrop Flow = "drop"
)

type Group struct {
	mu       sync.Mutex
	members  map[*Queue]struct{}
	changed  chan struct{}
	disabled bool
}

func NewGroup() *Group {
	return &Group{members: make(map[*Queue]struct{}), changed: make(chan struct{})}
}

func (g *Group) Add(queue *Queue) {
	if g == nil || queue == nil {
		return
	}
	g.mu.Lock()
	g.members[queue] = struct{}{}
	queue.group = g
	g.signalLocked()
	g.mu.Unlock()
}

func (g *Group) Remove(queue *Queue) {
	if g == nil || queue == nil {
		return
	}
	g.mu.Lock()
	delete(g.members, queue)
	g.signalLocked()
	g.mu.Unlock()
}

func (g *Group) WaitProducer(ctx context.Context) error {
	for {
		g.mu.Lock()
		blocked := g.allAckSubscribersHighLocked()
		changed := g.changed
		g.mu.Unlock()
		if !blocked {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (g *Group) ResumeProducer() {
	if g == nil {
		return
	}
	g.mu.Lock()
	// Process exit is terminal for this group: once released, producer
	// backpressure stays disabled so the final output drain cannot deadlock.
	g.disabled = true
	g.signalLocked()
	g.mu.Unlock()
}

func (g *Group) allAckSubscribersHighLocked() bool {
	if g.disabled {
		return false
	}
	ackCount := 0
	for member := range g.members {
		flow, high, closed := member.flowState()
		if closed || flow != FlowAck {
			continue
		}
		ackCount++
		if !high {
			return false
		}
	}
	return ackCount > 0
}

func (g *Group) signal() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.signalLocked()
	g.mu.Unlock()
}

func (g *Group) signalLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}
