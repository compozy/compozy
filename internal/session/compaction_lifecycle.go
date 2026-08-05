package session

import (
	"context"
	"errors"
	"fmt"
)

func (m *Manager) finishPressureCompaction(sessionID string) {
	m.compactionLifecycle.mu.Lock()
	if state := m.compactionLifecycle.runs[sessionID]; state != nil && state.inFlight {
		state.inFlight = false
		state.cancel = nil
		if state.done != nil {
			close(state.done)
			state.done = nil
		}
	}
	m.compactionLifecycle.mu.Unlock()
}

func (m *Manager) armCompactionCooldown(sessionID string) {
	m.compactionLifecycle.mu.Lock()
	if state := m.compactionLifecycle.runs[sessionID]; state != nil {
		state.cooldownUntil = m.now().Add(m.compaction.FailureCooldown)
	}
	m.compactionLifecycle.mu.Unlock()
}

func (m *Manager) cancelAndWaitSessionCompaction(ctx context.Context, sessionID string) error {
	if m == nil {
		return nil
	}
	m.compactionLifecycle.mu.Lock()
	state := m.compactionLifecycle.runs[sessionID]
	if state == nil {
		m.compactionLifecycle.mu.Unlock()
		return nil
	}
	if state.cancel != nil {
		state.cancel()
	}
	done := state.done
	m.compactionLifecycle.mu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return fmt.Errorf("session: wait for context compaction for %q: %w", sessionID, ctx.Err())
		}
	}
	m.compactionLifecycle.mu.Lock()
	delete(m.compactionLifecycle.runs, sessionID)
	m.compactionLifecycle.mu.Unlock()
	return nil
}

func (m *Manager) shutdownCompactions(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: compaction shutdown context is required")
	}
	m.compactionLifecycle.mu.Lock()
	m.compactionLifecycle.closing = true
	for _, state := range m.compactionLifecycle.runs {
		if state.cancel != nil {
			state.cancel()
		}
	}
	m.compactionLifecycle.mu.Unlock()
	if err := m.waitForCompactions(ctx); err != nil {
		return err
	}
	m.compactionLifecycle.mu.Lock()
	clear(m.compactionLifecycle.runs)
	m.compactionLifecycle.mu.Unlock()
	return nil
}

func (m *Manager) waitForCompactions(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: wait for compactions context is required")
	}
	m.compactionLifecycle.mu.Lock()
	done := make([]<-chan struct{}, 0, len(m.compactionLifecycle.runs))
	for _, state := range m.compactionLifecycle.runs {
		if state != nil && state.done != nil {
			done = append(done, state.done)
		}
	}
	m.compactionLifecycle.mu.Unlock()
	for _, runDone := range done {
		select {
		case <-runDone:
		case <-ctx.Done():
			return fmt.Errorf("session: wait for context compactions: %w", ctx.Err())
		}
	}
	return nil
}
