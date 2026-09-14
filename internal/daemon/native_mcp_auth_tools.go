package daemon

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/store"

	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	nativeMCPAuthToolsAPISettingsMCPServersPath = "/api/settings/mcp-servers"
	nativeMCPCallableDiscoveryNote              = "Auth-blocked MCP tools are omitted from callable discovery; " +
		"use compozy__mcp_status or compozy__mcp_auth_status for repair detail."
	nativeMCPStateHealthy     = "healthy"
	nativeMCPStateAuthBlocked = "auth-blocked"
	nativeMCPStateUnavailable = "unavailable"
)

type mcpAuthStatusInput struct {
	Owner      string `json:"owner,omitempty"`
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
	if err := validateNativeMCPOwner(input.Owner); err != nil {
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

		ProfileID:     scope.ProfileID,
		Kind:          toolspkg.SourceMCP,
		Owner:         serverName,
		RawServerName: serverName,
		WorkspaceID:   workspaceID,
	}, strings.TrimSpace(input.Owner),
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(status.ServerName) == "" {
		status.ServerName = serverName
	}
	envelope, err := n.mcpStatusCollection(ctx, scope, req.ToolID, workspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}

	runtimeStatus, found := mcpRuntimeStatusByName(
		envelope.MCPServers,
		serverName,
		firstNonEmpty(status.Owner, input.Owner),
	)
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
		RepairPaths:           mcpAuthRepairPathsFor(status),
		CallableDiscoveryNote: nativeMCPCallableDiscoveryNote,
	}
	return structuredResult(payload, fmt.Sprintf("%s %s", status.ServerName, state))
}

func mcpRuntimeStatusByName(
	servers []settingspkg.MCPServerItem, serverName, owner string,
) (*settingspkg.MCPServerRuntimeStatus, bool) {
	want := strings.TrimSpace(serverName)
	var selected *settingspkg.MCPServerItem
	for i := range servers {
		server := &servers[i]
		actualOwner := firstNonEmpty(server.Owner, mcpDefinitionOwnerManual)
		if owner != "" && actualOwner != owner {
			continue
		}
		if server.RuntimeName != want &&
			(server.Name != want || (owner == "" && actualOwner != mcpDefinitionOwnerManual)) {
			continue
		}
		if selected != nil {
			selectedOwner := firstNonEmpty(selected.Owner, mcpDefinitionOwnerManual)
			if owner == "" && selectedOwner == mcpDefinitionOwnerManual && actualOwner != mcpDefinitionOwnerManual {
				continue
			}
			if actualOwner == selectedOwner && selected.WorkspaceID != "" && server.WorkspaceID == "" {
				continue
			}
		}
		selected = server
	}
	if selected == nil {
		return nil, false
	}
	if selected.RuntimeStatus == nil {
		return nil, true
	}
	status := *selected.RuntimeStatus
	return &status, true
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
	if err := validateNativeMCPOwner(input.Owner); err != nil {
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

		ProfileID:     scope.ProfileID,
		Kind:          toolspkg.SourceMCP,
		Owner:         serverName,
		RawServerName: serverName,
		WorkspaceID:   strings.TrimSpace(firstNonEmpty(scope.WorkspaceID, req.WorkspaceID)),
	}, strings.TrimSpace(input.Owner),
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(status.ServerName) == "" {
		status.ServerName = serverName
	}
	payload := mcpAuthStatusPayload{
		Status:      status,
		RepairPaths: mcpAuthRepairPathsFor(status),
	}
	return structuredResult(payload, fmt.Sprintf("%s %s", status.ServerName, status.Status))
}

func mcpAuthRepairPathsFor(status toolspkg.MCPAuthStatus) mcpAuthRepairPaths {
	arg := strconv.Quote(strings.TrimSpace(status.ServerName))
	if status.Owner != "" {
		arg += " --owner " + strconv.Quote(status.Owner)
	}
	switch mcpauth.Scope(status.Scope) {
	case mcpauth.ScopeWorkspace:
		arg += " --scope workspace --workspace " + strconv.Quote(status.WorkspaceID)
	case mcpauth.ScopeProfile:
		arg += " --scope profile --profile " + strconv.Quote(status.WorkspaceID)
	case mcpauth.ScopeWorkspaceProfile:
		workspaceID, profileName, _ := strings.Cut(status.WorkspaceID, "@pf:")
		arg += " --scope profile --profile " + strconv.Quote(profileName) + " --workspace " + strconv.Quote(workspaceID)
	}

	return mcpAuthRepairPaths{
		StatusCLI:    "compozy mcp auth status " + arg,
		LoginCLI:     "compozy mcp auth login " + arg,
		LogoutCLI:    "compozy mcp auth logout " + arg,
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

func validateNativeMCPOwner(owner string) error {
	target := mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: "server", Owner: strings.TrimSpace(owner)}
	if err := target.Normalize().Validate(); err != nil {
		return toolspkg.NewValidationError("owner", toolspkg.ReasonSchemaInvalid, err.Error())
	}
	return nil
}

func (n *daemonNativeTools) mcpStatusCollection(
	ctx context.Context, scope toolspkg.Scope, toolID toolspkg.ToolID, workspaceID string,
) (settingspkg.CollectionEnvelope, error) {
	collectionRequest := settingspkg.CollectionRequest{
		Collection: settingspkg.CollectionMCPServers,
		Scope:      settingspkg.ScopeUser,
	}
	if workspaceID != "" {
		collectionRequest.Scope = settingspkg.ScopeWorkspace
		collectionRequest.WorkspaceID = workspaceID
	}
	if profileID := strings.TrimSpace(scope.ProfileID); profileID != "" && profileID != store.DefaultProfileID {
		if n.deps.Profiles == nil {
			return settingspkg.CollectionEnvelope{}, fmt.Errorf("daemon: profile reader is required for MCP status")
		}
		profileName, err := n.deps.Profiles.ProfileName(ctx, profileID)
		if err != nil {
			return settingspkg.CollectionEnvelope{}, err
		}
		collectionRequest.Scope, collectionRequest.ProfileName = settingspkg.ScopeProfile, profileName
	}
	settingsService := n.settingsService()
	if settingsService == nil {
		return settingspkg.CollectionEnvelope{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			toolID,
			"mcp runtime status provider is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	envelope, err := settingsService.ListCollection(ctx, collectionRequest)
	if err != nil {
		return settingspkg.CollectionEnvelope{}, fmt.Errorf("daemon: list MCP runtime status: %w", err)
	}
	return envelope, nil
}
