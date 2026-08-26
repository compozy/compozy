package core

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/gin-gonic/gin"
)

// CallMessagesCreate accepts an inert operator message for one session.
func (h *BaseHandlers) CallMessagesCreate(c *gin.Context) {
	if !h.requireCallsOperator(c, "message send") {
		return
	}
	var req contract.SendCallMessageRequest
	if err := decodeCallBody(c, &req); err != nil {
		h.respondCallsError(c, err)
		return
	}
	if strings.TrimSpace(req.To.Agent) != "" || strings.TrimSpace(req.To.SessionID) == "" {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, "message target must contain exactly one session_id"))
		return
	}
	selection, err := h.resolveProfileMutationSelection(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	scope, workspaceID, err := callSurfaceScope(c, req.Scope, req.WorkspaceID)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	actor := h.callsOperatorActor()
	message, err := h.Calls.SendMessage(context.WithoutCancel(c.Request.Context()), callspkg.SendMessageInput{
		ProfileID: selection.Scope.ProfileID, Scope: scope, WorkspaceID: workspaceID,
		From: callspkg.MessageSender{Kind: "operator", ID: actor.ID},
		To:   req.To.SessionID, CallID: req.CallID, Body: req.Text,
	})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, contract.SendCallMessageResponse{
		MessageID: message.MessageID, Delivery: publicCallDelivery(string(message.Delivery)),
	})
}

// CallMessagesList returns one profile-aware mailbox page.
func (h *BaseHandlers) CallMessagesList(c *gin.Context) {
	if !h.requireCallsOperator(c, "message list") {
		return
	}
	readScope, ok := h.callsReadScope(c)
	if !ok {
		return
	}
	base, err := callReadQuery(c, readScope)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	limit, err := parseOptionalPositiveIntQuery(c, "limit", callspkg.DefaultReadLimit, callspkg.MaxReadLimit)
	if err != nil {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, err.Error()))
		return
	}
	page, err := h.Calls.ListMessages(c.Request.Context(), callspkg.MessageListQuery{
		CallReadQuery: base, SessionID: c.Query("session"), Cursor: c.Query("cursor"), Limit: limit,
	})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	items, err := h.callMessagePayloads(c.Request.Context(), page.Items)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.CallMessagesResponse{Items: items, NextCursor: page.NextCursor})
}

// CallMessagesGet returns one message within the route's profile and workspace boundary.
func (h *BaseHandlers) CallMessagesGet(c *gin.Context) {
	if !h.requireCallsOperator(c, "message read") {
		return
	}
	readScope, ok := h.callsReadScope(c)
	if !ok {
		return
	}
	if readScope.AllProfiles {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, "message detail requires one profile"))
		return
	}
	scope, workspaceID, err := callSurfaceScope(c, "", "")
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	record, err := h.Calls.Message(c.Request.Context(), callspkg.CallScope{
		ProfileID: readScope.ProfileID, Scope: scope, WorkspaceID: workspaceID,
	}, strings.TrimSpace(c.Param("message_id")))
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, callMessagePayload(record, owners[record.ProfileID]))
}
