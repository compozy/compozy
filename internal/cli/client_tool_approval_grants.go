package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// ToolApprovalGrantSetRequest creates or replaces one explicit wider decision.
type ToolApprovalGrantSetRequest = contract.ToolApprovalGrantSetRequest

// ToolApprovalGrantRecord is one remembered native-tool approval decision.
type ToolApprovalGrantRecord = contract.ToolApprovalGrantPayload

// ToolApprovalGrantListRecord is one workspace's remembered native-tool decisions.
type ToolApprovalGrantListRecord = contract.ToolApprovalGrantListResponse

func (c *unixSocketClient) SetToolApprovalGrant(
	ctx context.Context,
	workspaceID string,
	request ToolApprovalGrantSetRequest,
) (ToolApprovalGrantRecord, error) {
	request.AgentName = strings.TrimSpace(request.AgentName)
	values := url.Values{taskWorkspaceIDKey: []string{strings.TrimSpace(workspaceID)}}
	var response contract.ToolApprovalGrantResponse
	if err := c.doJSON(
		ctx,
		http.MethodPut,
		"/api/tool-approval-grants",
		values,
		request,
		&response,
	); err != nil {
		return ToolApprovalGrantRecord{}, err
	}
	return response.Grant, nil
}

func (c *unixSocketClient) ListToolApprovalGrants(
	ctx context.Context,
	workspaceID string,
) (ToolApprovalGrantListRecord, error) {
	var response ToolApprovalGrantListRecord
	values := url.Values{taskWorkspaceIDKey: []string{strings.TrimSpace(workspaceID)}}
	if err := c.doJSON(ctx, http.MethodGet, "/api/tool-approval-grants", values, nil, &response); err != nil {
		return ToolApprovalGrantListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RevokeToolApprovalGrant(ctx context.Context, workspaceID string, id string) error {
	values := url.Values{taskWorkspaceIDKey: []string{strings.TrimSpace(workspaceID)}}
	path := "/api/tool-approval-grants/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, values, nil, nil)
}
