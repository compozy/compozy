package core

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// SessionArchiveManager owns reversible workspace-scoped session catalog visibility.
type SessionArchiveManager interface {
	Archive(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
	Unarchive(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
}
