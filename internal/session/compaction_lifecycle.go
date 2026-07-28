package session

import (
	"context"
	"errors"
	"fmt"
)

func (m *Manager) finishPressureCompaction(sessionID string) {
	m.compactionMu.Lock()
	if state := m.compactions[sessionID]; state != nil && state.inFlight {
		state.inFlight = false
		state.cancel = nil
		if state.done != nil {
			close(state.done)
			state.done = nil
		}
	}
	m.compactionMu.Unlock()
	m.compactionWG.Done()
}

func (m *Manager) armCompactionCooldown(sessionID string) {
	m.compactionMu.Lock()
	if state := m.compactions[sessionID]; state != nil {
		state.cooldownUntil = m.now().Add(m.compaction.FailureCooldown)
	}
	m.compactionMu.Unlock()
}

func (m *Manager) cancelAndWaitSessionCompaction(ctx context.Context, sessionID string) error {
	if m == nil {
		return nil
	}
	m.compactionMu.Lock()
	state := m.compactions[sessionID]
	if state == nil {
		m.compactionMu.Unlock()
		return nil
	}
	if state.cancel != nil {
		state.cancel()
	}
	done := state.done
	m.compactionMu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return fmt.Errorf("session: wait for context compaction for %q: %w", sessionID, ctx.Err())
		}
	}
	m.compactionMu.Lock()
	delete(m.compactions, sessionID)
	m.compactionMu.Unlock()
	return nil
}

func (m *Manager) shutdownCompactions(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: compaction shutdown context is required")
	}
	m.compactionMu.Lock()
	m.compactionClosing = true
	for _, state := range m.compactions {
		if state.cancel != nil {
			state.cancel()
		}
	}
	m.compactionMu.Unlock()
	if err := m.waitForCompactions(ctx); err != nil {
		return err
	}
	m.compactionMu.Lock()
	clear(m.compactions)
	m.compactionMu.Unlock()
	return nil
}

func (m *Manager) waitForCompactions(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: wait for compactions context is required")
	}
	done := make(chan struct{})
	go func() {
		m.compactionWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("session: wait for context compactions: %w", ctx.Err())
	}
}
