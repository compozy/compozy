package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const executorMCPKey = "mcp"

const toolResultIsErrorKey = "is_error"

// ListTools discovers tools from one configured MCP server.
func (e *CallExecutor) ListTools(
	ctx context.Context,
	source toolspkg.SourceRef,
) ([]toolspkg.MCPToolDescriptor, error) {
	discovery, err := e.DiscoverTools(ctx, source)
	return discovery.Descriptors, err
}

// DiscoverTools lists tools with the negotiated protocol and any nonfatal diagnostics.
func (e *CallExecutor) DiscoverTools(
	ctx context.Context,
	source toolspkg.SourceRef,
) (discovery ToolDiscovery, err error) {
	if e == nil {
		return ToolDiscovery{}, toolspkg.NewValidationError(
			"executor",
			toolspkg.ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if ctx == nil {
		return ToolDiscovery{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeCanceled,
			"",
			"mcp call context is required",
			toolspkg.ErrToolCanceled,
			toolspkg.ReasonCallCanceled,
		)
	}
	ctx, cancel := e.callContext(ctx)
	defer cancel()
	ctx = context.WithValue(ctx, mcpDiscoveryHTTPStatusKey{}, new(atomic.Int32))
	ctx, releaseRedactions := withMCPRequestRedactions(ctx)
	defer releaseRedactions()
	defer func() {
		err = redactMCPError(err)
		if reason, ok := toolspkg.ReasonOf(err); ok && reason == toolspkg.ReasonMCPUnreachable {
			slog.DebugContext(ctx, "mcp.discovery.failed", "server", redactMCPDiagnosticString(source.RawServerName),
				"reason", reason, "diagnostic", err.Error())
		}
	}()
	return e.listResolvedTools(ctx, source, toolspkg.BackgroundMCPDiscovery(ctx) && !toolspkg.FreshMCPDiscovery(ctx))
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
	resolved, err := e.resolveServer(ctx, source, "")
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := e.authorizeServer(ctx, source, &resolved); err != nil {
		return toolspkg.ToolResult{}, err
	}
	healthObservation := e.runtimeHealth.Begin(resolved.HealthKey)
	var runtimeHealthErr error
	defer func() {
		e.recordBearerRejection(ctx, &resolved, runtimeHealthErr)
		if runtimeHealthErr != nil {
			e.runtimeHealth.RecordFailure(healthObservation, runtimeHealthErr)
			return
		}
		e.runtimeHealth.RecordSuccess(healthObservation)
	}()
	authorizationHeader, err := e.prepareAuthorization(ctx, &resolved)
	if err != nil {
		runtimeHealthErr = err
		return toolspkg.ToolResult{}, err
	}
	client, err := e.openClient(ctx, &resolved, authorizationHeader)
	if err != nil {
		runtimeHealthErr = err
		return toolspkg.ToolResult{}, normalizeMCPErrorWithContext(ctx, req.ToolID, err, false)
	}
	defer func() {
		err = joinMCPClientCleanup(err, client)
	}()
	params, err := mcpCallParams(resolved.Server, negotiatedProtocolVersion(client.session), req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	mcpResult, err := client.session.CallTool(ctx, params)
	client.observeExchange(ctx, err)
	if err != nil {
		runtimeHealthErr = err
		return toolspkg.ToolResult{}, normalizeMCPErrorWithContext(ctx, req.ToolID, err, false)
	}
	if err := validateMCPToolResultCapabilities(req.ToolID, mcpResult); err != nil {
		runtimeHealthErr = err
		return toolspkg.ToolResult{}, runtimeHealthErr
	}
	if result, err = toolResultFromMCP(mcpResult); err != nil {
		runtimeHealthErr = err
		return toolspkg.ToolResult{}, runtimeHealthErr
	}
	return redactMCPToolResult(result)
}

func normalizeMCPErrorWithContext(ctx context.Context, id toolspkg.ToolID, err error, discovery bool) error {
	if errors.Is(err, mcpauth.ErrTokenRejected) {
		return rejectedBearerError(id)
	}
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
	if errors.Is(err, errRemoteAuthenticationRejected) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, errRemoteAuthenticationRejected.Error(),
			errRemoteAuthenticationRejected, toolspkg.ReasonMCPAuthUnconfigured,
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
		status := 0
		if observed := mcpDiscoveryHTTPStatus(ctx); observed != nil {
			status = int(observed.Load())
		}
		return normalizeMCPDiscoveryError(err, status)
	}
	return normalizeMCPError(id, err)
}

// Status returns token-redacted auth diagnostics for registry availability.
func (e *CallExecutor) Status(
	ctx context.Context,
	source toolspkg.SourceRef, definitionOwner string,
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
	resolved, err := e.resolveServer(ctx, source, definitionOwner)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, err
	}
	status, err = e.authStatus(ctx, &resolved)
	status.Disabled = resolved.Server.IsDisabled()
	return status, err
}

func (e *CallExecutor) authorizeServer(ctx context.Context, source toolspkg.SourceRef, resolved *ResolvedServer) error {
	if resolved.Server.IsDisabled() {
		return mcpServerDisabledError(resolved.Server.Name)
	}
	if e.consent != nil {
		return e.consent(ctx, source, *resolved)
	}
	return nil
}

func mcpServerDisabledError(name string) error {
	return toolspkg.NewToolError(
		"source_disabled", "", fmt.Sprintf("MCP server %q is off", name),
		toolspkg.ErrToolUnavailable, toolspkg.ReasonSourceDisabled,
	)
}

func (e *CallExecutor) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, e.timeout)
}

func (e *CallExecutor) resolveServer(
	ctx context.Context,
	source toolspkg.SourceRef, definitionOwner string,
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
	resolved, err := e.servers.ResolveMCPServer(ctx, source, definitionOwner)
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

func mcpCallParams(
	server compozyconfig.MCPServer,
	protocolVersion string,
	req toolspkg.MCPToolCallRequest,
) (*mcpsdk.CallToolParams, error) {
	if err := validateMCPTransportToolName(server, protocolVersion, req.RawToolName); err != nil {
		return nil, err
	}
	arguments, err := decodeArguments(req.Input)
	if err != nil {
		return nil, err
	}
	return &mcpsdk.CallToolParams{Name: req.RawToolName, Arguments: arguments}, nil
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

func normalizeMCPDiscoveryError(err error, status int) error {
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
		diagnostics.RedactAndBound(redactMCPDiagnosticString(mcpDiscoveryDiagnostic(err, status)), 512),
		fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, normalized),
		toolspkg.ReasonMCPUnreachable,
	)
}

func mcpDiscoveryDiagnostic(err error, status int) string {
	prefix := "mcp: upstream"
	if status != 0 {
		prefix = fmt.Sprintf("%s HTTP %d %s", prefix, status, http.StatusText(status))
	}
	if rpcErr, ok := errors.AsType[*jsonrpc.Error](err); ok {
		return fmt.Sprintf("%s JSON-RPC error %d: %s", prefix, rpcErr.Code, rpcErr.Message)
	}
	if status != 0 {
		return prefix
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return "mcp: upstream network error: " + err.Error()
	}
	return "mcp: upstream discovery failed: " + err.Error()
}

func mcpServerMatches(server compozyconfig.MCPServer, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	if strings.TrimSpace(server.Name) == target || server.EffectiveRuntimeName() == target {
		return true
	}
	id, err := toolspkg.Canonicalize(server.EffectiveRuntimeName(), "tool")
	if err != nil {
		return false
	}
	owner, err := toolspkg.MCPToolOwner(id)
	return err == nil && owner == target
}

func cloneMCPServer(server compozyconfig.MCPServer) compozyconfig.MCPServer {
	server.Args = append([]string(nil), server.Args...)
	server.Env = cloneStringMap(server.Env)
	server.InputValues = maps.Clone(server.InputValues)
	server.SecretEnv = cloneStringMap(server.SecretEnv)
	server.Headers = cloneStringMap(server.Headers)
	server.SecretHeaders = cloneStringMap(server.SecretHeaders)
	server.Auth.Scopes = append([]string(nil), server.Auth.Scopes...)
	return server
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
}

func cloneRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return slices.Clone(src)
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
