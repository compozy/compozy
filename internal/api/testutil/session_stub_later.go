package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

func (s StubSessionManager) MoveToLater(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	req session.LaterRequest,
) (*session.Info, error) {
	if s.MoveToLaterFn != nil {
		return s.MoveToLaterFn(ctx, workspaceID, sessionID, req)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) ResumeFromLater(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	withChildren bool,
) (*session.Info, error) {
	if s.ResumeFromLaterFn != nil {
		return s.ResumeFromLaterFn(ctx, workspaceID, sessionID, withChildren)
	}
	return nil, session.ErrSessionNotFound
}
