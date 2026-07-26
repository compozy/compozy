package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

const (
	promptUserKey = "user"
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
		ClientMessageID:   input.clientMessageID,
		Mode:              session.BusyInputMode(req.Mode),
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
	if strings.TrimSpace(req.Text) == "" {
		core.RespondError(c, http.StatusBadRequest, errors.New("text is required"), true)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.SteerPrompt(context.WithoutCancel(c.Request.Context()), sessionID, req.Text)
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
	message         string
	clientMessageID string
}

func extractPromptInput(req contract.SendPromptRequest) (extractedPromptInput, error) {
	if message := strings.TrimSpace(req.Message); message != "" {
		return extractedPromptInput{
			message:         message,
			clientMessageID: strings.TrimSpace(req.MessageID),
		}, nil
	}

	for _, msg := range slices.Backward(req.Messages) {
		if strings.TrimSpace(msg.Role) != promptUserKey {
			continue
		}
		clientMessageID := strings.TrimSpace(msg.ID)
		if clientMessageID == "" {
			clientMessageID = strings.TrimSpace(req.MessageID)
		}

		if content := strings.TrimSpace(msg.Content); content != "" {
			return extractedPromptInput{message: content, clientMessageID: clientMessageID}, nil
		}

		parts := make([]string, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			partType := strings.TrimSpace(part.Type)
			if partType != "" && !strings.EqualFold(partType, "text") {
				continue
			}
			if text := strings.TrimSpace(part.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return extractedPromptInput{
				message:         strings.Join(parts, "\n"),
				clientMessageID: clientMessageID,
			}, nil
		}
	}

	return extractedPromptInput{}, errors.New("message is required")
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
