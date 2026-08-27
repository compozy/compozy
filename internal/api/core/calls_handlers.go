package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/gin-gonic/gin"
)

const callsOperatorActorKind = "human"

const callPromptPreviewBytes = 4096

// CallsCreate accepts one call or a bounded per-item batch.
func (h *BaseHandlers) CallsCreate(c *gin.Context) {
	if !h.requireCallsOperator(c, "call create") {
		return
	}
	var req contract.CreateCallRequest
	if err := decodeCallBody(c, &req); err != nil {
		h.respondCallsError(c, err)
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
	operationCtx, cancel, err := h.callsOperationContext(c.Request.Context())
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	defer cancel()
	caller, err := h.Calls.ResolveOperatorCaller(operationCtx, callspkg.CallScope{
		ProfileID: selection.Scope.ProfileID, Scope: scope, WorkspaceID: workspaceID,
	}, actor)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	inputs, batch, err := h.createCallInputs(req, selection.Scope.ProfileID, scope, workspaceID, caller, actor)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	if batch {
		h.createCallBatch(operationCtx, c, inputs)
		return
	}
	record, err := h.Calls.Create(operationCtx, inputs[0])
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	payload, err := h.callCreatePayload(operationCtx, &record)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusCreated, payload)
}

func (h *BaseHandlers) createCallBatch(
	operationCtx context.Context,
	c *gin.Context,
	inputs []callspkg.CreateInput,
) {
	outcomes, err := h.Calls.CreateBatch(operationCtx, inputs)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	items := make([]contract.CallBatchItemPayload, 0, len(outcomes))
	for _, outcome := range outcomes {
		item := contract.CallBatchItemPayload{}
		if outcome.Call != nil {
			payload, mapErr := h.callCreatePayload(operationCtx, outcome.Call)
			if mapErr != nil {
				h.respondCallsError(c, mapErr)
				return
			}
			item.CallID = payload.CallID
			item.ChildSessionID = payload.ChildSessionID
			item.State = payload.State
			item.Replayed = payload.Replayed
			item.IdleExpiresAt = payload.IdleExpiresAt
		} else if outcome.Error != nil {
			payload := callErrorResponse(outcome.Error)
			item.Error = &payload
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, items)
}

// CallsList returns a counted cursor page.
func (h *BaseHandlers) CallsList(c *gin.Context) {
	if !h.requireCallsOperator(c, "call list") {
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
	query := callspkg.CallListQuery{CallReadQuery: base}
	query.State = parseCallStates(c.QueryArray("state"))
	attention, err := parseOptionalBoolPointer(c.Query("attention"))
	if err != nil {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, "attention must be a boolean"))
		return
	}
	query.Attention = attention != nil && *attention
	query.Caller = c.Query("caller")
	query.ChildSessionID = c.Query("child_session_id")
	query.RootSessionID = c.Query("root_session_id")
	query.Agent = c.Query("agent")
	query.Cursor = c.Query("cursor")
	query.Limit, err = parseOptionalPositiveIntQuery(c, "limit", callspkg.DefaultReadLimit, callspkg.MaxReadLimit)
	if err != nil {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, err.Error()))
		return
	}
	page, err := h.Calls.List(c.Request.Context(), query)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	items, err := h.callPayloads(c.Request.Context(), page.Items)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.CallsResponse{Items: items, NextCursor: page.NextCursor, Total: page.Total})
}

// CallsGet returns one call through the list's owner boundary.
func (h *BaseHandlers) CallsGet(c *gin.Context) {
	h.callReadDetail(c, false)
}

// CallsResult returns the exact stored JSON result.
func (h *BaseHandlers) CallsResult(c *gin.Context) {
	h.callReadDetail(c, true)
}

func (h *BaseHandlers) callReadDetail(c *gin.Context, wholeResult bool) {
	if !h.requireCallsOperator(c, "call read") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	callID := strings.TrimSpace(c.Param("call_id"))
	if wholeResult {
		result, err := h.Calls.Result(c.Request.Context(), query, callID)
		if err != nil {
			h.respondCallsError(c, err)
			return
		}
		c.JSON(http.StatusOK, contract.CallResultResponse{CallID: result.CallID, Result: result.Bytes})
		return
	}
	record, err := h.Calls.GetRead(c.Request.Context(), query, callID)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	projected, err := h.Calls.ProjectPayloads(c.Request.Context(), []callspkg.CallRecord{record})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	content, err := h.callPayloadContent(c.Request.Context(), &record, projected[0])
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, callPayload(&record, owners[record.ProfileID], content))
}

// CallsPrompt returns the exact authored prompt through the call read boundary.
func (h *BaseHandlers) CallsPrompt(c *gin.Context) {
	if !h.requireCallsOperator(c, "call prompt read") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	prompt, err := h.Calls.Prompt(c.Request.Context(), query, strings.TrimSpace(c.Param("call_id")))
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.CallPromptResponse{CallID: prompt.CallID, Prompt: prompt.Text})
}

// CallsSuperseded returns preserved late-result evidence through the call read boundary.
func (h *BaseHandlers) CallsSuperseded(c *gin.Context) {
	if !h.requireCallsOperator(c, "call superseded read") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	result, err := h.Calls.Superseded(c.Request.Context(), query, strings.TrimSpace(c.Param("call_id")))
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.CallSupersededResponse{CallID: result.CallID, Result: result.Bytes})
}

// CallsAwait waits for a bounded interval and returns settled and pending identities.
func (h *BaseHandlers) CallsAwait(c *gin.Context) {
	if !h.requireCallsOperator(c, "call await") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	var req contract.AwaitCallsRequest
	if c.Request.ContentLength != 0 {
		if err := decodeCallBody(c, &req); err != nil {
			h.respondCallsError(c, err)
			return
		}
	}
	if callID := strings.TrimSpace(c.Param("call_id")); callID != "" {
		req.CallIDs = append([]string{callID}, req.CallIDs...)
	}
	outcome, err := h.Calls.Await(c.Request.Context(), callspkg.AwaitInput{
		ProfileID: query.ReadScope.ProfileID, Scope: query.Scope, WorkspaceID: query.WorkspaceID,
		CallIDs: req.CallIDs, Timeout: time.Duration(req.TimeoutMS) * time.Millisecond, Resume: req.Resume,
	})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	settled, err := h.callPayloads(c.Request.Context(), outcome.Settled)
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AwaitCallsResponse{
		Settled: settled, Pending: outcome.Pending, Outcome: string(outcome.Outcome), Resume: outcome.Resume,
		ClampedTimeoutMS: outcome.ClampedTimeout.Milliseconds(),
	})
}

// CallsCancel is idempotent and owner-bound.
func (h *BaseHandlers) CallsCancel(c *gin.Context) {
	if !h.requireCallsOperator(c, "call cancel") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	callID := strings.TrimSpace(c.Param("call_id"))
	if _, err := h.Calls.GetRead(c.Request.Context(), query, callID); err != nil {
		h.respondCallsError(c, err)
		return
	}
	var req contract.CancelCallRequest
	if c.Request.ContentLength != 0 {
		if err := decodeCallBody(c, &req); err != nil {
			h.respondCallsError(c, err)
			return
		}
	}
	operationCtx, cancel, err := h.callsOperationContext(c.Request.Context())
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	defer cancel()
	record, err := h.Calls.Cancel(operationCtx, callspkg.CancelInput{
		Scope: callspkg.CallScope{
			ProfileID: query.ReadScope.ProfileID, Scope: query.Scope, WorkspaceID: query.WorkspaceID,
		},
		CallID: callID, Reason: req.Reason, Actor: h.callsOperatorActor(),
	})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.CancelCallResponse{State: string(record.State)})
}

// CallsPublish posts one completed call as bounded Network evidence.
func (h *BaseHandlers) CallsPublish(c *gin.Context) {
	if !h.requireCallsOperator(c, "call publish") {
		return
	}
	query, ok := h.resolvedCallReadQuery(c)
	if !ok {
		return
	}
	var req contract.PublishCallRequest
	if err := decodeCallBody(c, &req); err != nil {
		h.respondCallsError(c, err)
		return
	}
	operationCtx, cancel, err := h.callsOperationContext(c.Request.Context())
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	defer cancel()
	receipt, err := h.Calls.Publish(operationCtx, callspkg.PublishInput{
		ProfileID: query.ReadScope.ProfileID, Scope: query.Scope, WorkspaceID: query.WorkspaceID,
		CallID: strings.TrimSpace(c.Param("call_id")), Actor: h.callsOperatorActor(),
		Channel: req.Channel, ThreadID: req.ThreadID,
	})
	if err != nil {
		h.respondCallsError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.PublishCallResponse{
		NetworkMessageID: receipt.NetworkMessageID,
		Published:        receipt.Published,
	})
}

func (h *BaseHandlers) requireCallsOperator(c *gin.Context, surface string) bool {
	if !h.requireOperatorSurface(c, surface) {
		return false
	}
	if h == nil || h.Calls == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: calls service is not configured"))
		return false
	}
	return true
}

func (h *BaseHandlers) callsOperatorActor() callspkg.Actor {
	return callspkg.Actor{Kind: callsOperatorActorKind, ID: "operator:" + h.transportName()}
}

func (h *BaseHandlers) callsReadScope(c *gin.Context) (callspkg.ReadScope, bool) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return callspkg.ReadScope{}, false
	}
	return callspkg.ReadScope{ProfileID: scope.ProfileID, AllProfiles: scope.AllProfiles}, true
}

func (h *BaseHandlers) resolvedCallReadQuery(c *gin.Context) (callspkg.CallReadQuery, bool) {
	readScope, ok := h.callsReadScope(c)
	if !ok {
		return callspkg.CallReadQuery{}, false
	}
	if readScope.AllProfiles {
		h.respondCallsError(c, callRequestError(callspkg.CodeValidation, "call detail requires one profile"))
		return callspkg.CallReadQuery{}, false
	}
	query, err := callReadQuery(c, readScope)
	if err != nil {
		h.respondCallsError(c, err)
		return callspkg.CallReadQuery{}, false
	}
	query.Actor = h.callsOperatorActor()
	return query, true
}

func boundedCallPreview(payload []byte, limit int) []byte {
	if limit <= 0 || limit > 64<<10 {
		limit = 64 << 10
	}
	if len(payload) <= limit {
		return append([]byte(nil), payload...)
	}
	return nil
}
