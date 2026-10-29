package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

var errWorkspaceScopedResourceNotFound = errors.New("api: workspace-scoped resource not found")

type workspaceScope struct {
	Resolved   workspacepkg.ResolvedWorkspace
	ID         string
	RegistryID string
	// AllUserProfiles marks a User session route read under the all-profiles lens; the routed
	// session's own profile becomes the concrete scope profile.
	AllUserProfiles bool
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
	if !h.RequireProjectReference(c, workspaceRef) {
		return workspaceScope{}, false
	}
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
	if workspacepkg.ParseRef(workspaceRef).Kind == workspacepkg.RefUser {
		profile, err := h.resolveProfileReadScope(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return workspaceScope{}, false
		}
		if profile.AllProfiles {
			// Reading one existing User session under the all-profiles lens is allowed; the session
			// supplies the concrete profile (USW BR-7). Collection routes still need one profile.
			if routeSessionID(c) == "" {
				h.respondError(
					c,
					http.StatusBadRequest,
					errors.New("session routes in the User scope require one profile"),
				)
				return workspaceScope{}, false
			}
			return workspaceScope{
				Resolved:        workspacepkg.ResolvedWorkspace{Scope: workspacepkg.ScopeUser},
				AllUserProfiles: true,
			}, true
		}
		return workspaceScope{
			Resolved: workspacepkg.ResolvedWorkspace{Scope: workspacepkg.ScopeUser, ProfileID: profile.ProfileID},
		}, true
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRef)
	if errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) && allowsMissingWorkspace(c) {
		var registration workspacepkg.Workspace
		registration, err = h.Workspaces.Get(c.Request.Context(), workspaceRef)
		registration.RootMissing = true
		resolved = workspacepkg.ResolvedWorkspace{
			Workspace: registration, WorkspaceID: registration.ID, Scope: workspacepkg.ScopeWorkspace,
		}
		c.Set("workspace_root_missing", registration.ID)
	}
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

func allowsMissingWorkspace(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet {
		return true
	}
	if c.Request.Method == http.MethodDelete && strings.HasSuffix(c.FullPath(), "/worktrees/:worktree_id") {
		return true
	}
	return routeSessionID(c) != "" && c.Request.Method == http.MethodPost &&
		(strings.HasSuffix(c.FullPath(), "/stop") || strings.HasSuffix(c.FullPath(), "/archive"))
}

const workspaceRootMissingCode = "workspace_root_missing"

// MissingWorkspaceDiagnostics describes unavailable execution files without invalidating retained state.
func MissingWorkspaceDiagnostics(workspaceID string) []contract.DiagnosticItem {
	if workspaceID == "" {
		return nil
	}
	return []contract.DiagnosticItem{{
		ID: "workspace_root_missing:" + workspaceID, Code: workspaceRootMissingCode,
		Severity: "warn", Category: "session", DataFreshness: "live",
		Title:    "Workspace folder is missing",
		Message:  "The workspace folder is missing. Session state is retained; stop and archive remain available.",
		Evidence: map[string]any{"workspace_id": workspaceID},
	}}
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

func (h *BaseHandlers) requireSessionInProfile(
	c *gin.Context,
	info *session.Info,
	scope profilepkg.ReadScope,
) bool {
	if info == nil || !scope.Matches(info.ProfileID) {
		h.respondError(c, http.StatusNotFound, errWorkspaceScopedResourceNotFound)
		return false
	}
	return true
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
	if scope.AllUserProfiles {
		scope.Resolved.ProfileID = info.ProfileID
	}
	if scope.Resolved.Scope == workspacepkg.ScopeUser && info.ProfileID != scope.Resolved.ProfileID {
		h.respondError(c, http.StatusNotFound, errWorkspaceScopedResourceNotFound)
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

func routeSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if sessionID := strings.TrimSpace(c.Param("session_id")); sessionID != "" {
		return sessionID
	}
	if strings.Contains(c.FullPath(), "/sessions/:id") {
		return strings.TrimSpace(c.Param("id"))
	}
	return ""
}
