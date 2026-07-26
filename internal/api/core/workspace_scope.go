package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

var errWorkspaceScopedResourceNotFound = errors.New("api: workspace-scoped resource not found")

type workspaceScope struct {
	Resolved   workspacepkg.ResolvedWorkspace
	ID         string
	RegistryID string
}

func (s *workspaceScope) NetworkChannelRef(channel string) store.NetworkChannelRef {
	return store.NetworkChannelRef{
		WorkspaceID: s.NetworkWorkspaceID(),
		Channel:     strings.TrimSpace(channel),
	}
}

func (s *workspaceScope) NetworkWorkspaceID() string {
	return strings.TrimSpace(s.RegistryID)
}

func (s *workspaceScope) SessionWorkspaceID() string {
	return strings.TrimSpace(s.RegistryID)
}

func (s *workspaceScope) BodyWorkspaceIDMatches(workspaceID string) bool {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return true
	}
	return trimmed == strings.TrimSpace(s.ID) || trimmed == strings.TrimSpace(s.RegistryID)
}

func (h *BaseHandlers) resolveWorkspaceScope(c *gin.Context) (workspaceScope, bool) {
	if c == nil {
		return workspaceScope{}, false
	}
	workspaceRef := workspaceRefFromRoute(c)
	if workspaceRef == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: workspace_id path is required", h.transportName()))
		return workspaceScope{}, false
	}
	if h.Workspaces == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: %w", h.transportName(), workspacepkg.ErrWorkspaceResolverUnavailable),
		)
		return workspaceScope{}, false
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRef)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return workspaceScope{}, false
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("%s: resolved workspace_id is empty", h.transportName()),
		)
		return workspaceScope{}, false
	}
	registryID := strings.TrimSpace(resolved.ID)
	if registryID == "" {
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("%s: resolved workspace registry id is empty", h.transportName()),
		)
		return workspaceScope{}, false
	}
	return workspaceScope{Resolved: resolved, ID: workspaceID, RegistryID: registryID}, true
}

func workspaceRefFromRoute(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value := strings.TrimSpace(c.Param("workspace_id")); value != "" {
		return value
	}
	return strings.TrimSpace(c.Param("id"))
}

func (h *BaseHandlers) requireSessionInWorkspace(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if h == nil || h.Sessions == nil {
		return nil, errors.New("api: sessions are required")
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return nil, errors.New("api: session_id is required")
	}
	info, err := h.Sessions.Status(ctx, trimmedSessionID)
	if err != nil {
		return nil, err
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return nil, errWorkspaceScopedResourceNotFound
	}
	return info, nil
}

func statusForWorkspaceScopedResourceError(err error) int {
	if errors.Is(err, errWorkspaceScopedResourceNotFound) {
		return http.StatusNotFound
	}
	return StatusForSessionError(err)
}

func (h *BaseHandlers) routeSessionInWorkspace(
	c *gin.Context,
) (workspaceScope, string, *session.Info, bool) {
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return workspaceScope{}, "", nil, false
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Param("id"))
	}
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: session_id path is required", h.transportName()))
		return workspaceScope{}, "", nil, false
	}
	info, err := h.requireSessionInWorkspace(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	if err != nil {
		h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
		return workspaceScope{}, "", nil, false
	}
	return scope, sessionID, info, true
}

// RequireRouteSessionInWorkspace validates that the route session id belongs to
// the workspace route scope before transport-specific handlers open the session.
func (h *BaseHandlers) RequireRouteSessionInWorkspace(c *gin.Context) (string, bool) {
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	return sessionID, ok
}
