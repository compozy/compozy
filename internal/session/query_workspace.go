package session

import (
	"context"

	"strings"
)

func (m *Manager) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if ctx == nil {
		return "", nil
	}
	if strings.TrimSpace(workspaceID) == "" || m.workspace == nil {
		return "", nil
	}

	resolved, err := m.workspace.Resolve(ctx, workspaceID)
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
