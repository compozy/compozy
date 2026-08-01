package core

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// PromptDispatch owns the delivery context for one accepted prompt response.
// Its lifecycle ends when a response writer finishes or disconnects.
type PromptDispatch struct {
	result         session.SendPromptResult
	cancelDelivery context.CancelFunc
}

type invalidPromptRequestError struct {
	cause error
}

func (e invalidPromptRequestError) Error() string {
	return "invalid request payload"
}

func (e invalidPromptRequestError) Unwrap() error {
	return e.cause
}

// DispatchSessionPrompt validates and dispatches one authenticated prompt with
// its detached execution and request-bound delivery lifetimes.
func (h *BaseHandlers) DispatchSessionPrompt(c *gin.Context) (*PromptDispatch, bool) {
	var request contract.SendPromptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.Logger.Debug("api: decode prompt request failed", "transport", h.transportName(), "error", err)
		h.respondError(c, http.StatusBadRequest, invalidPromptRequestError{cause: err})
		return nil, false
	}

	input, err := contract.ExtractPromptInput(request)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return nil, false
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return nil, false
	}
	caller, err := h.PromptCallerForWorkspace(c, c.Param("workspace_id"))
	if err != nil {
		h.respondError(c, http.StatusForbidden, err)
		return nil, false
	}

	deliveryContext, cancelDelivery := context.WithCancel(c.Request.Context())
	result, err := h.Sessions.SendPrompt(
		context.WithoutCancel(c.Request.Context()),
		sessionID,
		session.SendPromptOpts{
			Message:           input.Message,
			MessageID:         input.MessageID,
			IdempotencyKey:    input.IdempotencyKey,
			Mode:              session.BusyInputMode(request.Mode),
			Runtime:           PromptRuntimeSelectionFromPayload(request.Runtime),
			DeliveryContext:   deliveryContext,
			Caller:            caller,
			AllowGoalCommands: true,
		},
	)
	if err != nil {
		cancelDelivery()
		h.respondError(c, StatusForSessionError(err), err)
		return nil, false
	}
	return &PromptDispatch{result: result, cancelDelivery: cancelDelivery}, true
}

// RespondPromptV1 writes an accepted prompt as the AI SDK v1 stream or its
// non-streaming prompt-result envelope.
func (h *BaseHandlers) RespondPromptV1(c *gin.Context, dispatch *PromptDispatch) {
	defer dispatch.cancelDelivery()
	if dispatch.result.Events == nil {
		RespondPromptResult(c, dispatch.result, h.MaskInternalErrors)
		return
	}
	turnID, err := AcceptedPromptStreamTurnID(dispatch.result)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.Header("x-vercel-ai-ui-message-stream", "v1")
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	encoder := NewPromptStreamEncoder(h.Now)
	if err := encoder.Start(writer, turnID); err != nil {
		return
	}
	DeliverPromptEventStream(
		c.Request.Context(), h.StreamDoneChannel(), dispatch.result.Events, dispatch.cancelDelivery, encoder, writer,
	)
}

// RespondPromptRaw writes an accepted prompt as raw ACP event frames or its
// non-streaming prompt-result envelope.
func (h *BaseHandlers) RespondPromptRaw(c *gin.Context, dispatch *PromptDispatch) {
	defer dispatch.cancelDelivery()
	if dispatch.result.Events == nil {
		RespondPromptResult(c, dispatch.result, h.MaskInternalErrors)
		return
	}

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, ok := <-dispatch.result.Events:
			if !ok {
				return
			}
			if err := WriteSSE(
				writer,
				SSEMessage{Name: event.Type, Data: AgentEventPayloadFromEvent(event)},
			); err != nil {
				return
			}
		}
	}
}

// DispatchSessionSteer validates and dispatches one detached steering command.
func (h *BaseHandlers) DispatchSessionSteer(c *gin.Context) (session.SendPromptResult, bool) {
	var request contract.SteerPromptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, invalidPromptRequestError{cause: err})
		return session.SendPromptResult{}, false
	}
	if err := request.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return session.SendPromptResult{}, false
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return session.SendPromptResult{}, false
	}
	result, err := h.Sessions.SteerPrompt(
		context.WithoutCancel(c.Request.Context()),
		sessionID,
		session.SteerPromptOpts{
			Message: request.Text, MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
		},
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return session.SendPromptResult{}, false
	}
	return result, true
}
