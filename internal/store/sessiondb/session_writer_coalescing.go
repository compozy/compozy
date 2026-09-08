package sessiondb

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	sessionWriterCoalesceWindow   = 5 * time.Millisecond
	sessionWriterCoalesceRequests = 32
)

// writePendingChunks combines only not-yet-acknowledged consecutive chunk writes.
// Every caller waits for the same durable transaction; committed rows never change.
func (s *SessionDB) writePendingChunks(first sessionWriteRequest) (*sessionWriteRequest, int) {
	candidate, eligible, err := newCoalescibleSessionEvent(first.event)
	if first.kind != sessionWriteEvent || err != nil || !eligible {
		result := s.executeWrite(first)
		first.result <- result
		if result.err != nil {
			return nil, 0
		}
		return nil, sessionWriteCheckpointWeight(first, result)
	}
	requests, pending := s.collectPendingChunks(first, candidate)
	accepted := requests[:0]
	events := make([]store.SessionEvent, 0, len(requests))
	for _, request := range requests {
		if err := request.ctx.Err(); err != nil {
			request.result <- sessionWriteResult{err: fmt.Errorf("store: session write canceled before execution: %w", err)}
			continue
		}
		accepted = append(accepted, request)
		events = append(events, request.event)
	}
	if len(accepted) == 0 {
		return pending, 0
	}
	// A shared transaction has a writer-owned bound; one canceled caller cannot
	// cancel other accepted writes. A cancellation racing commit is an unknown ack.
	ctx, cancel := context.WithTimeout(s.writerCtx, defaultDrainTimeout)
	defer cancel()
	persisted, err := s.writeEventBatch(ctx, events)
	result := sessionWriteResult{err: err}
	if err == nil {
		if len(persisted) != 1 {
			result.err = fmt.Errorf("store: homogeneous chunk batch produced %d rows", len(persisted))
		}
		if len(persisted) == 1 {
			result.event = persisted[0]
		}
	}
	for _, request := range accepted {
		request.result <- result
	}
	if err != nil {
		return pending, 0
	}
	return pending, 1
}

func (s *SessionDB) collectPendingChunks(
	first sessionWriteRequest, candidate coalescibleSessionEvent,
) ([]sessionWriteRequest, *sessionWriteRequest) {
	requests := []sessionWriteRequest{first}
	bytes := len(candidate.text)
	timer := time.NewTimer(sessionWriterCoalesceWindow)
	defer timer.Stop()
	var pending *sessionWriteRequest
collect:
	for len(requests) < sessionWriterCoalesceRequests {
		select {
		case next := <-s.writeCh:
			chunk, eligible, err := newCoalescibleSessionEvent(next.event)
			if next.kind != sessionWriteEvent || err != nil || !eligible || chunk.key != candidate.key ||
				bytes+len(chunk.text) > sessionEventCoalesceMaxTextBytes {
				pending = &next
				break collect
			}
			requests = append(requests, next)
			bytes += len(chunk.text)
		case <-timer.C:
			break collect
		case <-s.writerCtx.Done():
			break collect
		}
	}
	return requests, pending
}
