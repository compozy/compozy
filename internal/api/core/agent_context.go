package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// AgentContext returns the bounded situation payload for the validated caller session.
func (h *BaseHandlers) AgentContext(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, "agent.context")
	if !ok {
		return
	}
	if h.AgentContextService == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: agent context service is not configured"))
		return
	}
	payload, err := h.AgentContextService.ContextForSession(c.Request.Context(), sessionInfoFromAgentCaller(caller))
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentContextResponse{Context: contract.NormalizeAgentContextPayload(&payload)})
}

func (h *BaseHandlers) enrichAgentMePayload(
	ctx context.Context,
	caller agentidentity.Caller,
	payload *contract.AgentMePayload,
) {
	if h == nil || payload == nil {
		return
	}
	if h.AgentContextService != nil {
		contextPayload, err := h.AgentContextService.ContextForSession(ctx, sessionInfoFromAgentCaller(caller))
		if err == nil {
			payload.Workspace = contextPayload.Workspace
			payload.Capabilities = contextPayload.Capabilities.Capabilities
			payload.Limits = contextPayload.Limits
			if contextPayload.Task.Lease != nil {
				payload.ActiveTaskLeases = []contract.TaskRunLeaseSummaryPayload{*contextPayload.Task.Lease}
			}
		}
	}
	if coordinatorPayload, err := h.agentCoordinatorConfigPayload(ctx, caller.Session.WorkspaceID); err == nil {
		payload.Coordinator = coordinatorPayload
	}
}

func sessionInfoFromAgentCaller(caller agentidentity.Caller) *session.Info {
	return &session.Info{
		ID:               strings.TrimSpace(caller.Session.ID),
		ProfileID:        strings.TrimSpace(caller.Session.ProfileID),
		Name:             strings.TrimSpace(caller.Session.Name),
		AgentName:        strings.TrimSpace(caller.Session.AgentName),
		Provider:         strings.TrimSpace(caller.Session.Provider),
		Model:            strings.TrimSpace(caller.Session.Model),
		WorkspaceID:      strings.TrimSpace(caller.Session.WorkspaceID),
		Workspace:        strings.TrimSpace(caller.Session.WorkspacePath),
		Type:             caller.Session.Type,
		Lineage:          store.CloneSessionLineage(caller.Session.Lineage),
		State:            caller.Session.State,
		SoulSnapshotID:   strings.TrimSpace(caller.Session.SoulSnapshotID),
		SoulDigest:       strings.TrimSpace(caller.Session.SoulDigest),
		ParentSoulDigest: strings.TrimSpace(caller.Session.ParentSoulDigest),
		CreatedAt:        caller.Session.CreatedAt,
		UpdatedAt:        caller.Session.UpdatedAt,
	}
}
