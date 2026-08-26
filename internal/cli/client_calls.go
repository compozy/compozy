package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type callListQuery struct {
	WorkspaceID string
	States      []string
	Caller      string
	Cursor      string
	Limit       int
	SessionID   string
}

type callAPIClient interface {
	CreateCall(context.Context, string, contract.CreateCallRequest) (contract.CallCreatePayload, error)
	ListCalls(context.Context, callListQuery) (contract.CallsResponse, error)
	GetCall(context.Context, string, string) (contract.CallPayload, error)
	GetCallResult(context.Context, string, string) (contract.CallResultResponse, error)
	AwaitCall(context.Context, string, string, contract.AwaitCallsRequest) (contract.AwaitCallsResponse, error)
	CancelCall(context.Context, string, string, contract.CancelCallRequest) (contract.CancelCallResponse, error)
	PublishCall(context.Context, string, string, contract.PublishCallRequest) (contract.PublishCallResponse, error)
	SendCallMessage(context.Context, string, contract.SendCallMessageRequest) (contract.SendCallMessageResponse, error)
	ListCallMessages(context.Context, callListQuery) (contract.CallMessagesResponse, error)
}

func (c *daemonClient) PublishCall(
	ctx context.Context,
	workspaceID, callID string,
	request contract.PublishCallRequest,
) (contract.PublishCallResponse, error) {
	var response contract.PublishCallResponse
	err := c.doJSON(
		ctx, http.MethodPost, callClientPath(workspaceID, callID)+"/publish",
		profileQueryValues(ctx, nil), request, &response,
	)
	return response, err
}

func (c *daemonClient) CreateCall(
	ctx context.Context,
	workspaceID string,
	request contract.CreateCallRequest,
) (contract.CallCreatePayload, error) {
	var response contract.CallCreatePayload
	err := c.doJSON(ctx, http.MethodPost, callsClientPath(workspaceID), profileQueryValues(ctx, nil), request, &response)
	return response, err
}

func (c *daemonClient) ListCalls(ctx context.Context, query callListQuery) (contract.CallsResponse, error) {
	values := url.Values{}
	for _, state := range cleanCallValues(query.States) {
		values.Add("state", state)
	}
	setCallQueryValue(values, "caller", query.Caller)
	setCallQueryValue(values, "cursor", query.Cursor)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var response contract.CallsResponse
	err := c.doJSON(ctx, http.MethodGet, callsClientPath(query.WorkspaceID), profileQueryValues(ctx, values), nil, &response)
	return response, err
}

func (c *daemonClient) GetCall(ctx context.Context, workspaceID, callID string) (contract.CallPayload, error) {
	var response contract.CallPayload
	err := c.doJSON(ctx, http.MethodGet, callClientPath(workspaceID, callID), profileQueryValues(ctx, nil), nil, &response)
	return response, err
}

func (c *daemonClient) GetCallResult(
	ctx context.Context,
	workspaceID, callID string,
) (contract.CallResultResponse, error) {
	var response contract.CallResultResponse
	err := c.doJSON(ctx, http.MethodGet, callClientPath(workspaceID, callID)+"/result", profileQueryValues(ctx, nil), nil, &response)
	return response, err
}

func (c *daemonClient) AwaitCall(
	ctx context.Context,
	workspaceID, callID string,
	request contract.AwaitCallsRequest,
) (contract.AwaitCallsResponse, error) {
	var response contract.AwaitCallsResponse
	err := c.doBoundedWaitJSON(
		ctx,
		http.MethodPost,
		callClientPath(workspaceID, callID)+"/await",
		profileQueryValues(ctx, nil),
		request,
		boundedWaitDuration(request.TimeoutMS),
		&response,
	)
	return response, err
}

func (c *daemonClient) CancelCall(
	ctx context.Context,
	workspaceID, callID string,
	request contract.CancelCallRequest,
) (contract.CancelCallResponse, error) {
	var response contract.CancelCallResponse
	err := c.doJSON(ctx, http.MethodPost, callClientPath(workspaceID, callID)+"/cancel", profileQueryValues(ctx, nil), request, &response)
	return response, err
}

func (c *daemonClient) SendCallMessage(
	ctx context.Context,
	workspaceID string,
	request contract.SendCallMessageRequest,
) (contract.SendCallMessageResponse, error) {
	var response contract.SendCallMessageResponse
	err := c.doJSON(ctx, http.MethodPost, messagesClientPath(workspaceID), profileQueryValues(ctx, nil), request, &response)
	return response, err
}

func (c *daemonClient) ListCallMessages(
	ctx context.Context,
	query callListQuery,
) (contract.CallMessagesResponse, error) {
	values := url.Values{}
	setCallQueryValue(values, "session", query.SessionID)
	setCallQueryValue(values, "cursor", query.Cursor)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var response contract.CallMessagesResponse
	err := c.doJSON(ctx, http.MethodGet, messagesClientPath(query.WorkspaceID), profileQueryValues(ctx, values), nil, &response)
	return response, err
}

func callsClientPath(workspaceID string) string {
	return scopedCallsClientPath(workspaceID) + "/calls"
}

func messagesClientPath(workspaceID string) string {
	return scopedCallsClientPath(workspaceID) + "/messages"
}

func callClientPath(workspaceID, callID string) string {
	return callsClientPath(workspaceID) + "/" + url.PathEscape(strings.TrimSpace(callID))
}

func scopedCallsClientPath(workspaceID string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "/api"
	}
	return "/api/workspaces/" + url.PathEscape(workspaceID)
}

func setCallQueryValue(values url.Values, key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		values.Set(key, value)
	}
}
