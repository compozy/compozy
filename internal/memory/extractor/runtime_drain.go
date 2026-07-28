package extractor

import (
	"context"
	"errors"
	"fmt"
)

// Stats returns current queue counters without blocking workers.
func (r *Runtime) Stats() RuntimeStats {
	if r == nil {
		return RuntimeStats{Closed: true}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := RuntimeStats{
		ActiveProviderSessions: len(r.providerSlots),
		SkippedTurns:           r.skippedTurns,
		DroppedTurns:           r.droppedTurns,
		CoalescedTurns:         r.coalescedTurns,
		BackpressuredSessions:  r.backpressuredSessions,
		Closed:                 r.closed,
	}
	for _, state := range r.sessions {
		if state == nil {
			continue
		}
		if state.inFlight {
			stats.InFlightSessions++
		}
		if state.queued != nil {
			stats.QueuedSessions++
		}
	}
	return stats
}

// Drain waits for the current queue to empty and asks the extractor to flush.
func (r *Runtime) Drain(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("memory extractor: drain context is required")
	}
	r.mu.Lock()
	if r.draining {
		r.mu.Unlock()
		return errRuntimeDraining
	}
	r.draining = true
	for _, state := range r.sessions {
		r.stopThrottleFlushLocked(state)
	}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.draining = false
		r.mu.Unlock()
	}()
	for {
		if err := r.flushQueuedSessions(ctx); err != nil {
			return err
		}
		if err := r.waitWorkers(ctx); err != nil {
			return err
		}
		if !r.hasPendingWork() {
			break
		}
	}
	if err := r.extractor.Drain(ctx); err != nil {
		return fmt.Errorf("memory extractor: drain provider: %w", err)
	}
	return nil
}

// Close rejects new work, joins workers, and flushes the extractor.
func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("memory extractor: close context is required")
	}
	r.mu.Lock()
	alreadyClosed := r.closed
	r.closed = true
	r.mu.Unlock()
	if alreadyClosed {
		return nil
	}
	err := r.Drain(ctx)
	if err != nil {
		r.cancelWorker()
		return err
	}
	r.cancelWorker()
	return nil
}

func (r *Runtime) waitWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("memory extractor: wait workers: %w", ctx.Err())
	}
}

func (r *Runtime) flushQueuedSessions(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory extractor: flush queued sessions: %w", err)
	}
	starters := make(map[string]request)
	r.mu.Lock()
	for sessionID, state := range r.sessions {
		if state == nil || state.inFlight || state.queued == nil {
			continue
		}
		r.stopThrottleFlushLocked(state)
		starters[sessionID] = *state.queued
		state.queued = nil
		state.inFlight = true
		r.wg.Add(1)
	}
	r.mu.Unlock()
	for sessionID, req := range starters {
		go r.runSession(sessionID, req)
	}
	return nil
}

func (r *Runtime) flushSession(sessionID string) {
	r.mu.Lock()
	state := r.sessions[sessionID]
	if r.closed || r.draining || state == nil || state.inFlight || state.queued == nil {
		if state != nil {
			state.flushTimer = nil
		}
		r.mu.Unlock()
		return
	}
	current := *state.queued
	state.queued = nil
	state.flushTimer = nil
	state.inFlight = true
	r.wg.Add(1)
	r.mu.Unlock()
	go r.runSession(sessionID, current)
}

func (r *Runtime) hasPendingWork() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions) > 0
}
