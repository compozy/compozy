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
		if state.retired {
			delete(m.compactionLifecycle.runs, sessionID)
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

// cancelSessionCompaction leaves an in-flight run tracked for manager shutdown.
// Session finalization must not depend on an external compactor returning.
func (m *Manager) cancelSessionCompaction(sessionID string) {
	if m == nil {
		return
	}
	m.compactionLifecycle.mu.Lock()
	defer m.compactionLifecycle.mu.Unlock()
	state := m.compactionLifecycle.runs[sessionID]
	if state == nil {
		return
	}
	state.retired = true
	if state.cancel != nil {
		state.cancel()
	}
	if !state.inFlight {
		delete(m.compactionLifecycle.runs, sessionID)
	}
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
