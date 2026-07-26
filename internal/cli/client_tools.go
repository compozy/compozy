package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	clientToolsAPIKeyKey = "api_key"
	clientToolsPromptKey = "prompt"
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
	partial := sanitizeToolResult(*e.response.Error.PartialResult)
	return &partial
}

func (c *unixSocketClient) ListTools(ctx context.Context, query ToolQuery) (ToolsResponseRecord, error) {
	var response ToolsResponseRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/tools", toolValues(query), nil, &response); err != nil {
		return ToolsResponseRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) SearchTools(
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

func (c *unixSocketClient) GetTool(
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

func (c *unixSocketClient) CreateToolApproval(
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

func (c *unixSocketClient) InvokeTool(
	ctx context.Context,
	id string,
	request ToolInvokeRequest,
) (ToolInvokeResponseRecord, error) {
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.AgentName = strings.TrimSpace(request.AgentName)
	request.ToolCallID = strings.TrimSpace(request.ToolCallID)
	request.TurnID = strings.TrimSpace(request.TurnID)
	request.CorrelationID = strings.TrimSpace(request.CorrelationID)
	request.SensitiveInputFields = trimNonEmptyStrings(request.SensitiveInputFields)
	var response ToolInvokeResponseRecord
	path := "/api/tools/" + url.PathEscape(strings.TrimSpace(id)) + "/invoke"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ToolInvokeResponseRecord{}, err
	}
	return sanitizeToolInvokeResponse(response), nil
}

func (c *unixSocketClient) ReadToolArtifact(
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

func (c *unixSocketClient) ListToolsets(ctx context.Context, query ToolQuery) (ToolsetsResponseRecord, error) {
	var response ToolsetsResponseRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/toolsets", toolValues(query), nil, &response); err != nil {
		return ToolsetsResponseRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetToolset(
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

func sanitizeToolErrorResponse(response ToolErrorResponseRecord) ToolErrorResponseRecord {
	response.Error.Message = redactToolDiagnostic(response.Error.Message)
	if len(response.Error.Details) > 0 {
		response.Error.Details = nil
	}
	if response.Error.PartialResult != nil {
		partial := sanitizeToolResult(*response.Error.PartialResult)
		response.Error.PartialResult = &partial
	}
	return response
}

func sanitizeToolInvokeResponse(response ToolInvokeResponseRecord) ToolInvokeResponseRecord {
	response.Result = sanitizeToolResult(response.Result)
	return response
}

func sanitizeToolResult(result toolspkg.ToolResult) toolspkg.ToolResult {
	result.Preview = redactToolDiagnostic(result.Preview)
	result.Structured = redactToolRawJSON(result.Structured)
	result.Metadata = redactToolMetadata(result.Metadata)
	for i := range result.Content {
		result.Content[i].Text = redactToolDiagnostic(result.Content[i].Text)
		result.Content[i].Data = redactToolRawJSON(result.Content[i].Data)
		result.Content[i].Metadata = redactToolMetadata(result.Content[i].Metadata)
	}
	return result
}

func redactToolRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		redacted := redactToolDiagnostic(string(raw))
		if !json.Valid([]byte(redacted)) {
			return raw
		}
		return json.RawMessage(redacted)
	}
	redacted := redactToolJSONValue(value)
	payload, err := json.Marshal(redacted)
	if err != nil {
		return raw
	}
	return json.RawMessage(payload)
}

func redactToolJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if sensitiveToolFieldName(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			typed[key] = redactToolJSONValue(item)
		}
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = redactToolJSONValue(item)
		}
		return typed
	case string:
		return redactToolDiagnostic(typed)
	default:
		return typed
	}
}

func redactToolMetadata(metadata map[string]json.RawMessage) map[string]json.RawMessage {
	if len(metadata) == 0 {
		return metadata
	}
	redacted := make(map[string]json.RawMessage, len(metadata))
	for key, value := range metadata {
		if sensitiveToolFieldName(key) {
			redacted[key] = json.RawMessage(`"[REDACTED]"`)
			continue
		}
		redacted[key] = redactToolRawJSON(value)
	}
	return redacted
}

func sensitiveToolFieldName(key string) bool {
	parts := sensitiveToolFieldNameParts(key)
	normalized := strings.Join(parts, "_")
	const tokenField = "token"
	for _, marker := range []string{
		clientToolsAPIKeyKey,
		"authorization",
		"password",
		"secret",
		"pkce",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	if len(parts) == 1 {
		return parts[0] == tokenField
	}
	if len(parts) == 0 || benignTokenMetric(parts) {
		return false
	}
	last := parts[len(parts)-1]
	return last == tokenField || last == "tokens"
}

func sensitiveToolFieldNameParts(key string) []string {
	runes := []rune(strings.TrimSpace(key))
	if len(runes) == 0 {
		return nil
	}
	var normalized strings.Builder
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			normalized.WriteRune('_')
			continue
		}
		if unicode.IsUpper(r) && i > 0 {
			previous := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				(unicode.IsUpper(previous) && next != 0 && unicode.IsLower(next)) {
				normalized.WriteRune('_')
			}
		}
		normalized.WriteRune(unicode.ToLower(r))
	}
	return strings.FieldsFunc(normalized.String(), func(r rune) bool {
		return r == '_'
	})
}

func benignTokenMetric(parts []string) bool {
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case "completion", clientToolsPromptKey, listTotalField:
		return parts[1] == "tokens"
	default:
		return false
	}
}

func redactToolDiagnostic(value string) string {
	redactedClaims := taskpkg.RedactClaimTokens(strings.TrimSpace(value))
	claimMarker := "agh_claim_" + "[REDACTED]"
	claimGuard := "__AGH_REDACTED_CLAIM_" + "TOKEN__"
	protectedClaims := strings.ReplaceAll(redactedClaims, claimMarker, claimGuard)
	redacted := diagnostics.Redact(protectedClaims)
	return strings.ReplaceAll(redacted, claimGuard, claimMarker)
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
