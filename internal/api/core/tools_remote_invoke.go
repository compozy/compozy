package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remotehost"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

// The raw fields shadow the local caller fields so even explicit null or empty
// values count as an attempted override.
type remoteToolInvokeEnvelope struct {
	contract.ToolInvokeRequest
	SessionID     json.RawMessage `json:"session_id"`
	AgentName     json.RawMessage `json:"agent_name"`
	TurnID        json.RawMessage `json:"turn_id"`
	ApprovalToken json.RawMessage `json:"approval_token"`
}

func (h *BaseHandlers) bindToolInvoke(c *gin.Context, id toolspkg.ToolID) (contract.ToolInvokeRequest, bool) {
	if _, remote := remotehost.PrincipalFromContext(c.Request.Context()); remote {
		return h.bindRemoteToolInvoke(c, id)
	}
	var req contract.ToolInvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondToolInvokeDecodeError(c, id, err)
		return contract.ToolInvokeRequest{}, false
	}
	return req, true
}

func (h *BaseHandlers) bindRemoteToolInvoke(c *gin.Context, id toolspkg.ToolID) (contract.ToolInvokeRequest, bool) {
	for key := range c.Request.URL.Query() {
		switch strings.ToLower(key) {
		case sessionIDKey, agentNameKey, "turn_id", "approval_token":
			RespondRemoteError(c, remotehost.ErrRemoteCallerOverride)
			return contract.ToolInvokeRequest{}, false
		}
	}
	var envelope remoteToolInvokeEnvelope
	if err := c.ShouldBindJSON(&envelope); err != nil {
		h.respondToolInvokeDecodeError(c, id, err)
		return contract.ToolInvokeRequest{}, false
	}
	if envelope.SessionID != nil || envelope.AgentName != nil || envelope.TurnID != nil ||
		envelope.ApprovalToken != nil {
		RespondRemoteError(c, remotehost.ErrRemoteCallerOverride)
		return contract.ToolInvokeRequest{}, false
	}
	return envelope.ToolInvokeRequest, true
}

func (h *BaseHandlers) respondToolInvokeDecodeError(c *gin.Context, id toolspkg.ToolID, err error) {
	h.respondToolError(c, toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		fmt.Sprintf("%s: decode tool invoke request: %v", h.transportName(), err),
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	))
}
