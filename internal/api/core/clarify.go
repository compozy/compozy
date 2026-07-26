package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/gin-gonic/gin"
)

// ListSessionClarifications returns only the broker's live projection for one authorized session.
func (h *BaseHandlers) ListSessionClarifications(c *gin.Context) {
	if h.Clarify == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: clarification broker is required"))
		return
	}
	scope, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	pending, err := h.Clarify.Pending(c.Request.Context(), toolspkg.Scope{
		WorkspaceID: scope.SessionWorkspaceID(),
		SessionID:   sessionID,
		AgentName:   info.AgentName,
		Operator:    true,
	})
	if err != nil {
		h.respondError(c, statusForClarifyError(err), err)
		return
	}
	payloads := make([]contract.ClarificationPendingPayload, 0, len(pending))
	for _, request := range pending {
		payloads = append(payloads, contract.ClarificationPendingPayloadFromDomain(request))
	}
	c.JSON(http.StatusOK, contract.ClarificationsResponse{Clarifications: payloads})
}

// AnswerSessionClarification resolves one live request after workspace/session authorization.
func (h *BaseHandlers) AnswerSessionClarification(c *gin.Context) {
	if h.Clarify == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: clarification broker is required"))
		return
	}
	scope, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: clarification request_id is required"))
		return
	}
	var request contract.ClarificationAnswerRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	answer, err := h.Clarify.Answer(
		c.Request.Context(),
		toolspkg.Scope{
			WorkspaceID: scope.SessionWorkspaceID(),
			SessionID:   sessionID,
			AgentName:   info.AgentName,
			Operator:    true,
		},
		requestID,
		toolspkg.ClarifyAnswerRequest{ChoiceIndex: request.ChoiceIndex, Text: request.Text},
	)
	if err != nil {
		h.respondError(c, statusForClarifyError(err), err)
		return
	}
	c.JSON(http.StatusOK, answer)
}

func statusForClarifyError(err error) int {
	switch {
	case errors.Is(err, toolspkg.ErrClarifyInvalid):
		return http.StatusBadRequest
	case errors.Is(err, toolspkg.ErrClarifyNotFound):
		return http.StatusNotFound
	case errors.Is(err, toolspkg.ErrClarifyPending), errors.Is(err, toolspkg.ErrClarifyCanceled):
		return http.StatusConflict
	case errors.Is(err, toolspkg.ErrClarifyClosed):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
