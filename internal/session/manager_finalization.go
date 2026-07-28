package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type sessionFinalization struct {
	done      chan struct{}
	err       error
	claimable bool
	completed bool
}

func (m *Manager) claimOrWaitFinalization(ctx context.Context, session *Session) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	owned, finalization := m.claimFinalization(session)
	if owned || finalization == nil {
		return owned, nil
	}

	select {
	case <-finalization.done:
		return false, finalization.err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (m *Manager) finishFinalization(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if finalization, ok := m.finalizing[id]; ok {
		finalization.err = err
		finalization.completed = true
		close(finalization.done)
	}
	delete(m.finalizing, id)
}

func (m *Manager) stopTarget(id string) (*Session, *sessionFinalization, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, nil, errors.New("session: session id is required")
	}

	m.mu.RLock()
	session := m.sessions[target]
	finalization := m.finalizing[target]
	m.mu.RUnlock()
	if session == nil && finalization == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	return session, finalization, nil
}

func waitForSessionFinalization(ctx context.Context, finalization *sessionFinalization) error {
	select {
	case <-finalization.done:
		return finalization.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) claimFinalization(session *Session) (bool, *sessionFinalization) {
	if session == nil {
		return false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if finalization, ok := m.finalizing[session.ID]; ok {
		if finalization.claimable && !finalization.completed {
			finalization.claimable = false
			return true, finalization
		}
		return false, finalization
	}

	current, ok := m.sessions[session.ID]
	if !ok || current != session {
		return false, nil
	}

	finalization := &sessionFinalization{done: make(chan struct{})}
	m.finalizing[session.ID] = finalization
	return true, finalization
}

// observeFinalization registers a waiter before a stop request can wake the
// process watcher. The observer does not own finalization: the watcher or the
// stopping caller may claim it, but both retain the same result channel.
func (m *Manager) observeFinalization(session *Session) *sessionFinalization {
	if session == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if finalization, ok := m.finalizing[session.ID]; ok {
		return finalization
	}
	current, ok := m.sessions[session.ID]
	if !ok || current != session {
		return nil
	}

	finalization := &sessionFinalization{done: make(chan struct{}), claimable: true}
	m.finalizing[session.ID] = finalization
	return finalization
}

func (m *Manager) claimOrWaitObservedFinalization(
	ctx context.Context,
	session *Session,
	observed *sessionFinalization,
) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if observed == nil {
		return m.claimOrWaitFinalization(ctx, session)
	}

	m.mu.Lock()
	switch {
	case observed.completed:
		err := observed.err
		m.mu.Unlock()
		return false, err
	case observed.claimable && m.finalizing[session.ID] == observed && m.sessions[session.ID] == session:
		observed.claimable = false
		m.mu.Unlock()
		return true, nil
	default:
		m.mu.Unlock()
	}

	return false, waitForSessionFinalization(ctx, observed)
}

// WaitForFinalizations blocks until all in-flight finalization routines finish.
func (m *Manager) WaitForFinalizations(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: wait for finalizations context is required")
	}

	for {
		m.mu.RLock()
		pending := make([]<-chan struct{}, 0, len(m.finalizing))
		for _, finalization := range m.finalizing {
			if finalization != nil {
				pending = append(pending, finalization.done)
			}
		}
		m.mu.RUnlock()

		if len(pending) == 0 {
			return nil
		}

		for _, done := range pending {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
