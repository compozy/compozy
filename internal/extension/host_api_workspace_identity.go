package extensionpkg

import (
	"context"
	"errors"
	"strings"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *HostAPIHandler) resolveWorkspaceRoot(ctx context.Context, rawWorkspace string) (string, error) {
	if strings.TrimSpace(rawWorkspace) == "" {
		return "", nil
	}
	if h.workspaces == nil {
		return "", invalidParamsRPCError(errors.New("workspace resolver is not configured"))
	}
	resolved, err := h.workspaces.Resolve(ctx, rawWorkspace)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolved.RootDir), nil
}

func (h *HostAPIHandler) resolveWorkspaceRegistrationID(ctx context.Context, rawWorkspace string) (string, error) {
	trimmed := strings.TrimSpace(rawWorkspace)
	if trimmed == "" {
		return "", nil
	}
	if h.workspaces == nil {
		return trimmed, nil
	}
	resolved, err := h.workspaces.Resolve(ctx, trimmed)
	if err != nil {
		return "", err
	}
	return hostAPIResolvedWorkspaceRegistrationID(&resolved)
}

func (h *HostAPIHandler) resolveStableWorkspaceID(ctx context.Context, rawWorkspace string) (string, error) {
	trimmed := strings.TrimSpace(rawWorkspace)
	if trimmed == "" {
		return "", nil
	}
	if h.workspaces == nil {
		return trimmed, nil
	}
	resolved, err := h.workspaces.Resolve(ctx, trimmed)
	if err != nil {
		return "", err
	}
	return hostAPIResolvedStableWorkspaceID(&resolved)
}

func (h *HostAPIHandler) resolveRequiredWorkspaceID(ctx context.Context, rawWorkspace string) (string, error) {
	workspaceID, err := h.resolveWorkspaceRegistrationID(ctx, rawWorkspace)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return "", invalidParamsRPCError(errors.New("workspace_id is required"))
	}
	return strings.TrimSpace(workspaceID), nil
}

func hostAPIResolvedWorkspaceRegistrationID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("extension: resolved workspace is required")
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		return "", errors.New("extension: resolved workspace registration id is empty")
	}
	return workspaceID, nil
}

func hostAPIResolvedStableWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("extension: resolved workspace is required")
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("extension: resolved stable workspace id is empty")
	}
	return workspaceID, nil
}
