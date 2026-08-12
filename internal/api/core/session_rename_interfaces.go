package core

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// SessionRenameManager owns durable workspace-scoped session display names.
type SessionRenameManager interface {
	Rename(ctx context.Context, workspaceID string, sessionID string, name string) (*session.Info, error)
}
