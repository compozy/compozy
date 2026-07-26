package udsapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

const promptStreamFormatRaw = "raw"

type promptRequest = contract.SendPromptRequest

func (h *Handlers) promptSession(c *gin.Context) {
	var req promptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, fmt.Errorf("udsapi: decode prompt request: %w", err), false)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		core.RespondError(c, http.StatusBadRequest, errors.New("message is required"), false)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	caller, err := h.PromptCallerForWorkspace(c, c.Param("workspace_id"))
	if err != nil {
		core.RespondError(c, core.StatusForTaskError(err), err, false)
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Query("format")), promptStreamFormatRaw) {
		h.promptSessionRaw(
			c,
			sessionID,
			req.Message,
			strings.TrimSpace(req.MessageID),
			session.BusyInputMode(req.Mode),
			caller,
		)
		return
	}

	executionCtx := context.WithoutCancel(c.Request.Context())
	deliveryCtx, cancelDelivery := context.WithCancel(c.Request.Context())
	defer cancelDelivery()
	result, err := h.Sessions.SendPrompt(executionCtx, sessionID, session.SendPromptOpts{
		Message:           req.Message,
		ClientMessageID:   strings.TrimSpace(req.MessageID),
		Mode:              session.BusyInputMode(req.Mode),
		DeliveryContext:   deliveryCtx,
		Caller:            caller,
		AllowGoalCommands: true,
	})
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}
	if result.Events == nil {
		core.RespondPromptResult(c, result, false)
		return
	}
	events := result.Events
	turnID, err := core.AcceptedPromptStreamTurnID(result)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}

	c.Header("x-vercel-ai-ui-message-stream", "v1")
	writer, err := core.PrepareSSE(c)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}
	streamEncoder := core.NewPromptStreamEncoder(h.Now)
	if err := streamEncoder.Start(writer, turnID); err != nil {
		cancelDelivery()
		return
	}

	core.DeliverPromptEventStream(
		c.Request.Context(), h.StreamDoneChannel(), events, cancelDelivery, streamEncoder, writer,
	)
}

func (h *Handlers) promptSessionRaw(
	c *gin.Context,
	sessionID string,
	message string,
	clientMessageID string,
	mode session.BusyInputMode,
	caller session.PromptCaller,
) {
	executionCtx := context.WithoutCancel(c.Request.Context())
	deliveryCtx, cancelDelivery := context.WithCancel(c.Request.Context())
	defer cancelDelivery()
	result, err := h.Sessions.SendPrompt(executionCtx, sessionID, session.SendPromptOpts{
		Message:           message,
		ClientMessageID:   clientMessageID,
		Mode:              mode,
		DeliveryContext:   deliveryCtx,
		Caller:            caller,
		AllowGoalCommands: true,
	})
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}
	if result.Events == nil {
		core.RespondPromptResult(c, result, false)
		return
	}
	events := result.Events

	writer, err := core.PrepareSSE(c)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			cancelDelivery()
			return
		case <-h.StreamDoneChannel():
			cancelDelivery()
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := core.WriteSSE(writer, core.SSEMessage{
				Name: event.Type,
				Data: core.AgentEventPayloadFromEvent(event),
			}); err != nil {
				cancelDelivery()
				return
			}
		}
	}
}

func (h *Handlers) interruptSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.InterruptPrompt(c.Request.Context(), sessionID)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}
	core.RespondPromptResult(c, result, false)
}

func (h *Handlers) steerSessionPrompt(c *gin.Context) {
	var req contract.SteerPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, fmt.Errorf("udsapi: decode steer request: %w", err), false)
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		core.RespondError(c, http.StatusBadRequest, errors.New("text is required"), false)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.SteerPrompt(c.Request.Context(), sessionID, req.Text)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}
	core.RespondPromptResult(c, result, false)
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
