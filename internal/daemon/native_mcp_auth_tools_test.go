package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	profilepkg "github.com/compozy/compozy/internal/profile"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
			Sessions: nativeTestSessionManager("ws-auth"),
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
		if payload.RepairPaths.LoginCLI != `compozy mcp auth login "linear"` ||
			payload.RepairPaths.LogoutCLI != `compozy mcp auth logout "linear"` ||
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

	// Invariant: both native diagnostics preserve an explicit owner and repair the owning scope.
	// Owner: native tool adapter; canonical suite: TestDaemonNativeMCPAuthStatusTool.
	for _, toolID := range []toolspkg.ToolID{toolspkg.ToolIDMCPStatus, toolspkg.ToolIDMCPAuthStatus} {
		t.Run("Should preserve extension ownership for "+string(toolID), func(t *testing.T) {
			t.Parallel()
			provider := &nativeMCPAuthStatusProvider{status: toolspkg.MCPAuthStatus{
				ServerName:  "github",
				Owner:       "extension:github",
				Scope:       "workspace",
				WorkspaceID: "ws-owned",
				Status:      "needs_login",
			}}
			settingsService := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{
				{
					Name:          "github",
					Owner:         "manual",
					RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{State: settingspkg.MCPServerRuntimeStateReady},
				},
				{
					Name:        "github",
					Owner:       "extension:github",
					RuntimeName: "github.github",
					WorkspaceID: "ws-owned",
					RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{
						State: settingspkg.MCPServerRuntimeStateAuthRequired,
					},
				},
			}}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Sessions: nativeTestSessionManager("ws-owned"),
				MCPAuth:  func() toolspkg.MCPAuthStatusProvider { return provider },
				Settings: func() core.SettingsService { return settingsService },
			}, nativeApproveAllPolicyInputs())
			result, err := registry.Call(
				t.Context(),
				toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-owned"},
				toolspkg.CallRequest{
					ToolID: toolID, Input: json.RawMessage(`{"server_name":"github","owner":"extension:github"}`),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if provider.owner != "extension:github" {
				t.Fatalf("source lost owner: %#v", provider.source)
			}
			var payload struct {
				RepairPaths mcpAuthRepairPaths                  `json:"repair_paths"`
				Runtime     *settingspkg.MCPServerRuntimeStatus `json:"runtime"`
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.RepairPaths.LoginCLI != `compozy mcp auth login "github" --owner "extension:github" --scope workspace --workspace "ws-owned"` {
				t.Fatalf("wrong repair target: %#v", payload.RepairPaths)
			}
			if toolID == toolspkg.ToolIDMCPStatus &&
				(payload.Runtime == nil || payload.Runtime.State != settingspkg.MCPServerRuntimeStateAuthRequired) {
				t.Fatalf("selected manual runtime: %#v", payload.Runtime)
			}
		})
	}

	// Invariant: a workspace session inheriting a user-declared server reports the effective user
	// credential labeled inherited and repairs the user scope that satisfies the runtime (#1072).
	// Owner: native tool adapter; canonical suite: TestDaemonNativeMCPAuthStatusTool.
	for _, toolID := range []toolspkg.ToolID{toolspkg.ToolIDMCPStatus, toolspkg.ToolIDMCPAuthStatus} {
		t.Run("Should label the inherited user credential for "+string(toolID), func(t *testing.T) {
			t.Parallel()
			provider := &nativeMCPAuthStatusProvider{status: toolspkg.MCPAuthStatus{
				ServerName: "newrelic", Owner: "manual", Scope: "user", Status: "authenticated",
				TokenPresent: true, Refreshable: true,
			}}
			settingsService := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{{
				Name: "newrelic", Owner: "manual",
				RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{State: settingspkg.MCPServerRuntimeStateReady},
			}}}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Sessions: nativeTestSessionManager("ws-inherit"),
				MCPAuth:  func() toolspkg.MCPAuthStatusProvider { return provider },
				Settings: func() core.SettingsService { return settingsService },
			}, nativeApproveAllPolicyInputs())
			result, err := registry.Call(
				t.Context(),
				toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-inherit"},
				toolspkg.CallRequest{ToolID: toolID, Input: json.RawMessage(`{"server_name":"newrelic"}`)},
			)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Auth        *toolspkg.MCPAuthStatus `json:"auth"`
				Status      *toolspkg.MCPAuthStatus `json:"status"`
				State       string                  `json:"state"`
				RepairPaths mcpAuthRepairPaths      `json:"repair_paths"`
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatal(err)
			}
			status := payload.Status
			if toolID == toolspkg.ToolIDMCPStatus {
				status = payload.Auth
				if payload.State != nativeMCPStateHealthy {
					t.Fatalf("state = %q, want healthy", payload.State)
				}
			}
			if status == nil || status.Scope != "user" || status.Status != "authenticated" || !status.Inherited {
				t.Fatalf("auth status = %#v, want inherited authenticated user record", status)
			}
			if payload.RepairPaths.LoginCLI != `compozy mcp auth login "newrelic" --owner "manual"` {
				t.Fatalf("login repair = %q, want the user scope", payload.RepairPaths.LoginCLI)
			}
		})
	}

	// Invariant: a named-profile session inherits a user credential, and the label depends on the
	// session profile reaching both native status tools. Owner: native tool adapter.
	for _, toolID := range []toolspkg.ToolID{toolspkg.ToolIDMCPStatus, toolspkg.ToolIDMCPAuthStatus} {
		t.Run("Should label a user credential inherited by a named profile for "+string(toolID), func(t *testing.T) {
			t.Parallel()
			const profileID = "profile-marketing"
			provider := &nativeMCPAuthStatusProvider{status: toolspkg.MCPAuthStatus{
				ServerName: "newrelic", Owner: "manual", Scope: "user", Status: "authenticated", TokenPresent: true,
			}}
			settingsService := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{{
				Name: "newrelic", Owner: "manual",
				RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{State: settingspkg.MCPServerRuntimeStateReady},
			}}}
			tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
				MCPAuth:  func() toolspkg.MCPAuthStatusProvider { return provider },
				Settings: func() core.SettingsService { return settingsService },
				Profiles: nativeProfileReaderStub{profiles: []profilepkg.WithCounts{{
					ID: profileID, Name: "marketing", State: profilepkg.StateActive,
				}}},
			}}
			call := tools.mcpAuthStatus
			if toolID == toolspkg.ToolIDMCPStatus {
				call = tools.mcpStatus
			}
			result, err := call(t.Context(), toolspkg.Scope{ProfileID: profileID}, toolspkg.CallRequest{
				ToolID: toolID, Input: json.RawMessage(`{"server_name":"newrelic"}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Auth   *toolspkg.MCPAuthStatus `json:"auth"`
				Status *toolspkg.MCPAuthStatus `json:"status"`
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatal(err)
			}
			status := payload.Status
			if toolID == toolspkg.ToolIDMCPStatus {
				status = payload.Auth
			}
			if status == nil || !status.Inherited || provider.source.ProfileID != profileID {
				t.Fatalf(
					"status = %#v source = %#v, want inherited user credential for the profile",
					status,
					provider.source,
				)
			}
		})
	}

	t.Run("Should label credentials owned by a broader scope than the session context", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name, scope, profileID, workspaceID string
			want                                bool
		}{
			{name: "Should not label a user credential in a user session", scope: "user"},
			{name: "Should label a user credential in a workspace session", scope: "user",
				workspaceID: "ws-team", want: true},
			{name: "Should label a user credential in a named profile session", scope: "user",
				profileID: "prof-marketing", want: true},
			{name: "Should not label a user credential in the default profile session", scope: "user",
				profileID: store.DefaultProfileID},
			{name: "Should not label a profile credential in its profile session", scope: "profile",
				profileID: "prof-marketing"},
			{name: "Should label a profile credential in a profile workspace session", scope: "profile",
				profileID: "prof-marketing", workspaceID: "ws-team", want: true},
			{name: "Should not label a workspace credential in its workspace session", scope: "workspace",
				workspaceID: "ws-team"},
			{name: "Should label a workspace credential in a named profile session", scope: "workspace",
				profileID: "prof-marketing", workspaceID: "ws-team", want: true},
			{name: "Should not label a workspace profile credential in its session", scope: "workspace_profile",
				profileID: "prof-marketing", workspaceID: "ws-team"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := mcpAuthStatusInherited(toolspkg.MCPAuthStatus{Scope: tc.scope}, tc.profileID, tc.workspaceID)
				if got != tc.want {
					t.Fatalf("mcpAuthStatusInherited(%+v) = %v, want %v", tc, got, tc.want)
				}
			})
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
			Sessions: nativeTestSessionManager("ws-dead"),
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
			Sessions: nativeTestSessionManager(""),
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
		requireNativeViewExcludes(t, views, toolspkg.ToolID("compozy__mcp_auth_login"))
		requireNativeViewExcludes(t, views, toolspkg.ToolID("compozy__mcp_auth_logout"))
	})

	t.Run("Should expose MCP probe status without login or logout tools", func(t *testing.T) {
		t.Parallel()

		provider := &nativeMCPAuthStatusProvider{
			status: toolspkg.MCPAuthStatus{
				ServerName: "linear",
				Status:     "needs_login",
			},
		}
		settingsService := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{{
			Name: "linear",
			RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{
				Configured: true,
				State:      settingspkg.MCPServerRuntimeStateAuthRequired,
				Probe:      settingspkg.MCPServerProbeSkipped,
			},
		}}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeTestSessionManager(""),
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return provider
			},
			Settings: func() core.SettingsService { return settingsService },
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
			payload.RepairPaths.LoginCLI != `compozy mcp auth login "linear"` ||
			!strings.Contains(payload.CallableDiscoveryNote, "omitted from callable discovery") {
			t.Fatalf("payload = %#v, want auth-blocked probe with management repair paths", payload)
		}
		if settingsService.lastRequest.Scope != settingspkg.ScopeUser || settingsService.lastRequest.WorkspaceID != "" {
			t.Fatalf("settings request = %#v, want user scope without workspace", settingsService.lastRequest)
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
			Sessions: nativeTestSessionManager(""),
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
			Sessions: nativeTestSessionManager(""),
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
			payload.RepairPaths.StatusCLI != `compozy mcp auth status "linear"` {
			t.Fatalf("payload = %#v, want requested server name in status and repair paths", payload)
		}
	})
}

type nativeMCPAuthStatusProvider struct {
	status                  toolspkg.MCPAuthStatus
	source                  toolspkg.SourceRef
	owner                   string
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
	source toolspkg.SourceRef, definitionOwner string,
) (toolspkg.MCPAuthStatus, error) {
	p.source = source
	p.owner = definitionOwner
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
