package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// AdoptSession delegates to AdoptSessionFn or reports a missing session.
func (s *StubSessionManager) AdoptSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	parentSessionID string,
) (*session.Info, error) {
	if s.AdoptSessionFn != nil {
		return s.AdoptSessionFn(ctx, workspaceID, sessionID, parentSessionID)
	}
	return nil, session.ErrSessionNotFound
}

// DetachSession delegates to DetachSessionFn or reports a missing session.
func (s *StubSessionManager) DetachSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.DetachSessionFn != nil {
		return s.DetachSessionFn(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}
