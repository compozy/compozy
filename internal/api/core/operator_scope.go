package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

const operatorScopeDeniedCode = "agent_scope_denied"

func (h *BaseHandlers) requireOperatorSurface(c *gin.Context, surface string) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	name := strings.TrimSpace(surface)
	if name == "" {
		name = "request"
	}
	c.JSON(http.StatusForbidden, contract.ErrorPayload{
		Error: name + " is an operator surface",
		Code:  operatorScopeDeniedCode,
	})
	return false
}

func (h *BaseHandlers) requireOperatorOrGrantedAgent(c *gin.Context, surface string) bool {
	if c == nil || c.Request == nil {
		return false
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return true
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, surface)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return false
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return false
	}
	if h.WorkspaceAccess != nil {
		decision, err := h.WorkspaceAccess.Authorize(c.Request.Context(), workspaceaccess.Request{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   caller.Session.ID,
				WorkspaceID: caller.Session.WorkspaceID,
				AgentName:   caller.Session.AgentName,
			},
			Target: workspaceaccess.TargetRef{
				Kind:        workspaceaccess.TargetAggregate,
				Surface:     surface,
				ProfileID:   scope.ProfileID,
				AllProfiles: scope.AllProfiles,
			},
			ReadOnly: true,
			Seam:     workspaceaccess.SeamCatalog,
		})
		if err == nil && decision.Allowed && decision.Source == workspaceaccess.SourceAgentGrant {
			return true
		}
	}
	c.JSON(
		http.StatusForbidden,
		contract.ErrorPayload{
			Error: surface + " requires an operator or unrestricted agent",
			Code:  operatorScopeDeniedCode,
		},
	)
	return false
}

func (h *BaseHandlers) allowWorkspaceAccessWrite(c *gin.Context, value string) bool {
	if value == "" || !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	c.JSON(
		http.StatusForbidden,
		contract.ErrorPayload{
			Error: "only the operator may set workspace_access",
			Code:  "workspace_access_operator_only",
		},
	)
	return false
}
