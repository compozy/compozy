package windowmanager

import (
	"context"
	"fmt"
)

func (m *Manager) resolveWorkspaceDefaults(
	ctx context.Context,
	workspaceID WorkspaceID,
) (Config, error) {
	defaults := m.defaultConfig()
	if m.workspaceConfig == nil {
		return defaults, nil
	}
	resolved, err := m.workspaceConfig.ResolveWindowManagerConfig(ctx, workspaceID, defaults)
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspace window manager config %q: %w", workspaceID, err)
	}
	canonical, err := canonicalConfig(resolved)
	if err != nil {
		return Config{}, fmt.Errorf("validate workspace window manager config %q: %w", workspaceID, err)
	}
	return canonical, nil
}
