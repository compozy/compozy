package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	toolspkg "github.com/compozy/agh/internal/tools"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

const executorMCPKey = "mcp"

const toolResultIsErrorKey = "is_error"

// ListTools discovers tools from one configured MCP server.
func (e *CallExecutor) ListTools(
	ctx context.Context,
	source toolspkg.SourceRef,
) ([]toolspkg.MCPToolDescriptor, error) {
	if e == nil {
		return nil, toolspkg.NewValidationError(
			"executor",
			toolspkg.ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if ctx == nil {
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			"",
			"mcp call context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	ctx, cancel := e.callContext(ctx)
	defer cancel()
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return nil, err
	}
	if err := e.ensureAuthorized(ctx, resolved); err != nil {
		return nil, err
	}
	client, err := e.openClient(ctx, resolved)
	if err != nil {
		return nil, normalizeMCPDiscoveryError(err)
	}
	defer closeMCPClient(client)
	if err := initializeClient(ctx, client); err != nil {
		return nil, normalizeMCPDiscoveryError(err)
	}
	result, err := client.ListTools(ctx, mcpsdk.ListToolsRequest{})
	if err != nil {
		return nil, normalizeMCPDiscoveryError(err)
	}
	descriptors := make([]toolspkg.MCPToolDescriptor, 0, len(result.Tools))
	for i := range result.Tools {
		descriptor, err := e.descriptorFromTool(source, resolved.Server, result.Tools[i])
		if err != nil {
			return nil, fmt.Errorf("mcp: normalize tool %q: %w", result.Tools[i].Name, err)
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

// CallTool invokes one configured MCP tool.
func (e *CallExecutor) CallTool(
	ctx context.Context,
	source toolspkg.SourceRef,
	req toolspkg.MCPToolCallRequest,
) (toolspkg.ToolResult, error) {
	if e == nil {
		return toolspkg.ToolResult{}, toolspkg.NewValidationError(
			"executor",
			toolspkg.ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if ctx == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			req.ToolID,
			"mcp call context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	ctx, cancel := e.callContext(ctx)
	defer cancel()
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := e.ensureAuthorized(ctx, resolved); err != nil {
		return toolspkg.ToolResult{}, err
	}
	client, err := e.openClient(ctx, resolved)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	defer closeMCPClient(client)
	if err := initializeClient(ctx, client); err != nil {
		return toolspkg.ToolResult{}, normalizeMCPError(req.ToolID, err)
	}
	arguments, err := decodeArguments(req.Input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := client.CallTool(ctx, mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name:      strings.TrimSpace(req.RawToolName),
			Arguments: arguments,
		},
	})
	if err != nil {
		return toolspkg.ToolResult{}, normalizeMCPError(req.ToolID, err)
	}
	return toolResultFromMCP(result)
}

// Status returns token-redacted auth diagnostics for registry availability.
func (e *CallExecutor) Status(
	ctx context.Context,
	source toolspkg.SourceRef,
) (toolspkg.MCPAuthStatus, error) {
	if e == nil {
		return toolspkg.MCPAuthStatus{}, toolspkg.NewValidationError(
			"executor",
			toolspkg.ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if ctx == nil {
		return toolspkg.MCPAuthStatus{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			"",
			"mcp status context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, err
	}
	return e.authStatus(ctx, resolved)
}

func (e *CallExecutor) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, e.timeout)
}

func (e *CallExecutor) resolveServer(
	ctx context.Context,
	source toolspkg.SourceRef,
) (ResolvedServer, error) {
	if ctx == nil {
		return ResolvedServer{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			"",
			"mcp server resolution context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	resolved, err := e.servers.ResolveMCPServer(ctx, source)
	if err != nil {
		return ResolvedServer{}, fmt.Errorf("mcp: resolve configured server: %w", err)
	}
	target := strings.TrimSpace(firstNonEmpty(source.RawServerName, source.Owner))
	if !mcpServerMatches(resolved.Server, target) && !mcpServerMatches(resolved.Server, source.Owner) {
		return ResolvedServer{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			"",
			fmt.Sprintf("mcp server %q is unavailable", target),
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonMCPUnreachable,
		)
	}
	resolved.Server = cloneMCPServer(resolved.Server)
	resolved.Target = resolved.Target.Normalize()
	if err := resolved.Target.Validate(); err != nil {
		return ResolvedServer{}, fmt.Errorf("mcp: validate resolved auth target: %w", err)
	}
	if resolved.Target.ServerName != strings.TrimSpace(resolved.Server.Name) {
		return ResolvedServer{}, errors.New("mcp: resolved auth target does not match server config")
	}
	return resolved, nil
}

func toolResultFromMCP(result *mcpsdk.CallToolResult) (toolspkg.ToolResult, error) {
	if result == nil {
		return toolspkg.ToolResult{}, nil
	}
	content := make([]toolspkg.ToolContent, 0, len(result.Content))
	for i := range result.Content {
		converted, err := toolContentFromMCP(result.Content[i])
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		content = append(content, converted)
	}
	var structured json.RawMessage
	if result.StructuredContent != nil {
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return toolspkg.ToolResult{}, fmt.Errorf("mcp: encode structured content: %w", err)
		}
		structured = data
	}
	metadata := map[string]json.RawMessage{}
	if result.IsError {
		metadata[toolResultIsErrorKey] = json.RawMessage(`true`)
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return toolspkg.ToolResult{
		Content:    content,
		Structured: structured,
		Preview:    mcpPreview(content, result.IsError),
		Metadata:   metadata,
	}, nil
}

func toolContentFromMCP(content mcpsdk.Content) (toolspkg.ToolContent, error) {
	switch typed := content.(type) {
	case mcpsdk.TextContent:
		return toolspkg.ToolContent{Type: typed.Type, Text: typed.Text}, nil
	case mcpsdk.ImageContent:
		data, err := json.Marshal(typed.Data)
		if err != nil {
			return toolspkg.ToolContent{}, fmt.Errorf("mcp: encode image content: %w", err)
		}
		return toolspkg.ToolContent{Type: typed.Type, Data: data, MIMEType: typed.MIMEType}, nil
	case mcpsdk.AudioContent:
		data, err := json.Marshal(typed.Data)
		if err != nil {
			return toolspkg.ToolContent{}, fmt.Errorf("mcp: encode audio content: %w", err)
		}
		return toolspkg.ToolContent{Type: typed.Type, Data: data, MIMEType: typed.MIMEType}, nil
	default:
		data, err := json.Marshal(content)
		if err != nil {
			return toolspkg.ToolContent{}, fmt.Errorf("mcp: encode content block: %w", err)
		}
		return toolspkg.ToolContent{Type: executorMCPKey, Data: data}, nil
	}
}

func mcpPreview(content []toolspkg.ToolContent, isError bool) string {
	prefix := ""
	if isError {
		prefix = "error: "
	}
	for _, item := range content {
		if strings.TrimSpace(item.Text) != "" {
			return prefix + strings.TrimSpace(item.Text)
		}
	}
	if len(content) == 0 {
		return prefix + "empty MCP result"
	}
	return fmt.Sprintf("%s%d MCP content blocks", prefix, len(content))
}

func decodeArguments(raw json.RawMessage) (any, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return map[string]any{}, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			"",
			"mcp tool input is invalid JSON",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return value, nil
}

func normalizeMCPError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			id,
			"mcp call canceled",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	case errors.Is(err, context.DeadlineExceeded):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeTimedOut,
			id,
			"mcp call timed out",
			toolspkg.ErrToolTimedOut,
			toolspkg.ReasonCallTimedOut,
		)
	case errors.Is(err, mcptransport.ErrAuthorizationRequired):
		return unavailableAuthError(toolspkg.ReasonMCPAuthRequired)
	default:
		return fmt.Errorf("mcp: call upstream server: %w", err)
	}
}

func normalizeMCPDiscoveryError(err error) error {
	normalized := normalizeMCPError("", err)
	if normalized == nil {
		return nil
	}
	if _, ok := toolspkg.ReasonOf(normalized); ok {
		return normalized
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		"",
		"mcp server is unreachable",
		fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, normalized),
		toolspkg.ReasonMCPUnreachable,
	)
}

func mcpServerMatches(server aghconfig.MCPServer, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	if strings.TrimSpace(server.Name) == target {
		return true
	}
	id, err := toolspkg.Canonicalize(server.Name, "tool")
	if err != nil {
		return false
	}
	owner, err := mcpOwner(id)
	return err == nil && owner == target
}

func mcpOwner(id toolspkg.ToolID) (string, error) {
	segments, err := id.Segments()
	if err != nil {
		return "", err
	}
	if len(segments) != 3 {
		return "", toolspkg.NewValidationError(
			"tool_id",
			toolspkg.ReasonIDInvalidFormat,
			"mcp tool id must contain namespace server and tool segments",
		)
	}
	return segments[1], nil
}

func cloneMCPServer(server aghconfig.MCPServer) aghconfig.MCPServer {
	server.Args = append([]string(nil), server.Args...)
	server.Env = cloneStringMap(server.Env)
	server.SecretEnv = cloneStringMap(server.SecretEnv)
	server.Auth.Scopes = append([]string(nil), server.Auth.Scopes...)
	return server
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func cloneRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

func trimStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
