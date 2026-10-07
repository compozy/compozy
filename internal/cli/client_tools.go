package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// ToolRecord is the shared tool registry projection payload.
type ToolRecord = contract.ToolPayload

// ToolsResponseRecord is the shared tool registry list/search response.
type ToolsResponseRecord = contract.ToolsResponse

// ToolResponseRecord is the shared single-tool registry response.
type ToolResponseRecord = contract.ToolResponse

// ToolSearchRequest captures the shared registry search request.
type ToolSearchRequest = contract.ToolSearchRequest

// ToolInvokeRequest captures the shared registry invoke request.
type ToolInvokeRequest = contract.ToolInvokeRequest

// ToolInvokeResponseRecord is the shared registry invoke response.
type ToolInvokeResponseRecord = contract.ToolInvokeResponse

// ToolArtifactPageRecord is one exact page from a retained oversized tool result.
type ToolArtifactPageRecord = contract.ToolArtifactPageResponse

// ToolsetRecord is the shared toolset projection payload.
type ToolsetRecord = contract.ToolsetPayload

// ToolsetsResponseRecord is the shared toolset list response.
type ToolsetsResponseRecord = contract.ToolsetsResponse

// ToolsetResponseRecord is the shared single-toolset response.
type ToolsetResponseRecord = contract.ToolsetResponse

// ToolErrorResponseRecord is the shared structured tool error response.
type ToolErrorResponseRecord = contract.ToolErrorResponse

// ToolApprovalRequest captures one local approval-token mint request.
type ToolApprovalRequest = contract.ToolApprovalRequest

// ToolApprovalRecord is the shared tool approval payload.
type ToolApprovalRecord = contract.ToolApprovalPayload

// ToolQuery captures operator scope filters for registry and toolset commands.
type ToolQuery struct {
	WorkspaceID string
	SessionID   string
	AgentName   string
}

// ToolClient is the CLI transport surface for registry tools and retained results.
type ToolClient interface {
	ListTools(ctx context.Context, query ToolQuery) (ToolsResponseRecord, error)
	SearchTools(ctx context.Context, request ToolSearchRequest) (ToolsResponseRecord, error)
	GetTool(ctx context.Context, id string, query ToolQuery) (ToolResponseRecord, error)
	CreateToolApproval(ctx context.Context, id string, request ToolApprovalRequest) (ToolApprovalRecord, error)
	SetToolApprovalGrant(
		ctx context.Context,
		workspaceID string,
		request ToolApprovalGrantSetRequest,
	) (ToolApprovalGrantRecord, error)
	ListToolApprovalGrants(ctx context.Context, workspaceID string) (ToolApprovalGrantListRecord, error)
	RevokeToolApprovalGrant(ctx context.Context, workspaceID string, id string) error
	InvokeTool(ctx context.Context, id string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error)
	ReadToolArtifact(
		ctx context.Context,
		workspaceID string,
		artifactURI string,
		offset int64,
		limit int64,
	) (ToolArtifactPageRecord, error)
	ListToolsets(ctx context.Context, query ToolQuery) (ToolsetsResponseRecord, error)
	GetToolset(ctx context.Context, id string, query ToolQuery) (ToolsetResponseRecord, error)
}

type toolAPIError struct {
	statusCode int
	status     string
	response   ToolErrorResponseRecord
}

const nilToolErrorString = "<nil>"

func newToolAPIError(statusCode int, status string, response ToolErrorResponseRecord) *toolAPIError {
	return &toolAPIError{
		statusCode: statusCode,
		status:     strings.TrimSpace(status),
		response:   sanitizeToolErrorResponse(response),
	}
}

func (e *toolAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	payload := e.response.Error
	code := strings.TrimSpace(string(payload.Code))
	message := strings.TrimSpace(payload.Message)
	if code == "" {
		code = "tool_error"
	}
	if message == "" {
		message = strings.TrimSpace(e.status)
	}
	if message == "" && e.statusCode > 0 {
		message = fmt.Sprintf("HTTP %d", e.statusCode)
	}
	message = redactToolDiagnostic(message)
	return code + ": " + message
}

func (e *toolAPIError) Response() ToolErrorResponseRecord {
	if e == nil {
		return ToolErrorResponseRecord{}
	}
	return sanitizeToolErrorResponse(e.response)
}

// PartialToolResult exposes the safe bounded result to transport adapters such as hosted MCP.
func (e *toolAPIError) PartialToolResult() *toolspkg.ToolResult {
	if e == nil || e.response.Error.PartialResult == nil {
		return nil
	}
	return new(sanitizeToolResult(*e.response.Error.PartialResult))
}

func (c *daemonClient) ListTools(ctx context.Context, query ToolQuery) (ToolsResponseRecord, error) {
	var response ToolsResponseRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/tools", toolValues(query), nil, &response); err != nil {
		return ToolsResponseRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) SearchTools(
	ctx context.Context,
	request ToolSearchRequest,
) (ToolsResponseRecord, error) {
	request.Query = strings.TrimSpace(request.Query)
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.AgentName = strings.TrimSpace(request.AgentName)
	var response ToolsResponseRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/tools/search", nil, request, &response); err != nil {
		return ToolsResponseRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetTool(
	ctx context.Context,
	id string,
	query ToolQuery,
) (ToolResponseRecord, error) {
	var response ToolResponseRecord
	path := "/api/tools/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, toolValues(query), nil, &response); err != nil {
		return ToolResponseRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) CreateToolApproval(
	ctx context.Context,
	id string,
	request ToolApprovalRequest,
) (ToolApprovalRecord, error) {
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.AgentName = strings.TrimSpace(request.AgentName)
	request.InputDigest = strings.TrimSpace(request.InputDigest)
	var response contract.ToolApprovalResponse
	path := "/api/tools/" + url.PathEscape(strings.TrimSpace(id)) + "/approvals"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ToolApprovalRecord{}, err
	}
	return response.Approval, nil
}

func (c *daemonClient) InvokeTool(
	ctx context.Context,
	id string,
	request ToolInvokeRequest,
) (responseRecord ToolInvokeResponseRecord, err error) {
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.AgentName = strings.TrimSpace(request.AgentName)
	request.ToolCallID = strings.TrimSpace(request.ToolCallID)
	request.TurnID = strings.TrimSpace(request.TurnID)
	request.CorrelationID = strings.TrimSpace(request.CorrelationID)
	request.SensitiveInputFields = trimNonEmptyStrings(request.SensitiveInputFields)
	path := "/api/tools/" + url.PathEscape(strings.TrimSpace(id)) + "/invoke"
	response, err := c.doRequest(ctx, http.MethodPost, path, request)
	if err != nil {
		return ToolInvokeResponseRecord{}, err
	}
	defer mergeResponseBodyCloseError(&err, response, http.MethodPost, path)
	if response.StatusCode == http.StatusAccepted {
		return ToolInvokeResponseRecord{}, readAPIError(response)
	}
	if err := c.decodeJSONResponse(ctx, http.MethodPost, path, response, &responseRecord); err != nil {
		return ToolInvokeResponseRecord{}, err
	}
	return sanitizeToolInvokeResponse(responseRecord), nil
}

func (c *daemonClient) ReadToolArtifact(
	ctx context.Context,
	workspaceID string,
	artifactURI string,
	offset int64,
	limit int64,
) (ToolArtifactPageRecord, error) {
	artifactID, err := toolspkg.ParseToolArtifactURI(artifactURI)
	if err != nil {
		return ToolArtifactPageRecord{}, fmt.Errorf("cli: parse tool artifact URI: %w", err)
	}
	values := url.Values{}
	if offset != 0 {
		values.Set("offset", fmt.Sprintf("%d", offset))
	}
	if limit != 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/workspaces/" + url.PathEscape(strings.TrimSpace(workspaceID)) +
		"/tool-artifacts/" + url.PathEscape(artifactID)
	var response ToolArtifactPageRecord
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return ToolArtifactPageRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ListToolsets(ctx context.Context, query ToolQuery) (ToolsetsResponseRecord, error) {
	var response ToolsetsResponseRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/toolsets", toolValues(query), nil, &response); err != nil {
		return ToolsetsResponseRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetToolset(
	ctx context.Context,
	id string,
	query ToolQuery,
) (ToolsetResponseRecord, error) {
	var response ToolsetResponseRecord
	path := "/api/toolsets/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, toolValues(query), nil, &response); err != nil {
		return ToolsetResponseRecord{}, err
	}
	return response, nil
}

func toolValues(query ToolQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.SessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.AgentName); trimmed != "" {
		values.Set("agent_name", trimmed)
	}
	return values
}

func trimNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
