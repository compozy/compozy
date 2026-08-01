package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type promptRequest = contract.SendPromptRequest
type uiMessageEnvelope = contract.PromptUIMessage
type uiMessageTextPart = contract.PromptUITextPart

func (h *Handlers) promptSession(c *gin.Context) {
	var req contract.SendPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Debug("httpapi: decode prompt request failed", "error", err)
		core.RespondError(c, http.StatusBadRequest, invalidRequestPayloadError{cause: err}, true)
		return
	}

	input, err := extractPromptInput(req)
	if err != nil {
		core.RespondError(c, http.StatusBadRequest, err, true)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	caller, err := h.PromptCallerForWorkspace(c, c.Param("workspace_id"))
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, true)
		return
	}

	executionCtx := context.WithoutCancel(c.Request.Context())
	deliveryCtx, cancelDelivery := context.WithCancel(c.Request.Context())
	defer cancelDelivery()
	result, err := h.Sessions.SendPrompt(executionCtx, sessionID, session.SendPromptOpts{
		Message:           input.message,
		MessageID:         input.messageID,
		IdempotencyKey:    input.idempotencyKey,
		Mode:              session.BusyInputMode(req.Mode),
		Runtime:           core.PromptRuntimeSelectionFromPayload(req.Runtime),
		DeliveryContext:   deliveryCtx,
		Caller:            caller,
		AllowGoalCommands: true,
	})
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, true)
		return
	}
	if result.Events == nil {
		core.RespondPromptResult(c, result, true)
		return
	}
	events := result.Events
	turnID, err := core.AcceptedPromptStreamTurnID(result)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, true)
		return
	}

	c.Header("x-vercel-ai-ui-message-stream", "v1")
	writer, err := core.PrepareSSE(c)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, true)
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
	var req contract.SteerPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, invalidRequestPayloadError{cause: err}, true)
		return
	}
	if err := req.Validate(); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, true)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.SteerPrompt(
		context.WithoutCancel(c.Request.Context()),
		sessionID,
		session.SteerPromptOpts{
			Message: req.Text, MessageID: req.MessageID, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, true)
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
	input, err := extractPromptInput(req)
	return input.message, err
}

type extractedPromptInput struct {
	message        string
	messageID      string
	idempotencyKey string
}

func extractPromptInput(req contract.SendPromptRequest) (extractedPromptInput, error) {
	input, err := contract.ExtractPromptInput(req)
	if err != nil {
		return extractedPromptInput{}, err
	}
	return extractedPromptInput{
		message: input.Message, messageID: input.MessageID, idempotencyKey: input.IdempotencyKey,
	}, nil
}

type invalidRequestPayloadError struct {
	cause error
}

func (e invalidRequestPayloadError) Error() string {
	return "invalid request payload"
}

func (e invalidRequestPayloadError) Unwrap() error {
	return e.cause
}
