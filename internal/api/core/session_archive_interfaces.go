package core

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// SessionArchiveManager owns reversible workspace-scoped session catalog visibility.
type SessionArchiveManager interface {
	Archive(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
	// ArchiveWithStop stops a live session through the normal stop path before archiving it.
	ArchiveWithStop(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
	Unarchive(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
}

// SessionTreeManager owns the operations that include a session's hierarchy
// descendants.
type SessionTreeManager interface {
	// Descendants lists the session's children recursively.
	Descendants(ctx context.Context, workspaceID string, sessionID string) ([]*session.Info, error)
	// ArchiveWithChildren archives the session and its descendants together.
	ArchiveWithChildren(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		opts session.ArchiveTreeOptions,
	) (*session.SessionTree, error)
	// DeleteWithChildren deletes the descendants, then the session, returning
	// the ids deleted before any failure.
	DeleteWithChildren(ctx context.Context, workspaceID string, sessionID string) ([]string, error)
}
