package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
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
