package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

var errToolApprovalGrantServiceUnavailable = errors.New("tool approval grant service unavailable")

// SetToolApprovalGrant creates or replaces one explicit wider native-tool decision.
func (h *BaseHandlers) SetToolApprovalGrant(c *gin.Context) {
	service, ok := h.toolApprovalGrantService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errToolApprovalGrantServiceUnavailable)
		return
	}
	workspaceID, ok := h.resolveToolApprovalGrantWorkspace(c)
	if !ok {
		return
	}
	var req contract.ToolApprovalGrantSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode tool approval grant set request: %w", h.transportName(), err),
		)
		return
	}
	grant, err := req.Domain().BuildGrant(workspaceID)
	if err != nil {
		h.respondError(c, statusForToolApprovalGrantError(err), err)
		return
	}
	stored, err := service.PutApprovalGrant(c.Request.Context(), grant)
	if err != nil {
		h.respondError(c, statusForToolApprovalGrantError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolApprovalGrantResponse{
		Grant: contract.ToolApprovalGrantPayloadFromDomain(stored),
	})
}

// ListToolApprovalGrants returns remembered native-tool decisions for one resolved workspace.
func (h *BaseHandlers) ListToolApprovalGrants(c *gin.Context) {
	service, ok := h.toolApprovalGrantService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errToolApprovalGrantServiceUnavailable)
		return
	}
	workspaceID, ok := h.resolveToolApprovalGrantWorkspace(c)
	if !ok {
		return
	}
	grants, err := service.ListApprovalGrants(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondError(c, statusForToolApprovalGrantError(err), err)
		return
	}
	payloads := make([]contract.ToolApprovalGrantPayload, 0, len(grants))
	for _, grant := range grants {
		payloads = append(payloads, contract.ToolApprovalGrantPayloadFromDomain(grant))
	}
	c.JSON(http.StatusOK, contract.ToolApprovalGrantListResponse{Grants: payloads, Total: len(payloads)})
}

// RevokeToolApprovalGrant removes one remembered native-tool decision from the resolved workspace.
func (h *BaseHandlers) RevokeToolApprovalGrant(c *gin.Context) {
	service, ok := h.toolApprovalGrantService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errToolApprovalGrantServiceUnavailable)
		return
	}
	workspaceID, ok := h.resolveToolApprovalGrantWorkspace(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("tool approval grant id is required"))
		return
	}
	if err := service.RevokeApprovalGrant(c.Request.Context(), workspaceID, id); err != nil {
		h.respondError(c, statusForToolApprovalGrantError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) toolApprovalGrantService() (ToolApprovalGrantService, bool) {
	if h == nil || h.ApprovalGrants == nil {
		return nil, false
	}
	return h.ApprovalGrants, true
}

func (h *BaseHandlers) resolveToolApprovalGrantWorkspace(c *gin.Context) (string, bool) {
	workspaceRef := strings.TrimSpace(firstNonEmpty(c.Query("workspace_id"), c.Query("workspace")))
	if workspaceRef == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("workspace_id query is required"))
		return "", false
	}
	if h.Workspaces == nil {
		h.respondError(c, http.StatusServiceUnavailable, workspacepkg.ErrWorkspaceResolverUnavailable)
		return "", false
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRef)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return "", false
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("%s: resolved workspace registry id is empty", h.transportName()),
		)
		return "", false
	}
	return workspaceID, true
}

func statusForToolApprovalGrantError(err error) int {
	switch {
	case errors.Is(err, toolspkg.ErrApprovalGrantInvalid):
		return http.StatusBadRequest
	case errors.Is(err, toolspkg.ErrApprovalGrantNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
