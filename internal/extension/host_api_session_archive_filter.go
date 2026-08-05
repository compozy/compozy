package extensionpkg

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func normalizeHostAPISessionArchiveFilter(raw string) (store.SessionArchiveFilter, error) {
	filter := store.SessionArchiveFilter(strings.TrimSpace(raw))
	if filter == "" {
		filter = store.SessionArchiveExclude
	}
	if err := filter.Validate(); err != nil {
		return "", invalidParamsRPCError(err)
	}
	return filter, nil
}

func (h *HostAPIHandler) resolveHostAPISessionListWorkspace(
	ctx context.Context,
	raw string,
) (string, string, error) {
	workspaceRef := strings.TrimSpace(raw)
	if workspaceRef == "" {
		return "", "", nil
	}
	if h.workspaces == nil {
		return workspaceRef, workspaceRef, nil
	}
	resolved, err := h.workspaces.Resolve(ctx, workspaceRef)
	if err != nil {
		return "", "", err
	}
	workspaceID, err := hostAPIResolvedWorkspaceRegistrationID(&resolved)
	if err != nil {
		return "", "", err
	}
	return workspaceID, strings.TrimSpace(resolved.RootDir), nil
}

func (h *HostAPIHandler) hostAPISessionWithArchiveMetadata(
	ctx context.Context,
	info *session.Info,
) (*session.Info, error) {
	if info == nil || info.State != session.StateStopped {
		return info, nil
	}
	return h.sessions.Status(ctx, info.ID)
}

func hostAPISessionMatchesArchive(info *session.Info, filter store.SessionArchiveFilter) bool {
	if info == nil {
		return false
	}
	archived := info.ArchivedAt != nil
	switch filter {
	case store.SessionArchiveExclude:
		return !archived
	case store.SessionArchiveOnly:
		return archived
	default:
		return true
	}
}

func hostAPISessionMatchesWorkspace(info *session.Info, workspaceID string, workspaceRoot string) bool {
	if info == nil {
		return false
	}
	if workspaceID == "" && workspaceRoot == "" {
		return true
	}
	return info.WorkspaceID == workspaceID || info.Workspace == workspaceRoot
}
