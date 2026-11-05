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
			ReadOnly: isReadRequest(c.Request),
			Seam:     workspaceaccess.SeamCatalog,
		})
		if err == nil && decision.Allowed {
			return true
		}
		if err == nil {
			c.JSON(http.StatusForbidden, contract.ErrorPayload{
				Error: surface + " requires an operator or an agent allowed to reach All projects: " +
					workspaceaccess.DenialMessage(decision),
				Code: operatorScopeDeniedCode,
			})
			return false
		}
	}
	c.JSON(
		http.StatusForbidden,
		contract.ErrorPayload{
			Error: surface + " requires an operator or an agent allowed to reach All projects",
			Code:  operatorScopeDeniedCode,
		},
	)
	return false
}

// isReadRequest reports whether an HTTP request only reads. The permission mode reaches All
// projects and other profiles for reads; changing them needs the unrestricted grant.
func isReadRequest(r *http.Request) bool {
	return r != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead)
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

func (h *BaseHandlers) allowAgentPermissionPolicyWrite(c *gin.Context, approval string, hasRules bool) bool {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	field := ""
	if approval != "" {
		field = "session_approval"
	} else if hasRules {
		field = "permission_rules"
	}
	if field == "" {
		return true
	}
	c.JSON(http.StatusForbidden, contract.ErrorPayload{
		Error: "only the operator may set " + field,
		Code:  field + "_operator_only",
	})
	return false
}

func (h *BaseHandlers) allowAgentMCPSecretValues(c *gin.Context, servers []contract.AgentMCPServerInput) bool {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	for _, server := range servers {
		if len(server.SecretHeaderValues)+len(server.SecretEnvValues) > 0 {
			c.JSON(
				http.StatusForbidden,
				contract.ErrorPayload{
					Error: "only the operator may set MCP secret values",
					Code:  "mcp_secret_values_operator_only",
				},
			)
			return false
		}
	}
	return true
}
