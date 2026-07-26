package daemon

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	settingspkg "github.com/compozy/agh/internal/settings"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeMCPAuthToolsAPISettingsMCPServersPath = "/api/settings/mcp-servers"
	nativeMCPCallableDiscoveryNote              = "Auth-blocked MCP tools are omitted from callable discovery; " +
		"use agh__mcp_status or agh__mcp_auth_status for repair detail."
	nativeMCPStateHealthy     = "healthy"
	nativeMCPStateAuthBlocked = "auth-blocked"
	nativeMCPStateUnavailable = "unavailable"
)

type mcpAuthStatusInput struct {
	ServerName string `json:"server_name"`
}

type mcpAuthStatusPayload struct {
	Status      toolspkg.MCPAuthStatus `json:"status"`
	RepairPaths mcpAuthRepairPaths     `json:"repair_paths"`
}

type mcpStatusPayload struct {
	ServerName            string                              `json:"server_name"`
	State                 string                              `json:"state"`
	Auth                  toolspkg.MCPAuthStatus              `json:"auth"`
	Runtime               *settingspkg.MCPServerRuntimeStatus `json:"runtime,omitempty"`
	RepairPaths           mcpAuthRepairPaths                  `json:"repair_paths"`
	CallableDiscoveryNote string                              `json:"callable_discovery_note"`
}

type mcpAuthRepairPaths struct {
	StatusCLI    string `json:"status_cli"`
	LoginCLI     string `json:"login_cli"`
	LogoutCLI    string `json:"logout_cli"`
	SettingsHTTP string `json:"settings_http"`
	SettingsUDS  string `json:"settings_uds"`
	Note         string `json:"note"`
}

func (n *daemonNativeTools) mcpAuthToolBindings(
	statusAvailability toolspkg.NativeAvailabilityFunc,
	authAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDMCPStatus: {
			call:         n.mcpStatus,
			availability: statusAvailability,
		},
		toolspkg.ToolIDMCPAuthStatus: {
			call:         n.mcpAuthStatus,
			availability: authAvailability,
		},
	}
}

func (n *daemonNativeTools) mcpStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input mcpAuthStatusInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	serverName, err := requiredNativeString(req.ToolID, "server_name", input.ServerName)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	provider := n.mcpAuthProvider()
	if provider == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"mcp status provider is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	workspaceID := strings.TrimSpace(firstNonEmpty(scope.WorkspaceID, req.WorkspaceID))
	status, err := provider.Status(ctx, toolspkg.SourceRef{
		Kind:          toolspkg.SourceMCP,
		Owner:         serverName,
		RawServerName: serverName,
		WorkspaceID:   workspaceID,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(status.ServerName) == "" {
		status.ServerName = serverName
	}
	collectionRequest := settingspkg.CollectionRequest{
		Collection: settingspkg.CollectionMCPServers,
		Scope:      settingspkg.ScopeGlobal,
	}
	if workspaceID != "" {
		collectionRequest.Scope = settingspkg.ScopeWorkspace
		collectionRequest.WorkspaceID = workspaceID
	}
	settingsService := n.settingsService()
	if settingsService == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"mcp runtime status provider is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	envelope, err := settingsService.ListCollection(ctx, collectionRequest)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: list MCP runtime status: %w", err)
	}
	runtimeStatus, found := mcpRuntimeStatusByName(envelope.MCPServers, serverName)
	if !found {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			req.ToolID,
			fmt.Sprintf("mcp server %q is not configured", serverName),
			toolspkg.ErrToolNotFound,
			toolspkg.ReasonMCPUnreachable,
		)
	}
	state := mcpProbeState(status)
	if runtimeStatus != nil {
		state = mcpRuntimeProbeState(runtimeStatus.State, state)
	}
	payload := mcpStatusPayload{
		ServerName:            status.ServerName,
		State:                 state,
		Auth:                  status,
		Runtime:               runtimeStatus,
		RepairPaths:           mcpAuthRepairPathsFor(status.ServerName),
		CallableDiscoveryNote: nativeMCPCallableDiscoveryNote,
	}
	return structuredResult(payload, fmt.Sprintf("%s %s", status.ServerName, state))
}

func mcpRuntimeStatusByName(
	servers []settingspkg.MCPServerItem,
	serverName string,
) (*settingspkg.MCPServerRuntimeStatus, bool) {
	want := strings.TrimSpace(serverName)
	for _, server := range servers {
		if strings.TrimSpace(server.Name) != want {
			continue
		}
		if server.RuntimeStatus == nil {
			return nil, true
		}
		status := *server.RuntimeStatus
		return &status, true
	}
	return nil, false
}

func mcpRuntimeProbeState(state settingspkg.MCPServerRuntimeState, fallback string) string {
	switch state {
	case settingspkg.MCPServerRuntimeStateDead:
		return "dead"
	case settingspkg.MCPServerRuntimeStateReady:
		return nativeMCPStateHealthy
	case settingspkg.MCPServerRuntimeStateAuthRequired,
		settingspkg.MCPServerRuntimeStateAuthExpired,
		settingspkg.MCPServerRuntimeStateAuthInvalid,
		settingspkg.MCPServerRuntimeStateAuthRefreshFailed:
		return nativeMCPStateAuthBlocked
	case settingspkg.MCPServerRuntimeStateConfigError,
		settingspkg.MCPServerRuntimeStatePermissionDenied,
		settingspkg.MCPServerRuntimeStateRuntimeUnavailable:
		return nativeMCPStateUnavailable
	default:
		return fallback
	}
}

func (n *daemonNativeTools) mcpAuthStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input mcpAuthStatusInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	serverName, err := requiredNativeString(req.ToolID, "server_name", input.ServerName)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	provider := n.mcpAuthProvider()
	if provider == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"mcp auth status provider is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	status, err := provider.Status(ctx, toolspkg.SourceRef{
		Kind:          toolspkg.SourceMCP,
		Owner:         serverName,
		RawServerName: serverName,
		WorkspaceID:   strings.TrimSpace(firstNonEmpty(scope.WorkspaceID, req.WorkspaceID)),
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(status.ServerName) == "" {
		status.ServerName = serverName
	}
	payload := mcpAuthStatusPayload{
		Status:      status,
		RepairPaths: mcpAuthRepairPathsFor(status.ServerName),
	}
	return structuredResult(payload, fmt.Sprintf("%s %s", status.ServerName, status.Status))
}

func mcpAuthRepairPathsFor(serverName string) mcpAuthRepairPaths {
	arg := strconv.Quote(strings.TrimSpace(serverName))
	return mcpAuthRepairPaths{
		StatusCLI:    "agh mcp auth status " + arg,
		LoginCLI:     "agh mcp auth login " + arg,
		LogoutCLI:    "agh mcp auth logout " + arg,
		SettingsHTTP: nativeMCPAuthToolsAPISettingsMCPServersPath,
		SettingsUDS:  nativeMCPAuthToolsAPISettingsMCPServersPath,
		Note:         "Login and logout remain management-only and are not exposed as tool calls.",
	}
}

func mcpProbeState(status toolspkg.MCPAuthStatus) string {
	if reason, ok := toolspkg.MCPAuthStatusReason(status); ok {
		switch reason {
		case toolspkg.ReasonMCPAuthRequired,
			toolspkg.ReasonMCPAuthExpired,
			toolspkg.ReasonMCPAuthInvalid,
			toolspkg.ReasonMCPAuthRefreshFailed:
			return nativeMCPStateAuthBlocked
		case toolspkg.ReasonMCPAuthUnconfigured:
			return nativeMCPStateUnavailable
		}
	}
	return nativeMCPStateHealthy
}
