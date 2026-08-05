package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

func (s StubSessionManager) Archive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.ArchiveFn != nil {
		return s.ArchiveFn(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) Unarchive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.UnarchiveFn != nil {
		return s.UnarchiveFn(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}
