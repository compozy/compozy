package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/agh/internal/api/core"
	settingspkg "github.com/compozy/agh/internal/settings"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonNativeMCPAuthStatusTool(t *testing.T) {
	t.Parallel()

	t.Run("Should expose redacted status with management repair paths", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
		provider := &nativeMCPAuthStatusProvider{
			status: toolspkg.MCPAuthStatus{
				ServerName:   "linear",
				Status:       "needs_login",
				AuthType:     "oauth2_pkce",
				ClientID:     "public-client-id",
				Scopes:       []string{"read", "write"},
				ExpiresAt:    &expiresAt,
				Refreshable:  true,
				TokenPresent: true,
				Diagnostic:   "login required",
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return provider
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-auth"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPAuthStatus,
				Input:  json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(mcp_auth_status) error = %v", err)
		}
		if provider.source.RawServerName != "linear" ||
			provider.source.Owner != "linear" ||
			provider.source.Kind != toolspkg.SourceMCP ||
			provider.source.WorkspaceID != "ws-auth" {
			t.Fatalf("provider source = %#v, want MCP source for linear", provider.source)
		}

		var payload mcpAuthStatusPayload
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("Unmarshal(mcp auth payload) error = %v", err)
		}
		if payload.Status.Status != "needs_login" ||
			payload.Status.AuthType != "oauth2_pkce" ||
			payload.Status.ClientID != "public-client-id" ||
			!payload.Status.TokenPresent ||
			!payload.Status.Refreshable {
			t.Fatalf("payload status = %#v, want redacted auth status model", payload.Status)
		}
		if payload.RepairPaths.LoginCLI != `agh mcp auth login "linear"` ||
			payload.RepairPaths.LogoutCLI != `agh mcp auth logout "linear"` ||
			payload.RepairPaths.SettingsHTTP != "/api/settings/mcp-servers" {
			t.Fatalf("repair paths = %#v, want management paths", payload.RepairPaths)
		}
		encoded := string(result.Structured)
		for _, needle := range []string{
			"access_token",
			"refresh_token",
			"client_secret",
			"code_verifier",
			"code_challenge",
			"callback_secret",
			"authorization_code",
		} {
			if strings.Contains(encoded, needle) {
				t.Fatalf("mcp auth status leaked %q in structured result: %s", needle, encoded)
			}
		}
	})

	t.Run("Should expose workspace-scoped dead runtime state and reason", func(t *testing.T) {
		t.Parallel()

		settingsService := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{{
			Name:        "linear",
			Scope:       settingspkg.ScopeWorkspace,
			WorkspaceID: "ws-dead",
			RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{
				Configured: true,
				State:      settingspkg.MCPServerRuntimeStateDead,
				Probe:      settingspkg.MCPServerProbeSkipped,
				Reason:     string(toolspkg.ReasonBackendDead),
				Diagnostic: "sidecar terminated",
			},
		}}}
		provider := &nativeMCPAuthStatusProvider{status: toolspkg.MCPAuthStatus{
			ServerName: "linear",
			Status:     "unconfigured",
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return provider
			},
			Settings: func() core.SettingsService { return settingsService },
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-dead"},
			toolspkg.CallRequest{
				ToolID:      toolspkg.ToolIDMCPStatus,
				WorkspaceID: "ws-dead",
				Input:       json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(mcp_status) error = %v", err)
		}
		var payload mcpStatusPayload
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(mcp status) error = %v", err)
		}
		if payload.State != "dead" || payload.Runtime == nil ||
			payload.Runtime.Reason != string(toolspkg.ReasonBackendDead) ||
			payload.Runtime.Diagnostic != "sidecar terminated" {
			t.Fatalf("mcp status payload = %#v, want dead runtime with backend_dead", payload)
		}
		if settingsService.lastRequest.Scope != settingspkg.ScopeWorkspace ||
			settingsService.lastRequest.WorkspaceID != "ws-dead" {
			t.Fatalf("settings request = %#v, want workspace ws-dead", settingsService.lastRequest)
		}
		if provider.source.WorkspaceID != "ws-dead" {
			t.Fatalf("MCP auth source = %#v, want workspace ws-dead", provider.source)
		}
	})

	t.Run("Should expose only status tools for MCP auth diagnostics", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return &nativeMCPAuthStatusProvider{}
			},
			Settings: func() core.SettingsService { return &nativeMCPSettingsService{} },
		}, nativeApproveAllPolicyInputs())

		views, err := registry.SessionProjection(t.Context(), toolspkg.Scope{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		requireNativeViewContains(t, views, toolspkg.ToolIDMCPStatus)
		requireNativeViewContains(t, views, toolspkg.ToolIDMCPAuthStatus)
		requireNativeViewExcludes(t, views, toolspkg.ToolID("agh__mcp_auth_login"))
		requireNativeViewExcludes(t, views, toolspkg.ToolID("agh__mcp_auth_logout"))
	})

	t.Run("Should expose MCP probe status without login or logout tools", func(t *testing.T) {
		t.Parallel()

		provider := &nativeMCPAuthStatusProvider{
			status: toolspkg.MCPAuthStatus{
				ServerName: "linear",
				Status:     "needs_login",
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return provider
			},
			Settings: func() core.SettingsService {
				return &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{{
					Name: "linear",
					RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{
						Configured: true,
						State:      settingspkg.MCPServerRuntimeStateAuthRequired,
						Probe:      settingspkg.MCPServerProbeSkipped,
					},
				}}}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPStatus,
				Input:  json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(mcp_status) error = %v", err)
		}
		var payload mcpStatusPayload
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("Unmarshal(mcp status payload) error = %v", err)
		}
		if payload.State != "auth-blocked" ||
			payload.RepairPaths.LoginCLI != `agh mcp auth login "linear"` ||
			!strings.Contains(payload.CallableDiscoveryNote, "omitted from callable discovery") {
			t.Fatalf("payload = %#v, want auth-blocked probe with management repair paths", payload)
		}
	})

	t.Run("Should reject status calls when MCP auth provider is unavailable", func(t *testing.T) {
		t.Parallel()

		nativeTools := &daemonNativeTools{deps: &daemonNativeToolsDeps{}}
		_, err := nativeTools.mcpAuthStatus(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPAuthStatus,
				Input:  json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolUnavailable, toolspkg.ReasonDependencyMissing)
	})

	t.Run("Should propagate MCP auth provider status errors", func(t *testing.T) {
		t.Parallel()

		sentinelErr := errors.New("status failed")
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return &nativeMCPAuthStatusProvider{err: sentinelErr}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPAuthStatus,
				Input:  json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		if !errors.Is(err, sentinelErr) {
			t.Fatalf("Registry.Call(mcp_auth_status) error = %v, want %v", err, sentinelErr)
		}
	})

	t.Run("Should use requested server name when provider leaves status name empty", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return &nativeMCPAuthStatusProvider{
					status: toolspkg.MCPAuthStatus{
						Status:     "not_configured",
						Diagnostic: "server is not configured for OAuth",
					},
					preserveEmptyServerName: true,
				}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPAuthStatus,
				Input:  json.RawMessage(`{"server_name":"linear"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(mcp_auth_status) error = %v", err)
		}
		var payload mcpAuthStatusPayload
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("Unmarshal(mcp auth payload) error = %v", err)
		}
		if payload.Status.ServerName != "linear" ||
			payload.RepairPaths.StatusCLI != `agh mcp auth status "linear"` {
			t.Fatalf("payload = %#v, want requested server name in status and repair paths", payload)
		}
	})
}

type nativeMCPAuthStatusProvider struct {
	status                  toolspkg.MCPAuthStatus
	source                  toolspkg.SourceRef
	err                     error
	preserveEmptyServerName bool
}

type nativeMCPSettingsService struct {
	core.SettingsService
	servers     []settingspkg.MCPServerItem
	lastRequest settingspkg.CollectionRequest
}

func (s *nativeMCPSettingsService) ListCollection(
	_ context.Context,
	req settingspkg.CollectionRequest,
) (settingspkg.CollectionEnvelope, error) {
	s.lastRequest = req
	return settingspkg.CollectionEnvelope{
		Collection:  req.Collection,
		Scope:       req.Scope,
		WorkspaceID: req.WorkspaceID,
		MCPServers:  append([]settingspkg.MCPServerItem(nil), s.servers...),
	}, nil
}

func (p *nativeMCPAuthStatusProvider) Status(
	_ context.Context,
	source toolspkg.SourceRef,
) (toolspkg.MCPAuthStatus, error) {
	p.source = source
	if p.err != nil {
		return toolspkg.MCPAuthStatus{}, p.err
	}
	status := p.status
	if !p.preserveEmptyServerName && strings.TrimSpace(status.ServerName) == "" {
		status.ServerName = strings.TrimSpace(source.RawServerName)
	}
	if strings.TrimSpace(status.Status) == "" {
		status.Status = "authenticated"
	}
	return status, nil
}
