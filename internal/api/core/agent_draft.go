package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type AgentInstructionsDrafter interface {
	DraftInstructions(context.Context, session.DraftInstructionsRequest) (string, error)
}

func (h *BaseHandlers) DraftAgentInstructions(c *gin.Context) {
	var request contract.DraftAgentInstructionsRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(request.Goal) == "" || strings.TrimSpace(request.Runtime.Provider) == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("goal and runtime.provider are required"))
		return
	}
	profile, _, err := h.agentResourceProfile(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	drafter, ok := h.Sessions.(AgentInstructionsDrafter)
	if !ok {
		c.JSON(
			http.StatusServiceUnavailable,
			contract.ErrorPayload{Code: "agent_draft_failed", Error: "Instruction drafting is unavailable"},
		)
		return
	}
	instructions, err := drafter.DraftInstructions(c.Request.Context(), session.DraftInstructionsRequest{
		Goal: request.Goal, ProfileID: profile.ProfileID, Provider: request.Runtime.Provider,
		Model: request.Runtime.Model, ReasoningEffort: request.Runtime.ReasoningEffort,
	})
	if err != nil {
		code, status := "agent_draft_failed", http.StatusBadGateway
		if errors.Is(err, acp.ErrTriageUnsupported) {
			code, status = "triage_unsupported", http.StatusUnprocessableEntity
		}
		c.JSON(status, contract.ErrorPayload{Code: code, Error: code})
		return
	}
	c.JSON(http.StatusOK, contract.DraftAgentInstructionsResponse{Instructions: instructions})
}
