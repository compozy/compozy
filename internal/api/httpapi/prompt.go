package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

type promptRequest = contract.SendPromptRequest
type uiMessageEnvelope = contract.PromptUIMessage
type uiMessageTextPart = contract.PromptUITextPart

func (h *Handlers) promptSession(c *gin.Context) {
	dispatch, ok := h.DispatchSessionPrompt(c)
	if !ok {
		return
	}
	h.RespondPromptV1(c, dispatch)
}

func (h *Handlers) interruptSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.InterruptPrompt(c.Request.Context(), sessionID)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, true)
		return
	}
	core.RespondPromptResult(c, result, true)
}

func (h *Handlers) steerSessionPrompt(c *gin.Context) {
	result, ok := h.DispatchSessionSteer(c)
	if !ok {
		return
	}
	core.RespondPromptResult(c, result, true)
}

func (h *Handlers) cancelQueuedSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	queueEntryID := strings.TrimSpace(c.Param("queue_entry_id"))
	if queueEntryID == "" {
		core.RespondError(c, http.StatusBadRequest, errors.New("queue entry id is required"), true)
		return
	}
	result, err := h.Sessions.CancelQueuedPrompt(c.Request.Context(), sessionID, queueEntryID)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, true)
		return
	}
	core.RespondPromptResult(c, result, true)
}

func extractPromptMessage(req contract.SendPromptRequest) (string, error) {
	input, err := contract.ExtractPromptInput(req)
	if err != nil {
		return "", err
	}
	return input.Message, nil
}
