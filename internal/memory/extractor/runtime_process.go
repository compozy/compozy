package extractor

import (
	"context"

	"strconv"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (r *Runtime) runSession(sessionID string, req request) {
	defer r.wg.Done()
	current := req
	for {
		if !r.process(current) {
			r.clearSession(sessionID)
			return
		}
		r.mu.Lock()
		state := r.sessions[sessionID]
		if state == nil || state.queued == nil {
			if state != nil {
				r.stopThrottleFlushLocked(state)
			}
			delete(r.sessions, sessionID)
			r.mu.Unlock()
			return
		}
		if !r.queuedReady(*state.queued) {
			state.inFlight = false
			r.scheduleThrottleFlushLocked(sessionID, state)
			r.mu.Unlock()
			return
		}
		current = *state.queued
		state.queued = nil
		r.mu.Unlock()
	}
}

func (r *Runtime) clearSession(sessionID string) {
	r.mu.Lock()
	state := r.sessions[sessionID]
	if state != nil {
		r.stopThrottleFlushLocked(state)
	}
	delete(r.sessions, sessionID)
	r.mu.Unlock()
}

func (r *Runtime) process(req request) bool {
	release, ok := r.acquireProviderSlot()
	if !ok {
		return false
	}
	defer release()
	r.recordEvent(r.workerCtx, Event{
		Op:   EventStarted,
		Turn: req.turn,
		Metadata: map[string]string{
			"coalesced_count": strconv.Itoa(req.coalesceCount),
		},
	})
	candidates, err := r.extractor.Extract(r.workerCtx, req.turn)
	if err != nil {
		r.recordEvent(r.workerCtx, Event{Op: EventFailed, Turn: req.turn, Error: err.Error()})
		r.logger.Warn("memory extractor: extract failed", "session_id", req.turn.SessionID, "error", err)
		return true
	}
	path, count, err := r.producer.Write(r.workerCtx, req.turn, candidates)
	if err != nil {
		r.recordEvent(r.workerCtx, Event{Op: EventFailed, Turn: req.turn, Error: err.Error()})
		r.logger.Warn("memory extractor: write inbox failed", "session_id", req.turn.SessionID, "error", err)
		return true
	}
	r.recordEvent(r.workerCtx, Event{
		Op:       EventCompleted,
		Turn:     req.turn,
		TargetID: path,
		Metadata: map[string]string{
			"candidate_count": strconv.Itoa(count),
		},
	})
	return true
}

func (r *Runtime) acquireProviderSlot() (func(), bool) {
	if r.providerSlots == nil {
		return func() {}, true
	}
	select {
	case r.providerSlots <- struct{}{}:
		return r.releaseProviderSlot, true
	default:
	}
	r.mu.Lock()
	r.backpressuredSessions++
	r.mu.Unlock()
	select {
	case r.providerSlots <- struct{}{}:
		return r.releaseProviderSlot, true
	case <-r.workerCtx.Done():
		return nil, false
	}
}

func (r *Runtime) releaseProviderSlot() {
	select {
	case <-r.providerSlots:
	default:
	}
}

func (r *Runtime) recordEvent(ctx context.Context, event Event) {
	if r.events == nil {
		return
	}
	event.At = r.now().UTC()
	if err := r.events.RecordExtractorEvent(ctx, event); err != nil {
		r.logger.Warn("memory extractor: record event failed", "op", event.Op, "error", err)
	}
}

func newRequest(turn memcontract.TurnRecord, now func() time.Time) (request, error) {
	normalized, err := normalizeTurn(turn, now)
	if err != nil {
		return request{}, err
	}
	return request{turn: normalized}, nil
}
