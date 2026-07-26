package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// NetworkCoordinationClient is the CLI transport surface for Network coordination and usage.
type NetworkCoordinationClient interface {
	GetNetworkCoordination(
		ctx context.Context,
		workspaceRef string,
		ref NetworkCoordinationRef,
	) (NetworkCoordinationRecord, error)
	PutNetworkCoordination(
		ctx context.Context,
		workspaceRef string,
		request PutNetworkCoordinationRequest,
	) (NetworkCoordinationRecord, error)
	PutNetworkCoordinationInvitation(
		ctx context.Context,
		workspaceRef string,
		request PutNetworkCoordinationInvitationRequest,
	) (NetworkCoordinationRecord, error)
	GetNetworkUsage(
		ctx context.Context,
		workspaceRef string,
		query NetworkUsageQuery,
	) (NetworkUsageRecord, error)
}

// NetworkCoordinationRecord is the shared coordination payload.
type NetworkCoordinationRecord = contract.NetworkCoordinationPayload

// NetworkCoordinationRef selects one explicit coordination scope and optional run eligibility view.
type NetworkCoordinationRef struct {
	Scope  string
	TaskID string
	RunID  string
}

// NetworkUsageRecord is the shared usage response payload.
type NetworkUsageRecord = contract.NetworkUsageResponse

// NetworkUsageQuery is the public bounded usage-filter contract.
type NetworkUsageQuery struct {
	OwnerKind string
	OwnerID   string
	RunID     string
	Channel   string
	Cursor    string
	Limit     int
}

// PutNetworkCoordinationRequest is the shared coordination mutation payload.
type PutNetworkCoordinationRequest = contract.PutNetworkCoordinationRequest

// PutNetworkCoordinationInvitationRequest is the shared invitation mutation payload.
type PutNetworkCoordinationInvitationRequest = contract.PutNetworkCoordinationInvitationRequest

func (c *unixSocketClient) GetNetworkCoordination(
	ctx context.Context,
	workspaceRef string,
	ref NetworkCoordinationRef,
) (NetworkCoordinationRecord, error) {
	path, err := networkCoordinationPath(workspaceRef)
	if err != nil {
		return NetworkCoordinationRecord{}, err
	}
	values := url.Values{}
	values.Set("scope", strings.TrimSpace(ref.Scope))
	if trimmed := strings.TrimSpace(ref.TaskID); trimmed != "" {
		values.Set("task_id", trimmed)
	}
	if trimmed := strings.TrimSpace(ref.RunID); trimmed != "" {
		values.Set("run_id", trimmed)
	}
	var response contract.NetworkCoordinationResponse
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return NetworkCoordinationRecord{}, err
	}
	return response.Coordination, nil
}

func (c *unixSocketClient) PutNetworkCoordination(
	ctx context.Context,
	workspaceRef string,
	request PutNetworkCoordinationRequest,
) (NetworkCoordinationRecord, error) {
	path, err := networkCoordinationPath(workspaceRef)
	if err != nil {
		return NetworkCoordinationRecord{}, err
	}
	var response contract.NetworkCoordinationResponse
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return NetworkCoordinationRecord{}, err
	}
	return response.Coordination, nil
}

func (c *unixSocketClient) PutNetworkCoordinationInvitation(
	ctx context.Context,
	workspaceRef string,
	request PutNetworkCoordinationInvitationRequest,
) (NetworkCoordinationRecord, error) {
	path, err := networkCoordinationPath(workspaceRef)
	if err != nil {
		return NetworkCoordinationRecord{}, err
	}
	var response contract.NetworkCoordinationResponse
	if err := c.doJSON(ctx, http.MethodPut, path+"/invitation", nil, request, &response); err != nil {
		return NetworkCoordinationRecord{}, err
	}
	return response.Coordination, nil
}

func (c *unixSocketClient) GetNetworkUsage(
	ctx context.Context,
	workspaceRef string,
	query NetworkUsageQuery,
) (NetworkUsageRecord, error) {
	base, err := networkBasePath(workspaceRef)
	if err != nil {
		return NetworkUsageRecord{}, err
	}
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.OwnerKind); trimmed != "" {
		values.Set("owner_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.OwnerID); trimmed != "" {
		values.Set("owner_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.RunID); trimmed != "" {
		values.Set("run_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Channel); trimmed != "" {
		values.Set("channel", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var response NetworkUsageRecord
	if err := c.doJSON(ctx, http.MethodGet, base+"/usage", values, nil, &response); err != nil {
		return NetworkUsageRecord{}, err
	}
	return response, nil
}

func networkCoordinationPath(workspaceRef string) (string, error) {
	workspaceRef, err := requireNetworkPathValue("workspace_id", workspaceRef)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceRef) + "/network-coordination", nil
}
