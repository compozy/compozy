package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// RequireSessionApprovalAuthority resolves agent identity before applying session policy.
func (h *BaseHandlers) RequireSessionApprovalAuthority(c *gin.Context, target string) bool {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	caller, ok := h.requireAgentCaller(c, "session.approve")
	if !ok {
		return false
	}
	ctx := session.WithActingSession(c.Request.Context(), caller.Session.ID)
	if err := session.AuthorizeSessionApproval(ctx, caller.Session.ID, target, h.Sessions.Status); err != nil {
		c.JSON(http.StatusForbidden, contract.ErrorPayload{
			Error: err.Error(), Code: "session_approve_outside_lineage",
		})
		return false
	}
	c.Request = c.Request.WithContext(ctx)
	return true
}

func (h *BaseHandlers) requireToolApprovalGrantAuthority(c *gin.Context) bool {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	caller, ok := h.requireAgentCaller(c, "tool.approvals")
	if !ok {
		return false
	}
	if err := session.AuthorizeToolApprovalGrantMutation(
		c.Request.Context(),
		caller.Session.ID,
		h.Sessions.Status,
	); err != nil {
		c.JSON(http.StatusForbidden, contract.ErrorPayload{
			Error: err.Error(), Code: "tool_approval_grants_denied",
		})
		return false
	}
	return true
}
