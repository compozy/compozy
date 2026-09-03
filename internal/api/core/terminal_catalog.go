package core

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const terminalCatalogRetention = 512
const terminalCatalogMailbox = 512

type terminalCatalogEvent struct {
	Sequence uint64
	Event    terminalpkg.Event
}

type terminalCatalog struct {
	mu             sync.Mutex
	events         []terminalCatalogEvent
	inbox          chan terminalCatalogEvent
	next           atomic.Uint64
	droppedThrough atomic.Uint64
	resetFloor     uint64
	signal         atomic.Pointer[terminalCatalogSignal]
}

type terminalCatalogSignal struct {
	done   chan struct{}
	closed atomic.Bool
}

func newTerminalCatalog(provider TerminalProvider) *terminalCatalog {
	catalog := &terminalCatalog{
		inbox: make(chan terminalCatalogEvent, terminalCatalogMailbox),
	}
	catalog.signal.Store(newTerminalCatalogSignal())
	provider.Observe(catalog.observe)
	return catalog
}

func (c *terminalCatalog) observe(_ context.Context, event terminalpkg.Event) {
	if !catalogEventKind(event.Kind) {
		return
	}
	sequence := c.next.Add(1)
	record := terminalCatalogEvent{Sequence: sequence, Event: event}
	signal := c.signal.Load()
	select {
	case c.inbox <- record:
	default:
		storeAtomicMaximum(&c.droppedThrough, sequence)
	}
	signal.close()
	if current := c.signal.Load(); current != signal {
		current.close()
	}
}

func newTerminalCatalogSignal() *terminalCatalogSignal {
	return &terminalCatalogSignal{done: make(chan struct{})}
}

func (s *terminalCatalogSignal) close() {
	if s != nil && s.closed.CompareAndSwap(false, true) {
		close(s.done)
	}
}

func (c *terminalCatalog) appendLocked(record terminalCatalogEvent) {
	c.events = append(c.events, record)
	if len(c.events) > terminalCatalogRetention {
		copy(c.events, c.events[len(c.events)-terminalCatalogRetention:])
		c.events = c.events[:terminalCatalogRetention]
	}
}

func (c *terminalCatalog) read(
	workspaceID, profileID string,
	after uint64,
) ([]terminalCatalogEvent, bool, uint64, <-chan struct{}) {
	for {
		signal := c.signal.Load()
		c.mu.Lock()
		c.drainLocked()
		replay, reset, fence := c.replayLocked(workspaceID, profileID, after)
		c.mu.Unlock()
		if !signal.closed.Load() {
			return replay, reset, fence, signal.done
		}
		c.signal.CompareAndSwap(signal, newTerminalCatalogSignal())
	}
}

func (c *terminalCatalog) drainLocked() {
	records := make([]terminalCatalogEvent, 0, len(c.inbox))
	for {
		select {
		case record := <-c.inbox:
			records = append(records, record)
		default:
			sort.Slice(records, func(left, right int) bool { return records[left].Sequence < records[right].Sequence })
			droppedThrough := c.droppedThrough.Load()
			if droppedThrough > c.resetFloor {
				c.events = nil
				c.resetFloor = droppedThrough
			}
			for _, record := range records {
				if record.Sequence > c.resetFloor {
					c.appendLocked(record)
				}
			}
			return
		}
	}
}

func (c *terminalCatalog) replayLocked(
	workspaceID, profileID string,
	after uint64,
) ([]terminalCatalogEvent, bool, uint64) {
	reset := after == 0
	fence := c.next.Load()
	if after > 0 && fence-after > terminalCatalogRetention {
		reset = true
	}
	if after > 0 && after < c.resetFloor {
		reset = true
	}
	if len(c.events) > 0 && after > 0 && after < c.events[0].Sequence-1 {
		reset = true
	}
	replay := make([]terminalCatalogEvent, 0)
	if !reset {
		for _, event := range c.events {
			if event.Sequence > after && event.Event.WorkspaceID == workspaceID && event.Event.ProfileID == profileID {
				replay = append(replay, event)
			}
		}
	}
	return replay, reset, fence
}

func storeAtomicMaximum(target *atomic.Uint64, value uint64) {
	for current := target.Load(); value > current; current = target.Load() {
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

func catalogEventKind(kind terminalpkg.EventKind) bool {
	switch kind {
	case terminalpkg.EventKindOpened,
		terminalpkg.EventKindClosed,
		terminalpkg.EventKindTitleChanged,
		terminalpkg.EventKindLeaseChanged,
		terminalpkg.EventKindModeChanged,
		terminalpkg.EventKindRecordingStarted,
		terminalpkg.EventKindRecordingStopped:
		return true
	default:
		return false
	}
}
