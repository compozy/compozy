package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

func (h *HostAPIHandler) handleCallsList(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPICallsListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	reader, query, err := h.hostAPICallsReadQuery(ctx, params.Scope, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	states := make([]callspkg.State, 0, len(params.State))
	for _, value := range params.State {
		if value = strings.TrimSpace(value); value != "" {
			states = append(states, callspkg.State(value))
		}
	}
	page, err := reader.List(ctx, callspkg.CallListQuery{
		CallReadQuery: query, State: states, Caller: params.Caller,
		Cursor: params.Cursor, Limit: params.Limit,
	})
	if err != nil {
		return nil, mapHostAPICallRPCError("call", "", err)
	}
	profileName, err := h.hostAPICallProfileName(ctx, query.ReadScope.ProfileID)
	if err != nil {
		return nil, err
	}
	projected, err := reader.ProjectPayloads(ctx, page.Items)
	if err != nil {
		return nil, mapHostAPICallRPCError("call", "", err)
	}
	items := make([]apicontract.CallPayload, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, hostAPICallPayload(&page.Items[index], profileName, projected[index]))
	}
	return apicontract.CallsResponse{Items: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func (h *HostAPIHandler) handleCallsGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPICallTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	callID := strings.TrimSpace(params.CallID)
	if callID == "" {
		return nil, invalidParamsRPCError(errors.New("call_id is required"))
	}
	reader, query, err := h.hostAPICallsReadQuery(ctx, params.Scope, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	record, err := reader.GetRead(ctx, query, callID)
	if err != nil {
		return nil, mapHostAPICallRPCError("call", callID, err)
	}
	profileName, err := h.hostAPICallProfileName(ctx, query.ReadScope.ProfileID)
	if err != nil {
		return nil, err
	}
	projected, err := reader.ProjectPayloads(ctx, []callspkg.CallRecord{record})
	if err != nil {
		return nil, mapHostAPICallRPCError("call", callID, err)
	}
	return hostAPICallPayload(&record, profileName, projected[0]), nil
}

func (h *HostAPIHandler) handleCallsResult(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPICallTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	callID := strings.TrimSpace(params.CallID)
	if callID == "" {
		return nil, invalidParamsRPCError(errors.New("call_id is required"))
	}
	reader, query, err := h.hostAPICallsReadQuery(ctx, params.Scope, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	result, err := reader.Result(ctx, query, callID)
	if err != nil {
		return nil, mapHostAPICallRPCError("call_result", callID, err)
	}
	return apicontract.CallResultResponse{
		CallID: result.CallID, Result: append(json.RawMessage(nil), result.Bytes...),
	}, nil
}

func (h *HostAPIHandler) handleMessagesList(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPIMessagesListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	reader, query, err := h.hostAPICallsReadQuery(ctx, params.Scope, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	page, err := reader.ListMessages(ctx, callspkg.MessageListQuery{
		CallReadQuery: query, SessionID: params.SessionID, Cursor: params.Cursor, Limit: params.Limit,
	})
	if err != nil {
		return nil, mapHostAPICallRPCError("message", "", err)
	}
	profileName, err := h.hostAPICallProfileName(ctx, query.ReadScope.ProfileID)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.CallMessagePayload, 0, len(page.Items))
	for _, record := range page.Items {
		items = append(items, hostAPICallMessagePayload(record, profileName))
	}
	return apicontract.CallMessagesResponse{Items: items, NextCursor: page.NextCursor}, nil
}

func (h *HostAPIHandler) hostAPICallsReadQuery(
	ctx context.Context,
	rawScope string,
	rawWorkspaceID string,
) (hostAPICallsReader, callspkg.CallReadQuery, error) {
	if h.calls == nil {
		return nil, callspkg.CallReadQuery{}, unavailableRPCError(errors.New("calls reader is not configured"))
	}
	profileID, err := hostAPIProfileID(ctx)
	if err != nil {
		return nil, callspkg.CallReadQuery{}, unavailableRPCError(err)
	}
	workspaceID := strings.TrimSpace(rawWorkspaceID)
	boundWorkspaceID, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil {
		return nil, callspkg.CallReadQuery{}, invalidParamsRPCError(err)
	}
	if bound {
		if workspaceID != "" && workspaceID != boundWorkspaceID {
			return nil, callspkg.CallReadQuery{}, invalidParamsRPCError(
				errors.New("workspace_id differs from the extension workspace"),
			)
		}
		workspaceID = boundWorkspaceID
	}
	query, err := callspkg.NormalizeReadQuery(callspkg.CallReadQuery{
		ReadScope: store.ReadScope{ProfileID: profileID},
		Scope:     callspkg.Scope(rawScope), WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, callspkg.CallReadQuery{}, invalidParamsRPCError(err)
	}
	return h.calls, query, nil
}

func (h *HostAPIHandler) hostAPICallProfileName(ctx context.Context, profileID string) (string, error) {
	if h.profiles == nil {
		return "", unavailableRPCError(errors.New("profile reader is not configured"))
	}
	profiles, err := h.profiles.List(ctx)
	if err != nil {
		return "", unavailableRPCError(err)
	}
	for _, item := range profiles {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(profileID) {
			return item.Name, nil
		}
	}
	return "", notFoundRPCError("profile", profileID, nil)
}

func hostAPICallPayload(
	record *callspkg.CallRecord,
	profileName string,
	projected callspkg.ProjectionContent,
) apicontract.CallPayload {
	payload := apicontract.CallPayload{
		CallID: record.CallID, ProfileID: record.ProfileID, ProfileName: profileName,
		Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		Caller: apicontract.CallOwnerPayload{Kind: string(record.Caller.Kind), ID: record.Caller.ID},
		Actor:  apicontract.CallOwnerPayload{Kind: record.Actor.Kind, ID: record.Actor.ID},
		Agent:  record.AgentName, ChildSessionID: record.ChildSessionID,
		ParentSessionID: record.ParentSessionID, RootSessionID: record.GovernedRootID,
		Depth: record.Depth, State: string(record.State), Verdict: string(record.Verdict),
		ExpectDigest: record.ExpectDigest, ResultBytes: record.ResultBytes,
		ResultBudget: record.ResultBudget.MaxBytes, ResultOverflow: string(record.ResultBudget.Overflow),
		Strict: record.Strict, IdleTTLSeconds: hostAPIDurationSeconds(record.IdleTTL),
		FailureCode: record.FailureCode, FailureDetail: record.FailureDetail,
		FirstIssueText: record.FirstIssueText, SecondIssueText: record.SecondIssueText,
		FinalProsePreview: record.FinalProsePreview,
		RepairAttempts:    record.RepairAttempts, Replayed: record.Replayed,
		DeadlineAt: hostAPITimePointer(record.DeadlineAt), CreatedAt: record.CreatedAt,
		StartedAt: hostAPITimePointer(record.StartedAt), SettledAt: hostAPITimePointer(record.SettledAt),
		UpdatedAt: record.UpdatedAt,
	}
	if len(projected.Prompt) > 0 {
		payload.PromptPreview = hostAPIBoundedTextPreview(string(projected.Prompt), 4<<10)
		payload.PromptBytes = len(projected.Prompt)
	}
	if record.State == callspkg.StateCompleted && len(projected.Result) > 0 {
		payload.ResultPreview = hostAPIBoundedJSONPreview(projected.Result, record.ResultBudget.MaxBytes)
	}
	if len(projected.Superseded) > 0 {
		payload.SupersededPreview = hostAPIBoundedJSONPreview(projected.Superseded, record.ResultBudget.MaxBytes)
		payload.SupersededBytes = len(projected.Superseded)
	}
	if record.Verdict != "" || record.ChildSessionID != "" {
		payload.Provenance = &apicontract.CallProvenancePayload{
			ProducedBy: record.AgentName, SessionID: record.ChildSessionID, Admitted: string(record.Verdict),
		}
	}
	return payload
}

func hostAPIBoundedJSONPreview(payload []byte, limit int) json.RawMessage {
	if limit <= 0 || limit > 64<<10 {
		limit = 64 << 10
	}
	if len(payload) > limit {
		return nil
	}
	return append(json.RawMessage(nil), payload...)
}

func hostAPIBoundedTextPreview(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.ToValidUTF8(value[:limit], "")
}

func hostAPICallMessagePayload(record callspkg.MessageRecord, profileName string) apicontract.CallMessagePayload {
	return apicontract.CallMessagePayload{
		MessageID: record.MessageID, ProfileID: record.ProfileID, ProfileName: profileName,
		Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		From:          apicontract.CallOwnerPayload{Kind: record.From.Kind, ID: record.From.ID},
		FromAgentName: record.FromAgentName, ToSessionID: record.ToSessionID,
		CallID: record.CallID, Text: record.Body, Delivery: hostAPICallDelivery(string(record.Delivery)),
		Reason: record.DeliveryReason, Attempts: record.DeliveryAttempts,
		CreatedAt: record.CreatedAt, DeliveredAt: hostAPITimePointer(record.DeliveredAt),
	}
}

func hostAPICallDelivery(value string) string {
	switch strings.TrimSpace(value) {
	case "pending":
		return "queued"
	case "injected":
		return "delivered-into-turn"
	case "woken":
		return "woke"
	default:
		return strings.TrimSpace(value)
	}
}

func hostAPIDurationSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	return int64((value + time.Second - 1) / time.Second)
}

func hostAPITimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	result := value.UTC()
	return &result
}

func mapHostAPICallRPCError(resource string, id string, err error) error {
	callErr, ok := errors.AsType[*callspkg.Error](err)
	if !ok {
		return unavailableRPCError(err)
	}
	data := map[string]any{
		extensionStateError: err.Error(),
		"code":              callErr.Code,
		hostAPIResourceKey:  strings.TrimSpace(resource),
		"id":                strings.TrimSpace(id),
	}
	status, message := hostAPICallErrorStatus(callErr.Code)
	return hostAPIStatusRPCError(status, message, data)
}

func hostAPICallErrorStatus(code callspkg.ErrorCode) (int, string) {
	switch code {
	case callspkg.CodeValidation, callspkg.CodeAgentUnknown, callspkg.CodeExpectInvalid,
		callspkg.CodePromptRequired, callspkg.CodeChildrenCap, callspkg.CodeWideningRejected,
		callspkg.CodeDepthExceeded, callspkg.CodeBatchEmpty, callspkg.CodeBatchOverCap,
		callspkg.CodeResultInvalid, callspkg.CodeResultOverBudget, callspkg.CodeDeadlineInvalid,
		callspkg.CodeMessageTooLarge:
		return 400, "Invalid call"
	case callspkg.CodeTargetDenied, callspkg.CodeWorkspaceDenied, callspkg.CodeSettlementDenied,
		callspkg.CodeMessageTargetBlocked, callspkg.CodeMessageTargetDenied:
		return 403, "Forbidden"
	case callspkg.CodeNotFound, callspkg.CodeTargetExpired, callspkg.CodeMessageNotFound:
		return HostAPINotFoundCode, "Not found"
	case callspkg.CodeCanceled, callspkg.CodeTimedOut:
		return 408, "Call timed out"
	case callspkg.CodeParentTerminal, callspkg.CodeIdempotencyConflict, callspkg.CodeNotSettled,
		callspkg.CodeAlreadySettled, callspkg.CodeReturnUnbound, callspkg.CodeMessageDuplicate,
		callspkg.CodePublishNoParticipation, callspkg.CodePublishNotSettled:
		return 409, "Conflict"
	case callspkg.CodeMessageRateLimited, callspkg.CodeMessagePendingCap:
		return 429, "Rate limited"
	case callspkg.CodePublishFailed:
		return 502, "Publication failed"
	default:
		return 500, "Call failed"
	}
}
