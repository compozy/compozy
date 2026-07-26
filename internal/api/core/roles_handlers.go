package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

var errRolesStatusUnavailable = errors.New("api: roles status provider is unavailable")

// ListRoles returns the closed effective role roster for an optional workspace.
func (h *BaseHandlers) ListRoles(c *gin.Context) {
	if h.Roles == nil {
		h.respondError(c, http.StatusServiceUnavailable, errRolesStatusUnavailable)
		return
	}
	statuses, err := h.Roles.RoleStatuses(c.Request.Context(), strings.TrimSpace(c.Query("workspace")))
	if err != nil {
		h.respondError(c, statusForRoleStatusError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RolesResponse{Roles: statuses})
}

// GetRole returns one effective role projection for an optional workspace.
func (h *BaseHandlers) GetRole(c *gin.Context) {
	if h.Roles == nil {
		h.respondError(c, http.StatusServiceUnavailable, errRolesStatusUnavailable)
		return
	}
	status, err := h.Roles.RoleStatus(
		c.Request.Context(),
		strings.TrimSpace(c.Query("workspace")),
		strings.TrimSpace(c.Param("role")),
	)
	if err != nil {
		h.respondError(c, statusForRoleStatusError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RoleStatusResponse{Role: status})
}

func statusForRoleStatusError(err error) int {
	if diagnosticCodeFromError(err) == contract.CodeRoleUnknown {
		return http.StatusNotFound
	}
	return statusForWorkspaceError(err)
}
