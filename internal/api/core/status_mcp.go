package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	settingspkg "github.com/compozy/agh/internal/settings"
)

func (h *BaseHandlers) mcpServerStatusPayloads(
	ctx context.Context,
	workspaceID string,
) ([]contract.MCPServerStatusPayload, error) {
	if h.Settings == nil {
		return nil, nil
	}
	request := settingspkg.CollectionRequest{
		Collection: settingspkg.CollectionMCPServers,
		Scope:      settingspkg.ScopeGlobal,
	}
	if workspaceID = strings.TrimSpace(workspaceID); workspaceID != "" {
		request.Scope = settingspkg.ScopeWorkspace
		request.WorkspaceID = workspaceID
	}
	envelope, err := h.Settings.ListCollection(ctx, request)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.MCPServerStatusPayload, 0, len(envelope.MCPServers))
	for _, server := range envelope.MCPServers {
		payloads = append(payloads, mcpServerStatusPayload(server))
	}
	return payloads, nil
}

func mcpServerStatusPayload(server settingspkg.MCPServerItem) contract.MCPServerStatusPayload {
	payload := contract.MCPServerStatusPayload{
		Name:          strings.TrimSpace(server.Name),
		Scope:         strings.TrimSpace(string(server.Scope)),
		WorkspaceID:   strings.TrimSpace(server.WorkspaceID),
		Transport:     strings.TrimSpace(string(server.Transport)),
		RuntimeStatus: statusStateConfigured,
	}
	if server.AuthStatus != nil {
		payload.AuthStatus = strings.TrimSpace(string(server.AuthStatus.Status))
	}
	if server.RuntimeStatus == nil {
		return payload
	}
	runtimeStatus := *server.RuntimeStatus
	payload.Configured = runtimeStatus.Configured
	payload.Initialized = runtimeStatus.Initialized
	payload.State = strings.TrimSpace(string(runtimeStatus.State))
	payload.Probe = strings.TrimSpace(string(runtimeStatus.Probe))
	payload.ToolCount = runtimeStatus.ToolCount
	payload.Reason = diagnostics.RedactAndBound(runtimeStatus.Reason, maxDiagnosticPayloadBytes)
	payload.Diagnostic = diagnostics.RedactAndBound(runtimeStatus.Diagnostic, maxDiagnosticPayloadBytes)
	payload.RuntimeStatus = mcpRuntimeStatus(runtimeStatus.State)
	return payload
}

func mcpRuntimeStatus(state settingspkg.MCPServerRuntimeState) string {
	switch state {
	case settingspkg.MCPServerRuntimeStateReady:
		return statusStateRunning
	case settingspkg.MCPServerRuntimeStateAuthRequired,
		settingspkg.MCPServerRuntimeStateAuthExpired,
		settingspkg.MCPServerRuntimeStateAuthInvalid,
		settingspkg.MCPServerRuntimeStateAuthRefreshFailed:
		return "auth_required"
	case settingspkg.MCPServerRuntimeStateConfigError,
		settingspkg.MCPServerRuntimeStatePermissionDenied,
		settingspkg.MCPServerRuntimeStateRuntimeUnavailable,
		settingspkg.MCPServerRuntimeStateDead:
		return memoryHealthStatusUnavailable
	default:
		return statusStateConfigured
	}
}

func mcpServerDiagnosticItem(status contract.MCPServerStatusPayload) contract.DiagnosticItem {
	severity, code := mcpServerDiagnosticSeverityAndCode(status)
	title := "MCP server is ready"
	if severity != contract.SeverityOK {
		title = "MCP server needs attention"
	}
	message := fmt.Sprintf("MCP server %q runtime status is %q.", status.Name, status.RuntimeStatus)
	if strings.TrimSpace(status.Diagnostic) != "" {
		message = status.Diagnostic
	}
	return diagnostics.NewItem(
		"doctor.mcp."+status.Name,
		code,
		contract.CategoryMCP,
		title,
		message,
		severity,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"server": status.Name,
			"state":  status.State,
			"probe":  status.Probe,
		}),
	)
}

func mcpServerDiagnosticSeverityAndCode(status contract.MCPServerStatusPayload) (string, string) {
	switch strings.TrimSpace(status.RuntimeStatus) {
	case "running":
		return contract.SeverityOK, contract.CodeMCPServerReady
	case "auth_required":
		return contract.SeverityWarn, contract.CodeMCPAuthRequired
	case "unavailable":
		return contract.SeverityError, contract.CodeMCPServerUnavailable
	default:
		return contract.SeverityInfo, contract.CodeMCPServerUnavailable
	}
}
