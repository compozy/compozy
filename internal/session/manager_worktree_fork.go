package session

import (
	"context"
	"errors"
	"fmt"
)

// CreateWorktreeForkAccepted atomically keeps the origin idle while accepting
// one clean child session.
func (m *Manager) CreateWorktreeForkAccepted(
	ctx context.Context,
	originSessionID string,
	opts CreateAcceptedOpts,
) (*Info, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	origin, ok := m.Get(originSessionID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, originSessionID)
	}
	if err := origin.reserveWorktreeFork(); err != nil {
		return nil, err
	}
	defer origin.releaseWorktreeFork()
	return m.CreateAccepted(ctx, opts)
}

func (s *Session) reserveWorktreeFork() error {
	if s == nil {
		return errors.New("session: origin session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.worktreeForkReserved || s.conversationRewindReserved || s.promptSetupCount > 0 ||
		s.currentTurnSource != "" || s.currentTurnID != "" {
		return ErrPromptInProgress
	}
	s.worktreeForkReserved = true
	return nil
}

func (s *Session) releaseWorktreeFork() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.worktreeForkReserved = false
	s.mu.Unlock()
}
