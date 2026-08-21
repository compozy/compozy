package core

import (
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// windowManagerProfile resolves the profile this request acts as.
//
// Desks belong to one profile at a time, so there is no aggregate reading of them:
// `all_profiles` is refused here rather than silently answered with the caller's
// own arrangement (US-026, ADR-015).
func (h *BaseHandlers) windowManagerProfile(
	c *gin.Context,
	workspaceID windowmanager.WorkspaceID,
) (string, bool) {
	if h == nil || h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return "", false
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", false
	}
	if scope.AllProfiles {
		respondProfileError(c, &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "window desktops belong to one profile at a time",
			Action:  "name the profile whose desktops you want",
			Cause:   profilepkg.ErrInvalidInput,
		})
		return "", false
	}
	return scope.ProfileID, true
}

// windowManagerService resolves the window manager owning that profile's desks.
//
// Registration does not come through here: claiming a client is one atomic
// provider operation, so it names the profile and lets the claim resolve its own
// runtime rather than holding one across two calls.
func (h *BaseHandlers) windowManagerService(
	c *gin.Context,
	workspaceID windowmanager.WorkspaceID,
) (WindowManagerService, bool) {
	profileID, ok := h.windowManagerProfile(c, workspaceID)
	if !ok {
		return nil, false
	}
	service, err := h.WindowManager.WindowManagerFor(profileID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return nil, false
	}
	return service, true
}
