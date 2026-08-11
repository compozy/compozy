package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const executorMCPKey = "mcp"

const toolResultIsErrorKey = "is_error"

// ListTools discovers tools from one configured MCP server.
func (e *CallExecutor) ListTools(
	ctx context.Context,
	source toolspkg.SourceRef,
) ([]toolspkg.MCPToolDescriptor, error) {
	descriptors, _, err := e.ListToolsWithProtocol(ctx, source)
	return descriptors, err
}

// ListToolsWithProtocol discovers tools and returns the protocol version negotiated with the server.
func (e *CallExecutor) ListToolsWithProtocol(
	ctx context.Context,
	source toolspkg.SourceRef,
) (descriptors []toolspkg.MCPToolDescriptor, protocolVersion string, err error) {
	if e == nil {
		return nil, "", toolspkg.NewValidationError(
			"executor",
			toolspkg.ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if ctx == nil {
		return nil, "", toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			"",
			"mcp call context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	ctx, cancel := e.callContext(ctx)
	defer cancel()
	ctx, releaseRedactions := withMCPRequestRedactions(ctx)
	defer releaseRedactions()
	defer func() {
		err = redactMCPError(err)
	}()
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return nil, "", err
	}
	if err := e.ensureAuthorized(ctx, resolved); err != nil {
		return nil, "", err
	}
	client, err := e.openClient(ctx, resolved)
	if err != nil {
		return nil, "", normalizeMCPErrorWithContext(ctx, "", err, true)
	}
	defer func() {
		err = joinMCPClientCleanup(err, client)
	}()
	protocolVersion = negotiatedProtocolVersion(client.session)
	cacheKey, err := e.toolCacheKey(ctx, resolved, protocolVersion)
	if err != nil {
		return nil, "", err
	}
	if descriptors, ok := e.cachedTools(cacheKey, time.Now()); ok {
		return descriptors, protocolVersion, nil
	}
	tools, ttlMs, err := listMCPTools(ctx, client.session)
	if err != nil {
		return nil, "", normalizeMCPErrorWithContext(ctx, "", err, true)
	}
	descriptors = make([]toolspkg.MCPToolDescriptor, 0, len(tools))
	for i := range tools {
		if tools[i] == nil {
			return nil, "", toolspkg.NewValidationError(
				"tools",
				toolspkg.ReasonSchemaInvalid,
				"mcp server returned an empty tool descriptor",
			)
		}
		descriptor, err := e.descriptorFromTool(source, resolved.Server, *tools[i])
		if err != nil {
			return nil, "", fmt.Errorf("mcp: normalize tool %q: %w", tools[i].Name, err)
		}
		descriptors = append(descriptors, descriptor)
	}
	e.cacheTools(cacheKey, descriptors, ttlMs, time.Now())
	if err := e.recordToolProjection(source, descriptors, ttlMs, time.Now()); err != nil {
		return nil, "", err
	}
	return descriptors, protocolVersion, nil
}

// CallTool invokes one configured MCP tool.
func (e *CallExecutor) CallTool(
	ctx context.Context,
	source toolspkg.SourceRef,
	req toolspkg.MCPToolCallRequest,
) (result toolspkg.ToolResult, err error) {
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
	ctx, releaseRedactions := withMCPRequestRedactions(ctx)
	defer releaseRedactions()
	defer func() {
		err = redactMCPError(err)
	}()
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := e.ensureAuthorized(ctx, resolved); err != nil {
		return toolspkg.ToolResult{}, err
	}
	client, err := e.openClient(ctx, resolved)
	if err != nil {
		return toolspkg.ToolResult{}, normalizeMCPErrorWithContext(ctx, req.ToolID, err, false)
	}
	defer func() {
		err = joinMCPClientCleanup(err, client)
	}()
	arguments, err := decodeArguments(req.Input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	mcpResult, err := client.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      strings.TrimSpace(req.RawToolName),
		Arguments: arguments,
	})
	if err != nil {
		return toolspkg.ToolResult{}, normalizeMCPErrorWithContext(ctx, req.ToolID, err, false)
	}
	if mcpResult.NeedsInput() {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"mcp server requires an unsupported client capability",
			&UnsupportedCapabilityError{Capability: "input_requests"},
			toolspkg.ReasonMCPUnreachable,
		)
	}
	result, err = toolResultFromMCP(mcpResult)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return redactMCPToolResult(result)
}

func normalizeMCPErrorWithContext(ctx context.Context, id toolspkg.ToolID, err error, discovery bool) error {
	if scopeErr, ok := errors.AsType[*InsufficientScopeError](err); ok {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			scopeErr.Error(),
			scopeErr,
			toolspkg.ReasonMCPAuthRequired,
		)
	}
	if errors.Is(err, ErrAuthorizationRequired) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			ErrAuthorizationRequired.Error(),
			ErrAuthorizationRequired,
			toolspkg.ReasonMCPAuthRequired,
		)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	} else if deadline, hasDeadline := ctx.Deadline(); hasDeadline && !time.Now().Before(deadline) {
		err = context.DeadlineExceeded
	} else {
		if networkError, ok := errors.AsType[net.Error](err); ok && networkError != nil && networkError.Timeout() {
			err = context.DeadlineExceeded
		}
	}
	if discovery {
		return normalizeMCPDiscoveryError(err)
	}
	return normalizeMCPError(id, err)
}

// Status returns token-redacted auth diagnostics for registry availability.
func (e *CallExecutor) Status(
	ctx context.Context,
	source toolspkg.SourceRef,
) (status toolspkg.MCPAuthStatus, err error) {
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
	ctx, releaseRedactions := withMCPRequestRedactions(ctx)
	defer releaseRedactions()
	defer func() {
		err = redactMCPError(err)
	}()
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

func mcpServerMatches(server compozyconfig.MCPServer, target string) bool {
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

func cloneMCPServer(server compozyconfig.MCPServer) compozyconfig.MCPServer {
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
