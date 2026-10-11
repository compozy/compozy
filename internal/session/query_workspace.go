package session

import (
	"context"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// WorkspaceResolver supplies runtime snapshots and inexpensive persisted registrations.
type WorkspaceResolver interface {
	workspacepkg.RuntimeResolver
	Get(context.Context, string) (workspacepkg.Workspace, error)
}

func (m *Manager) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if ctx == nil {
		return "", nil
	}
	if strings.TrimSpace(workspaceID) == "" || m.workspace == nil {
		return "", nil
	}

	resolved, err := m.workspace.Get(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolved.RootDir), nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
