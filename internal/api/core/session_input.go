package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// HandleListSessionInputs returns authoritative daemon-owned pending input.
func (h *BaseHandlers) HandleListSessionInputs(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	inputs, err := h.Sessions.ListPendingInputs(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	payloads := make([]contract.SessionInputPayload, 0, len(inputs))
	for _, input := range inputs {
		payloads = append(payloads, SessionInputPayloadFromSession(input))
	}
	c.JSON(http.StatusOK, contract.SessionInputListResponse{Inputs: payloads})
}

// HandleReplaceSessionInput atomically replaces one queued input.
func (h *BaseHandlers) HandleReplaceSessionInput(c *gin.Context) {
	sessionID, entryID, ok := h.requireSessionInputRoute(c)
	if !ok {
		return
	}
	var request contract.ReplaceSessionInputRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, invalidPromptRequestError{cause: err})
		return
	}
	if err := request.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	caller, err := h.PromptCallerForWorkspace(c, c.Param("workspace_id"))
	if err != nil {
		h.respondError(c, http.StatusForbidden, err)
		return
	}
	input, err := h.Sessions.ReplacePendingInput(
		c.Request.Context(),
		sessionID,
		entryID,
		session.ReplacePendingInputOpts{
			Text: request.Text, MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
			AllowCommands: true,
			Caller:        caller,
		},
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionInputResponse{Input: SessionInputPayloadFromSession(input)})
}

// HandlePromoteSessionInput atomically promotes queued input to steering.
func (h *BaseHandlers) HandlePromoteSessionInput(c *gin.Context) {
	sessionID, entryID, ok := h.requireSessionInputRoute(c)
	if !ok {
		return
	}
	var request contract.PromoteSessionInputRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, invalidPromptRequestError{cause: err})
		return
	}
	if err := request.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	caller, err := h.PromptCallerForWorkspace(c, c.Param("workspace_id"))
	if err != nil {
		h.respondError(c, http.StatusForbidden, err)
		return
	}
	result, err := h.Sessions.PromotePendingInputToSteer(
		c.Request.Context(),
		sessionID,
		entryID,
		session.PromotePendingInputOpts{
			Text: request.Text, MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
			ExpectedTurnID: request.ExpectedTurnID, AllowCommands: true, Caller: caller,
		},
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	RespondPromptResult(c, result, h.MaskInternalErrors)
}

// SessionInputPayloadFromSession maps daemon state into the public contract.
func SessionInputPayloadFromSession(input session.PendingInput) contract.SessionInputPayload {
	payload := contract.SessionInputPayload{
		ID: input.ID, SessionID: input.SessionID, MessageID: input.MessageID,
		IdempotencyKey: input.IdempotencyKey, TargetTurnID: input.TargetTurnID,
		Status: input.Status, Mode: contract.PromptMode(input.Mode),
		Delivery: contract.PromptDelivery(input.Delivery), Text: input.Text,
		QueueGeneration: input.QueueGeneration, EnqueuedAt: input.EnqueuedAt,
	}
	if input.Runtime != nil {
		payload.Runtime = contract.PromptRuntimeSelectionPayloadFromSelection(input.Runtime)
	}
	for _, invocation := range input.SkillInvocations {
		payload.SkillInvocations = append(payload.SkillInvocations, contract.SessionSkillInvocationPayload{
			CommandID: invocation.Ref.CommandID, Token: invocation.Token, Name: invocation.Ref.Name,
			Source: contract.SessionCommandSource{
				Kind: invocation.Ref.Source.Kind, ID: invocation.Ref.Source.ID,
				Key: invocation.Ref.Source.Key, Scope: invocation.Ref.Source.Scope,
			},
			Start: invocation.Start, End: invocation.End,
		})
	}
	return payload
}

func (h *BaseHandlers) requireSessionInputRoute(c *gin.Context) (string, string, bool) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return "", "", false
	}
	entryID := strings.TrimSpace(c.Param("queue_entry_id"))
	if entryID == "" {
		h.respondError(c, http.StatusBadRequest, invalidPromptRequestError{})
		return "", "", false
	}
	return sessionID, entryID, true
}
