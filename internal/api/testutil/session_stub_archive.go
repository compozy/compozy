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

func (s StubSessionManager) ArchiveWithStop(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if s.ArchiveWithStopFn != nil {
		return s.ArchiveWithStopFn(ctx, workspaceID, sessionID)
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

func (s StubSessionManager) Descendants(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) ([]*session.Info, error) {
	if s.DescendantsFn != nil {
		return s.DescendantsFn(ctx, workspaceID, sessionID)
	}
	return nil, nil
}

func (s StubSessionManager) ArchiveWithChildren(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	stop bool,
) (*session.SessionTree, error) {
	if s.ArchiveWithChildrenFn != nil {
		return s.ArchiveWithChildrenFn(ctx, workspaceID, sessionID, stop)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) DeleteWithChildren(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) ([]string, error) {
	if s.DeleteWithChildrenFn != nil {
		return s.DeleteWithChildrenFn(ctx, workspaceID, sessionID)
	}
	return nil, session.ErrSessionNotFound
}
