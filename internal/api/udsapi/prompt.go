package udsapi

import (
	"errors"
	"net/http"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

const promptStreamFormatRaw = "raw"

func (h *Handlers) promptSession(c *gin.Context) {
	dispatch, ok := h.DispatchSessionPrompt(c)
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(c.Query("format")), promptStreamFormatRaw) {
		h.RespondPromptRaw(c, dispatch)
		return
	}
	h.RespondPromptV1(c, dispatch)
}

func (h *Handlers) steerSessionPrompt(c *gin.Context) {
	result, ok := h.DispatchSessionSteer(c)
	if !ok {
		return
	}
	core.RespondPromptResult(c, result, false)
}

func (h *Handlers) listSessionInputs(c *gin.Context) {
	h.HandleListSessionInputs(c)
}

func (h *Handlers) replaceSessionInput(c *gin.Context) {
	h.HandleReplaceSessionInput(c)
}

func (h *Handlers) promoteSessionInput(c *gin.Context) {
	h.HandlePromoteSessionInput(c)
}

func (h *Handlers) cancelQueuedSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	queueEntryID := strings.TrimSpace(c.Param("queue_entry_id"))
	if queueEntryID == "" {
		core.RespondError(c, http.StatusBadRequest, errors.New("queue entry id is required"), false)
		return
	}
	result, err := h.Sessions.CancelQueuedPrompt(c.Request.Context(), sessionID, queueEntryID)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}
	core.RespondPromptResult(c, result, false)
}
