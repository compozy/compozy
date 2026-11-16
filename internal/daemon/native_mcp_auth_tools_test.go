package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
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

func TestRuntimeMCPInventory(t *testing.T) {
	t.Parallel()

	t.Run("Should reject construction without a digest key provider", func(t *testing.T) {
		t.Parallel()
		if inventory, err := newRuntimeMCPInventory(nil); err == nil || inventory != nil {
			t.Fatal("daemon inventory accepted a missing digest key provider")
		}
	})
	t.Run("Should propagate an unreadable installation key at construction", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, "mcp-digest.key"), []byte("invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
		if inventory, err := newRuntimeMCPInventory(
			vault.NewMCPDigestKeyProvider(home),
		); err == nil ||
			inventory != nil {
			t.Fatal("daemon inventory substituted an ephemeral key")
		}
	})
	// Invariant: distinct context reads have bounded retention; owner: daemon inventory cache.
	t.Run("Should evict old context snapshots at the cache limit", func(t *testing.T) {
		t.Parallel()
		inventory := newTestRuntimeMCPInventory(t)
		root := t.TempDir()
		for i := range runtimeMCPCacheLimit + 2 {
			_, err := inventory.List(
				t.Context(),
				runtimemcp.Request{
					Runtime: "claude",
					Context: runtimemcp.ReadContext{
						Home:       root,
						SystemRoot: filepath.Join(root, "system"),
						CWD:        filepath.Join(root, fmt.Sprint(i)),
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(inventory.cache) > runtimeMCPCacheLimit {
				t.Fatal("unbounded cache")
			}
		}
	})

	// Invariant: file changes invalidate only their context and snapshots cannot be mutated by callers. Owner: daemon inventory; UT-054.
	t.Run("Should invalidate runtime snapshots on file replacement UT-054", func(t *testing.T) {
		t.Parallel()
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".claude.json")
		if err := os.WriteFile(path, []byte(`{"mcpServers":{"old":{"command":"echo"}}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		inventory := newTestRuntimeMCPInventory(t)
		request := runtimemcp.Request{
			Runtime: "claude",
			Context: runtimemcp.ReadContext{Home: root, SystemRoot: filepath.Join(root, "system")},
		}
		first, err := inventory.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		first.Runtimes[0].Servers[0].Name = "mutated"
		again, err := inventory.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if again.Runtimes[0].Servers[0].Name != "old" {
			t.Fatal("caller mutated cache")
		}
		if err := os.WriteFile(path, []byte(`{"mcpServers":{"new-name":{"command":"echo"}}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		inventory.Poll()
		after, err := inventory.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if after.Runtimes[0].Servers[0].Name != "new-name" ||
			after.Runtimes[0].ActivationDigest == again.Runtimes[0].ActivationDigest {
			t.Fatalf("stale snapshot: %+v", after)
		}
		if err := os.WriteFile(path, []byte(`{"mcpServers":`), 0o600); err != nil {
			t.Fatal(err)
		}
		unreadable, err := inventory.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if unreadable.Runtimes[0].ReadState != "unreadable" ||
			unreadable.Runtimes[0].ActivationDigest != after.Runtimes[0].ActivationDigest {
			t.Fatal("transient read failure discarded the last activation identity")
		}
	})
	t.Run("Should filter native MCP inventory to Codex UT-090", func(t *testing.T) {
		t.Parallel()
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(root, ".codex", "config.toml"),
			[]byte("[mcp_servers.savee]\nurl = \"https://example.test/mcp\"\n"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		settings := &nativeRuntimeMCPSettings{
			inventory:   newTestRuntimeMCPInventory(t),
			readContext: runtimemcp.ReadContext{Home: root, SystemRoot: filepath.Join(root, "system")},
		}
		registry := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{
				Sessions: nativeTestSessionManager(""),
				Settings: func() core.SettingsService { return settings },
			},
			nativeApproveAllPolicyInputs(),
		)
		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPList,
				Input:  json.RawMessage(`{"runtime":"codex","include_runtimes":true}`),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		var payload contract.MCPListResponse
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Runtimes) != 1 || payload.Runtimes[0].Runtime != "codex" ||
			payload.Runtimes[0].Servers[0].Name != "savee" ||
			payload.Runtimes[0].Servers[0].Auth != "unknown" {
			t.Fatalf("payload=%+v", payload)
		}
	})
}

type nativeRuntimeMCPSettings struct {
	nativeMCPSettingsService
	inventory   *runtimeMCPInventory
	readContext runtimemcp.ReadContext
}

var _ core.RuntimeMCPReader = (*nativeRuntimeMCPSettings)(nil)

func (s *nativeRuntimeMCPSettings) ListRuntimeMCP(
	ctx context.Context,
	_ settingspkg.CollectionRequest,
	runtime string,
) (runtimemcp.Inventory, error) {
	return s.inventory.List(ctx, runtimemcp.Request{Context: s.readContext, Runtime: runtime})
}

// Invariant (UT-092/094): allow and revoke mutate the caller's consent through
// the registry, while a narrowed session cannot mutate it. Owner: native tool adapter.
func TestDaemonNativeMCPAllow(t *testing.T) {
	t.Parallel()
	t.Run("Should allow and revoke through the public registry without OAuth", func(t *testing.T) {
		t.Parallel()
		state, consentReq := newMCPConsentFixture(t)
		writeMCPDeclarationFile(
			t,
			consentReq.Server.DefinitionSource,
			`{"mcpServers":{"sentry":{"command":"fixture"}}}`,
		)
		service, err := settingspkg.NewService(
			testHomePaths(t),
			settingspkg.Dependencies{
				WorkspaceResolver:       state.workspaceResolver,
				ProfileResolver:         state.profiles,
				AttentionWorkspaceMutes: state.registry.(settingspkg.AttentionWorkspaceMuteStore),
				MCPConsent:              settingsMCPConsent{state: state},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		readReq := settingspkg.CollectionRequest{
			Collection:        settingspkg.CollectionMCPServers,
			Scope:             settingspkg.ScopeWorkspace,
			WorkspaceID:       consentReq.WorkspaceID,
			SkipRuntimeStatus: true,
		}
		listed, err := service.ListCollection(t.Context(), readReq)
		if err != nil || len(listed.MCPServers) != 1 || listed.MCPServers[0].Approval.Reason != "new" {
			t.Fatalf("new approval payload=%+v, %v", listed.MCPServers, err)
		}
		first := listed.MCPServers[0].Approval.DefinitionDigest
		if _, err := (settingsMCPConsent{state: state}).ApproveMCP(t.Context(), consentReq, first, false); err != nil {
			t.Fatal(err)
		}
		writeMCPDeclarationFile(
			t,
			consentReq.Server.DefinitionSource,
			`{"mcpServers":{"sentry":{"command":"fixture","args":["changed"]}}}`,
		)
		state.workspaceResolver.InvalidateAll()
		listed, err = service.ListCollection(t.Context(), readReq)
		if err != nil || len(listed.MCPServers) != 1 || listed.MCPServers[0].Approval.Reason != "changed" {
			t.Fatalf("changed approval payload=%+v, %v", listed.MCPServers, err)
		}
		writeMCPDeclarationFile(
			t,
			consentReq.Server.DefinitionSource,
			`{"mcpServers":{"sentry":{"command":"fixture"}}}`,
		)
		state.workspaceResolver.InvalidateAll()
		if _, err := (settingsMCPConsent{state: state}).ApproveMCP(t.Context(), consentReq, "", true); err != nil {
			t.Fatal(err)
		}

		tokens := toolspkg.NewApprovalTokenStore(time.Minute)
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(
			t,
			&daemonNativeToolsDeps{
				Sessions:   nativeTestSessionManager(consentReq.WorkspaceID),
				Workspaces: state.workspaceResolver,
				Settings:   func() core.SettingsService { return service },
			},
			toolspkg.NewStaticPolicyInputResolver(func() toolspkg.PolicyInputs {
				inputs := nativeApproveAllPolicyInputs()
				inputs.ApprovalAvailable = true
				return inputs
			}()),
			nil,
			toolspkg.WithApprovalBridge(newToolApprovalBridge(nil, time.Minute, tokens, nil, discardLogger())),
		)
		decision, err := (settingsMCPConsent{state: state}).MCPConsent(t.Context(), consentReq)
		if err != nil {
			t.Fatal(err)
		}
		scope := toolspkg.Scope{
			SessionID:   "sess-1",
			WorkspaceID: consentReq.WorkspaceID,
			ProfileID:   store.DefaultProfileID,
		}
		for _, action := range []string{"allow", "revoke"} {
			input, err := json.Marshal(
				map[string]string{
					"server_name":       "sentry",
					"decision":          action,
					"definition_digest": decision.DefinitionDigest,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			issuer := &boundToolApprovalIssuer{
				tokens:   tokens,
				registry: registry,
				binder: newNativeWorkspaceInputBinder(
					state.workspaceResolver,
					nativeTestSessionManager(consentReq.WorkspaceID),
					nil,
					nil,
				),
			}
			grant, err := issuer.CreateToolApproval(t.Context(), scope, toolspkg.ApprovalTokenRequest{
				ToolID: toolspkg.ToolIDMCPAllow, Input: input,
			})
			if err != nil {
				t.Fatal(err)
			}

			result, err := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{ToolID: toolspkg.ToolIDMCPAllow, Input: input, ApprovalToken: grant.ApprovalToken},
			)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Approval struct {
					State string `json:"state"`
				} `json:"approval"`
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatal(err)
			}
			current, err := (settingsMCPConsent{state: state}).MCPConsent(t.Context(), consentReq)
			if err != nil || current.Admitted != (action == "allow") ||
				(payload.Approval.State == "allowed") != current.Admitted {
				t.Fatalf("%s: %+v %s %v", action, current, result.Structured, err)
			}
		}
		restricted := nativeApproveAllPolicyInputs()
		restricted.Session.Enforced = true
		restricted.Session.Tools = []toolspkg.ToolID{"compozy__mcp_list"}
		narrowed := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{
				Sessions:   nativeTestSessionManager(consentReq.WorkspaceID),
				Workspaces: state.workspaceResolver,
				Settings:   func() core.SettingsService { return service },
			},
			restricted,
		)
		_, err = narrowed.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMCPAllow,
				Input:  json.RawMessage(`{"server_name":"sentry","decision":"revoke"}`),
			},
		)
		failure, ok := errors.AsType[*toolspkg.ToolError](err)
		if !ok || !slices.Contains(failure.ReasonCodes, toolspkg.ReasonSessionDenied) {
			t.Fatalf("narrowed call=%v", err)
		}
	})
}

// Invariant: copies resolve current files, preserve definitions and reject unsupported sources. Owner: daemon copy service.
func TestRuntimeMCPCopy(t *testing.T) {
	t.Parallel()
	// Invariant: import dispatch exposes safe structured results and respects session narrowing. Owner: native adapter; canonical suite: TestRuntimeMCPCopy.
	t.Run("Should dispatch the import through the native registry", func(t *testing.T) {
		t.Parallel()
		fixture := newRuntimeMCPCopyFixture(
			t,
			`{"mcpServers":{"native":{"command":"echo","env":{"TOKEN":"native-private-value"}}}}`,
		)
		deps := &daemonNativeToolsDeps{
			Sessions: nativeTestSessionManager(""),
			Settings: func() core.SettingsService { return fixture.service },
		}
		request := toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDMCPImport,
			Input:  json.RawMessage(`{"runtime":"claude","server_name":"native","scope":"user"}`),
		}
		scope := toolspkg.Scope{SessionID: "sess-1", ProfileID: store.DefaultProfileID}
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		result, err := registry.Call(t.Context(), scope, request)
		if err != nil {
			t.Fatal(err)
		}
		var payload contract.RuntimeMCPImportResponse
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.MCPServer.Name != "native" || !slices.Contains(payload.MCPServer.SecretEnvKeys, "TOKEN") ||
			strings.Contains(string(result.Structured), "native-private-value") {
			t.Fatal("native import returned an unsafe or incomplete result")
		}
		_, err = registry.Call(t.Context(), scope, request)
		failure, ok := errors.AsType[*toolspkg.ToolError](err)
		if !ok || string(failure.Code) != "mcp_import_name_taken" {
			t.Fatalf("native import lost its domain error: %v", err)
		}
		policy := nativeApproveAllPolicyInputs()
		policy.Session.Enforced = true
		policy.Session.Tools = []toolspkg.ToolID{toolspkg.ToolIDMCPList}
		restricted := newDaemonNativeRegistry(t, deps, policy)
		_, err = restricted.Call(t.Context(), scope, request)
		failure, ok = errors.AsType[*toolspkg.ToolError](err)
		if !ok || !slices.Contains(failure.ReasonCodes, toolspkg.ReasonSessionDenied) {
			t.Fatalf("native import bypassed session policy: %v", err)
		}
	})

	t.Run("Should copy a manual HTTP definition UT-061", func(t *testing.T) {
		t.Parallel()
		fixture := newRuntimeMCPCopyFixture(
			t,
			`{"mcpServers":{"advos":{"type":"http","url":"https://example.test/mcp"}}}`,
		)

		request := runtimemcp.Request{Runtime: "claude", Context: fixture.readContext}
		listed, err := fixture.inventory.List(t.Context(), request)
		if err != nil || len(listed.Runtimes) != 1 || len(listed.Runtimes[0].Servers) != 1 {
			t.Fatalf("runtime listing=%+v %v", listed, err)
		}
		previous := listed.Runtimes[0].Servers[0]
		restarted, err := newRuntimeMCPInventory(vault.NewMCPDigestKeyProvider(fixture.home.HomeDir))
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := restarted.Resolve(t.Context(), request, "advos")
		if err != nil || resolved.Metadata().DefinitionDigest != previous.DefinitionDigest {
			t.Fatalf("restart changed runtime definition: %v", err)
		}
		service, err := settingspkg.NewService(fixture.home, settingspkg.Dependencies{
			RuntimeMCPInventory: restarted, ProfileResolver: fixture.profiles, AttentionWorkspaceMutes: fixture.db,
			ProviderSecrets:   fixture.secrets,
			RuntimeMCPContext: func() (runtimemcp.ReadContext, error) { return fixture.readContext, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		importer, ok := service.(core.RuntimeMCPImporter)
		if !ok {
			t.Fatal("settings does not provide runtime import")
		}
		result, err := importer.ImportRuntimeMCP(
			t.Context(),
			settingspkg.RuntimeMCPImportRequest{Runtime: "claude", ServerName: "advos"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if result.Source.DefinitionDigest != previous.DefinitionDigest {
			t.Fatal("import did not retain the listed definition digest")
		}
		item := result.Mutation.MCPServer
		if item == nil || item.Name != "advos" || item.Owner != "manual" || item.Scope != settingspkg.ScopeUser ||
			item.Transport != "http" {
			t.Fatalf("unexpected copied definition: %+v", item)
		}
		collection, err := fixture.service.ListCollection(
			t.Context(),
			settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, SkipRuntimeStatus: true},
		)
		if err != nil || len(collection.MCPServers) != 1 || collection.MCPServers[0].Name != "advos" {
			t.Fatalf("persisted copy missing: %v", err)
		}
	})
	t.Run("Should move literal values into Vault and require fresh OAuth UT-062", func(t *testing.T) {
		t.Parallel()
		source := `{"mcpServers":{"local":{"command":"echo","env":{"TOKEN":"private-env","EMPTY":""}},"oauth":{"type":"http","url":"https://example.test/mcp","auth":"oauth"},"headers":{"type":"http","url":"https://example.test/mcp","headers":{"Authorization":"Bearer private-header"}}}}`
		fixture := newRuntimeMCPCopyFixture(t, source)
		for _, name := range []string{"local", "oauth", "headers"} {
			result, err := fixture.importer.ImportRuntimeMCP(
				t.Context(),
				settingspkg.RuntimeMCPImportRequest{Runtime: "claude", ServerName: name},
			)
			if err != nil {
				t.Fatal(err)
			}
			if name == "oauth" &&
				(result.Mutation.MCPServer.AuthStatus == nil || result.Mutation.MCPServer.AuthStatus.Status != "needs_login" || result.Mutation.MCPServer.AuthStatus.TokenPresent) {
				t.Fatal("copy inherited OAuth credentials")
			}
		}
		for ref, want := range map[string]string{"vault:mcp/user/local/env/TOKEN": "private-env", "vault:mcp/user/local/env/EMPTY": "", "vault:mcp/user/headers/headers/617574686f72697a6174696f6e": "Bearer private-header"} {
			got, err := fixture.secrets.ResolveRef(t.Context(), ref)
			if err != nil || got != want {
				t.Fatalf("copied secret did not resolve: %v", err)
			}
		}
		sidecar, err := os.ReadFile(filepath.Join(fixture.home.HomeDir, "mcp.json"))
		if err != nil {
			t.Fatal(err)
		}
		config, err := os.ReadFile(fixture.home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range []string{string(sidecar), string(config)} {
			if strings.Contains(raw, "private-env") || strings.Contains(raw, "private-header") {
				t.Fatal("definition leaked literal credentials")
			}
		}
		if strings.Contains(string(sidecar), `"env"`) || !strings.Contains(string(sidecar), `"secret_env"`) {
			t.Fatal("env values were not replaced with references")
		}
		reloaded, err := settingspkg.NewService(
			fixture.home,
			settingspkg.Dependencies{ProviderSecrets: fixture.secrets,
				ProfileResolver: fixture.profiles, AttentionWorkspaceMutes: fixture.db},
		)
		if err != nil {
			t.Fatal(err)
		}
		collection, err := reloaded.ListCollection(
			t.Context(),
			settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, SkipRuntimeStatus: true},
		)
		if err != nil || len(collection.MCPServers) != 3 {
			t.Fatalf("copy did not survive service reopen: %v", err)
		}
		after, err := os.ReadFile(filepath.Join(fixture.root, ".claude.json"))
		if err != nil || string(after) != source {
			t.Fatal("copy modified the runtime source")
		}
	})
	t.Run("Should refuse an occupied target name without overwriting it UT-063", func(t *testing.T) {
		t.Parallel()
		fixture := newRuntimeMCPCopyFixture(t, `{"mcpServers":{"savee":{"command":"echo"}}}`)
		req := settingspkg.RuntimeMCPImportRequest{Runtime: "claude", ServerName: "savee"}
		if _, err := fixture.importer.ImportRuntimeMCP(t.Context(), req); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.importer.ImportRuntimeMCP(t.Context(), req)
		failure, ok := errors.AsType[*runtimemcp.ImportError](err)
		if !ok || failure.Code != "mcp_import_name_taken" ||
			failure.Message != "A CompozyOS server named savee already exists in Personal" {
			t.Fatalf("wrong conflict: %v", err)
		}
		req.Name = "savee-copy"
		if _, err := fixture.importer.ImportRuntimeMCP(t.Context(), req); err != nil {
			t.Fatal(err)
		}
	})
	for _, tc := range []struct{ name, definition, server, code, message string }{
		{"Should reject a plugin source UT-064", `{"command":"echo"}`, "plugin:figma:figma", "mcp_import_unsupported_source", "plugin:figma:figma is provided by a Claude Code plugin and can't be copied"},
		{"Should reject SSE UT-064", `{"type":"sse","url":"https://example.test/mcp"}`, "legacy", "mcp_import_unsupported_transport", "CompozyOS runs stdio and HTTP servers; legacy uses SSE"},
		{"Should reject malformed arguments UT-064", `{"command":"echo","args":["kept",3]}`, "invalid-args", "mcp_import_unsupported_definition", "invalid-args has a definition CompozyOS can't represent"},
		{"Should reject overlapping env aliases UT-064", `{"command":"echo","env":{"A":"x"},"environment":{"B":"y"}}`, "aliases", "mcp_import_unsupported_definition", "aliases has a definition CompozyOS can't represent"},
		{"Should reject forbidden env UT-064", `{"command":"echo","env":{"NODE_OPTIONS":"--require=module"}}`, "forbidden", "mcp_import_unsupported_definition", "forbidden has a definition CompozyOS can't represent"},
		{"Should reject runtime CWD UT-064", `{"command":"echo","cwd":"/tmp"}`, "local-scripts", "mcp_import_unsupported_definition", "local-scripts runs in its own working directory (cwd), which CompozyOS servers can't keep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRuntimeMCPCopyFixture(t, `{"mcpServers":{"`+tc.server+`":`+tc.definition+`}}`)
			inventory, err := fixture.inventory.List(
				t.Context(),
				runtimemcp.Request{Runtime: "claude", Context: fixture.readContext},
			)
			if err != nil || len(inventory.Runtimes) != 1 || len(inventory.Runtimes[0].Servers) != 1 ||
				inventory.Runtimes[0].Servers[0].Copyable {
				t.Fatalf("inventory permitted a nonrepresentable copy: %v", err)
			}
			_, err = fixture.importer.ImportRuntimeMCP(
				t.Context(),
				settingspkg.RuntimeMCPImportRequest{Runtime: "claude", ServerName: tc.server},
			)
			failure, ok := errors.AsType[*runtimemcp.ImportError](err)
			if !ok || failure.Code != tc.code || failure.Message != tc.message {
				t.Fatalf("wrong copy refusal: %v", err)
			}
			if _, err := os.Stat(filepath.Join(fixture.home.HomeDir, "mcp.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused copy wrote a definition")
			}
		})
	}
	// Invariant: stale Copy refuses both removed and edited definitions with the matching repair message.
	// Owner: daemon runtime MCP copy; canonical suite: TestRuntimeMCPCopy.
	for _, tc := range []struct{ name, source, message string }{
		{"removed", `{"mcpServers":{}}`, "The server advos is no longer in ~/.claude.json"},
		{"edited", `{"mcpServers":{"advos":{"command":"changed"}}}`, "The server advos changed in ~/.claude.json. List it again before copying"},
	} {
		t.Run("Should refuse a "+tc.name+" source since listing UT-065", func(t *testing.T) {
			t.Parallel()
			fixture := newRuntimeMCPCopyFixture(t, `{"mcpServers":{"advos":{"command":"echo"}}}`)
			if _, err := fixture.inventory.List(
				t.Context(),
				runtimemcp.Request{Runtime: "claude", Context: fixture.readContext},
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fixture.root, ".claude.json"), []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := fixture.importer.ImportRuntimeMCP(
				t.Context(),
				settingspkg.RuntimeMCPImportRequest{Runtime: "claude", ServerName: "advos"},
			)
			failure, ok := errors.AsType[*runtimemcp.ImportError](err)
			if !ok || failure.Code != "mcp_server_not_found" ||
				failure.Message != tc.message {
				t.Fatalf("wrong stale-source error: %v", err)
			}
		})
	}
}

type runtimeMCPCopyFixture struct {
	profiles    *profilepkg.Manager
	db          *globaldb.GlobalDB
	root        string
	home        compozyconfig.HomePaths
	readContext runtimemcp.ReadContext
	inventory   *runtimeMCPInventory
	service     settingspkg.Service
	importer    core.RuntimeMCPImporter
	secrets     *vault.Service
}

func newRuntimeMCPCopyFixture(t *testing.T, source string) runtimeMCPCopyFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude.json"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	db := openDaemonTestGlobalDB(t)
	secrets, err := vault.NewService(db, vault.NewFileKeyProvider(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	home := testHomePaths(t)
	inventory, err := newRuntimeMCPInventory(vault.NewMCPDigestKeyProvider(home.HomeDir))
	if err != nil {
		t.Fatal(err)
	}
	readContext := runtimemcp.ReadContext{Home: root, SystemRoot: filepath.Join(root, "system")}
	profiles, err := profilepkg.NewManager(profilepkg.WithStore(db), profilepkg.WithHomePaths(home))
	if err != nil {
		t.Fatal(err)
	}

	service, err := settingspkg.NewService(
		home,
		settingspkg.Dependencies{
			RuntimeMCPInventory:     inventory,
			ProfileResolver:         profiles,
			AttentionWorkspaceMutes: db,
			ProviderSecrets:         secrets,
			RuntimeMCPContext:       func() (runtimemcp.ReadContext, error) { return readContext, nil },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	importer, ok := service.(core.RuntimeMCPImporter)
	if !ok {
		t.Fatal("settings does not provide runtime MCP copy")
	}
	return runtimeMCPCopyFixture{
		profiles: profiles, db: db,
		root:        root,
		home:        home,
		readContext: readContext,
		inventory:   inventory,
		service:     service,
		importer:    importer,
		secrets:     secrets,
	}
}

// Invariant: the session catalog retains blocked rows while projections remain callable-only and reads never discover. Owner: daemon MCP read model; UT-053, UT-080–082, UT-084, UT-091.
func TestSessionMCPReadModel(t *testing.T) {
	t.Parallel()
	t.Run("Should expose waiting definition and exact approval target", func(t *testing.T) {
		t.Parallel()
		model, _, executor, settings := newSessionMCPReadFixture(t)
		settings.servers = []settingspkg.MCPServerItem{
			{
				Name:             "waiting",
				Owner:            "manual",
				Scope:            settingspkg.ScopeProfile,
				WorkspaceID:      "ws-auth",
				ProfileName:      "work",
				Transport:        "stdio",
				Command:          "npx",
				Args:             []string{"-y", "token=super-secret", "--token", "opaque-credential"},
				URL:              "https://alice:password@example.test/mcp?custom=private-value",
				DefinitionSource: "/project/.compozy/mcp.json",
				SourceMetadata: settingspkg.SourceMetadata{
					EffectiveSource: settingspkg.SourceRef{
						Kind:  settingspkg.SourceKindWorkspaceMCPSidecar,
						Scope: settingspkg.ScopeWorkspace,
					},
				},
				Approval: settingspkg.MCPApprovalState{
					State:            "waiting",
					Reason:           "changed",
					DefinitionDigest: "sha256:definition",
				},
			},
		}
		before := executor.discoveries
		result, err := model.CheckSessionMCP(t.Context(), "sess-1", "waiting", "")
		if err != nil {
			t.Fatal(err)
		}
		row := result.CompozyServers[0]
		if row.State != "waiting_approval" || row.Scope != "profile" || row.WorkspaceID != "ws-auth" ||
			row.Profile != "work" ||
			row.URL != "https://example.test/mcp?custom=%5Bredacted%5D" || strings.Contains(strings.Join(row.Args, " "), "super-secret") ||
			row.Command != "npx" ||
			len(row.Args) != 4 || row.Args[3] != "[REDACTED]" ||
			row.Source.Path != "/project/.compozy/mcp.json" ||
			row.Source.Kind != string(settingspkg.SourceKindWorkspaceMCPSidecar) ||
			row.Approval == nil ||
			row.Approval.Reason != "changed" ||
			row.Approval.DefinitionDigest != "sha256:definition" {
			t.Fatalf("waiting row = %+v", row)
		}
		if executor.discoveries != before {
			t.Fatal("waiting definition was discovered")
		}
	})
	t.Run("Should omit definition previews outside pending approval", func(t *testing.T) {
		t.Parallel()
		model, _, _, settings := newSessionMCPReadFixture(t)
		for i := range settings.servers {
			settings.servers[i].Command = "private-command"
			settings.servers[i].Args = []string{"private-arg"}
			settings.servers[i].URL = "https://secret.test"
			settings.servers[i].DefinitionSource = "/private/path"
		}
		result, err := model.ListSessionMCP(t.Context(), "sess-1")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range result.CompozyServers {
			if row.State != "waiting_approval" &&
				(row.Command != "" || len(row.Args) != 0 || row.URL != "" || row.Source.Path != "") {
				t.Fatalf("leaked preview: %+v", row)
			}
		}
	})
	t.Run("Should return sign-in state without probing", func(t *testing.T) {
		t.Parallel()
		model, _, executor, settings := newSessionMCPReadFixture(t)
		settings.servers[0].AuthStatus = &mcpauth.Status{Status: mcpauth.StatusNeedsLogin}
		before := executor.discoveries
		result, err := model.CheckSessionMCP(t.Context(), "sess-1", settings.servers[0].Name, settings.servers[0].Owner)
		if err != nil {
			t.Fatal(err)
		}
		if executor.discoveries != before {
			t.Fatal("sign-in row was probed")
		}
		if result.CompozyServers[0].State != "needs_sign_in" {
			t.Fatalf("result: %+v", result)
		}
	})
	t.Run("Should check only the requested owner and preserve next-message activation", func(t *testing.T) {
		t.Parallel()
		model, manager, executor, settings := newSessionMCPReadFixture(t)
		settings.servers[0].Name, settings.servers[0].Owner = "docs", "extension:linear"
		settings.servers[1].Name = "docs"
		model.authority = sessionMCPAuthorityStub{servers: []string{"docs"}}
		manager.runtime.Pending = true
		manager.runtime.BoundServers = map[string]int{"sentry": 1}
		executor.fail = true
		failed, err := model.CheckSessionMCP(t.Context(), "sess-1", "docs", "extension:linear")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range failed.CompozyServers {
			if row.Owner == "extension:linear" && (row.State != "failed" || row.CallableCount != 0) {
				t.Fatalf("failed row = %+v", row)
			}
		}
		executor.fail = false
		before := executor.discoveries
		result, err := model.CheckSessionMCP(t.Context(), "sess-1", "docs", "extension:linear")
		if err != nil {
			t.Fatal(err)
		}
		if executor.discoveries != before+1 {
			t.Fatalf("discovery count = %d, want %d", executor.discoveries, before+1)
		}
		var checked session.MCPServerRow
		for _, row := range result.CompozyServers {
			if row.Owner == "extension:linear" {
				checked = row
			}
		}
		if len(manager.changes) != 2 || manager.changes[1] != result.MCPSummary {
			t.Fatalf("summary events = %+v", manager.changes)
		}
		if checked.State != "applies_next_message" || checked.CallableCount != 1 {
			t.Fatalf("checked row = %+v", checked)
		}
		if _, err := model.CheckSessionMCP(
			t.Context(),
			"sess-1",
			"docs",
			"extension:missing",
		); !errors.Is(
			err,
			settingspkg.ErrNotFound,
		) {
			t.Fatalf("unknown owner: %v", err)
		}
		if executor.discoveries != before+1 {
			t.Fatal("unknown owner triggered discovery")
		}
		model.authority = sessionMCPAuthorityStub{}
		if _, err := model.CheckSessionMCP(t.Context(), "sess-1", "docs", "extension:linear"); err != nil {
			t.Fatal(err)
		}
		if executor.discoveries != before+1 {
			t.Fatal("restricted server triggered discovery")
		}
	})

	t.Run("Should never show a runtime server the runtime will not load as loaded", func(t *testing.T) {
		t.Parallel()
		server := func(enabled bool, approval string) runtimemcp.Server {
			return runtimemcp.Server{EnabledInRuntime: enabled, Approval: approval}
		}
		cases := []struct {
			name         string
			server       runtimemcp.Server
			offInCompozy bool
			want         string
		}{
			{name: "enabled", server: server(true, "approved"), want: "loaded_by_runtime"},
			{name: "disabled", server: server(false, ""), want: "off_in_runtime"},
			{name: "rejected", server: server(true, "rejected"), want: "off_in_runtime"},
			{name: "pending", server: server(true, "pending"), want: "waiting_in_runtime"},
			{name: "switch off", server: server(true, ""), offInCompozy: true, want: "off_in_compozy_sessions"},
		}
		for _, tc := range cases {
			if got := runtimeMCPRowState(&tc.server, tc.offInCompozy); got != tc.want {
				t.Errorf("%s: state = %q, want %q", tc.name, got, tc.want)
			}
		}
	})
	for _, userScope := range []bool{false, true} {
		name := "Should read MCP settings for a default-profile workspace session"
		if userScope {
			name = "Should read MCP settings for a default-profile User session"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state, req := newMCPConsentFixture(t)
			model, manager, _, _ := newSessionMCPReadFixture(t)
			workspaceID := req.WorkspaceID
			if userScope {
				workspaceID = ""
			}
			manager.SessionManager = nativeTestSessionManager(workspaceID)
			service, err := settingspkg.NewService(testHomePaths(t), settingspkg.Dependencies{
				WorkspaceResolver: state.workspaceResolver, ProfileResolver: state.profiles,
				AttentionWorkspaceMutes: state.registry.(settingspkg.AttentionWorkspaceMuteStore),
			})
			if err != nil {
				t.Fatal(err)
			}
			model.state.profiles = state.profiles
			model.state.deps.Settings = service
			result, err := model.ListSessionMCP(t.Context(), "sess-1")
			if err != nil || result.SessionID != "sess-1" {
				t.Fatalf("default-profile MCP read = %+v, %v", result, err)
			}
		})
	}

	for _, tc := range []struct {
		name                                           string
		pending, sameCount, restricted, signedOut, off bool
	}{
		{name: "Should retain waiting zero-tool records without discovery UT-080"},
		{name: "Should show pending activation UT-081", pending: true},
		{name: "Should show a schema change with equal tool counts UT-081", pending: true, sameCount: true},
		{name: "Should explain a parent restriction UT-082", restricted: true},
		{name: "Should retain a signed-out only server UT-084", signedOut: true},
		{name: "Should mark runtime servers off UT-053", off: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			model, manager, executor, settings := newSessionMCPReadFixture(t)
			if tc.pending {
				manager.runtime.Pending = true
				manager.runtime.BoundServers = map[string]int{}
				if tc.sameCount {
					manager.runtime.BoundServers = map[string]int{"linear": 1, "sentry": 1}
					manager.runtime.PendingServers = map[string]bool{"linear": true}
				}
			}
			if tc.restricted {
				model.authority = sessionMCPAuthorityStub{servers: []string{"linear"}}
			}
			if tc.signedOut {
				settings.servers = []settingspkg.MCPServerItem{
					{
						Name:        "linear",
						RuntimeName: "linear",
						Owner:       "manual",
						AuthStatus:  &mcpauth.Status{Status: "needs_login"},
					},
				}
			}
			if tc.off {
				manager.runtime.Mode = "off"
				manager.runtime.Inventory.RuntimeMCP.CanDisable = true
				manager.runtime.Inventory.Servers = []runtimemcp.Server{
					{Name: "native", EnabledInRuntime: true, Transport: "stdio", Scope: "user"},
				}
			}
			before := executor.discoveries
			result, err := model.ListSessionMCP(t.Context(), "sess-1")
			if err != nil {
				t.Fatal(err)
			}
			if executor.discoveries != before || !settings.lastRequest.SkipRuntimeStatus {
				t.Fatalf("read discovered or probed: %d -> %d, %+v", before, executor.discoveries, settings.lastRequest)
			}
			rows := map[string]session.MCPServerRow{}
			for _, row := range result.CompozyServers {
				rows[row.Name] = row
			}
			switch {
			case tc.signedOut:
				if result.Usable != 0 || result.Attention != 1 || rows["linear"].State != "needs_sign_in" {
					t.Fatalf("signed out=%+v", result)
				}
			case tc.restricted:
				if rows["sentry"].State != "not_available" || rows["sentry"].Reason != "Limited by the parent session" {
					t.Fatalf("restricted=%+v", rows)
				}
			case tc.off:
				if result.RuntimeServers.Servers[0].State != "off_in_compozy_sessions" {
					t.Fatalf("runtime=%+v", result.RuntimeServers)
				}
			case tc.pending:
				if !result.PendingReload || rows["linear"].State != "applies_next_message" {
					t.Fatalf("pending=%+v", result)
				}
			default:
				if result.Usable != 2 || result.Attention != 1 || rows["linear"].State != "ready" ||
					rows["sentry"].State != "ready" ||
					rows["waiting"].State != "waiting_approval" ||
					rows["waiting"].ToolCount != 0 {
					t.Fatalf("catalog=%+v", result)
				}
			}
		})
	}
	t.Run("Should default the native MCP read to the caller UT-091", func(t *testing.T) {
		t.Parallel()
		model, manager, _, _ := newSessionMCPReadFixture(t)
		manager.reader = model
		registry := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{Sessions: manager},
			nativeApproveAllPolicyInputs(),
		)
		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-auth", ProfileID: store.DefaultProfileID},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDSessionMCPList, Input: json.RawMessage(`{}`)},
		)
		if err != nil {
			t.Fatal(err)
		}
		var payload session.MCP
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.SessionID != "sess-1" || payload.Usable != 2 {
			t.Fatalf("native=%+v", payload)
		}
	})
	// Invariant: poll cadence coalesces changes and equal summaries never notify twice; owner: daemon summary publisher.
	t.Run("Should debounce summary changes per bound session", func(t *testing.T) {
		t.Parallel()
		model, manager, _, settings := newSessionMCPReadFixture(t)
		if err := model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(manager.changes) != 1 {
			t.Fatalf("unchanged notifications=%+v", manager.changes)
		}
		manager.runtime.Pending = true
		manager.runtime.Pending = false // Intermediate state is coalesced before the next cadence.
		if err := model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(manager.changes) != 1 {
			t.Fatal("coalesced changes notified")
		}
		settings.servers = []settingspkg.MCPServerItem{
			{Name: "linear", AuthStatus: &mcpauth.Status{Status: "needs_login"}},
		}
		if err := model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(manager.changes) != 2 || manager.changes[1].Usable != 0 || manager.changes[1].Attention != 1 {
			t.Fatalf("auth notifications=%+v", manager.changes)
		}
	})
	// Invariant: explicit session ids cannot cross the trusted workspace/profile; owner: native adapter.
	for _, scope := range []toolspkg.Scope{{WorkspaceID: "foreign", ProfileID: store.DefaultProfileID}, {WorkspaceID: "ws-auth", ProfileID: "foreign-profile"}} {
		t.Run("Should reject a foreign native session scope "+scope.WorkspaceID+scope.ProfileID, func(t *testing.T) {
			t.Parallel()
			model, manager, _, _ := newSessionMCPReadFixture(t)
			manager.reader = model
			registry := newDaemonNativeRegistry(
				t,
				&daemonNativeToolsDeps{Sessions: manager},
				nativeApproveAllPolicyInputs(),
			)
			_, err := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDSessionMCPList,
					Input:  json.RawMessage(`{"session_id":"sess-1"}`),
				},
			)
			if !errors.Is(err, session.ErrSessionNotFound) {
				t.Fatalf("foreign scope err=%v", err)
			}
		})
	}
}

type sessionMCPAuthorityStub struct{ servers []string }

var _ session.MCPAuthorityResolver = sessionMCPAuthorityStub{}

func (s sessionMCPAuthorityStub) EffectiveMCPAuthority(context.Context, string) (store.SessionPermissionPolicy, error) {
	return store.SessionPermissionPolicy{MCPServers: s.servers}, nil
}

type sessionMCPManagerStub struct {
	core.SessionManager
	runtime session.MCPRuntimeSnapshot
	reader  session.MCPReader
	changes []session.MCPSummary
}

var _ session.MCPReader = (*sessionMCPManagerStub)(nil)

func (s *sessionMCPManagerStub) BoundMCPSessionIDs() []string { return []string{"sess-1"} }
func (s *sessionMCPManagerStub) RecordMCPChanged(_ context.Context, _ string, summary session.MCPSummary) error {
	s.changes = append(s.changes, summary)
	return nil
}

func (s *sessionMCPManagerStub) MCPReadSnapshot(
	context.Context,
	string,
	toolspkg.AdvertisedCatalog,
) (session.MCPRuntimeSnapshot, error) {
	return s.runtime, nil
}
func (s *sessionMCPManagerStub) ListSessionMCP(ctx context.Context, id string) (session.MCP, error) {
	return s.reader.ListSessionMCP(ctx, id)
}

type sessionMCPExecutorStub struct {
	discoveries int
	fail        bool
}

var _ toolspkg.MCPCallExecutor = (*sessionMCPExecutorStub)(nil)

func (s *sessionMCPExecutorStub) ListTools(context.Context, toolspkg.SourceRef) ([]toolspkg.MCPToolDescriptor, error) {
	s.discoveries++
	if s.fail {
		return nil, errors.New("MCP server unavailable")
	}
	return []toolspkg.MCPToolDescriptor{
		{RawName: "echo", ReadOnly: true, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}, nil
}

func (*sessionMCPExecutorStub) CallTool(
	context.Context,
	toolspkg.SourceRef,
	toolspkg.MCPToolCallRequest,
) (toolspkg.ToolResult, error) {
	return toolspkg.ToolResult{}, errors.New("unexpected call")
}

func newSessionMCPReadFixture(
	t *testing.T,
) (*sessionMCPReadModel, *sessionMCPManagerStub, *sessionMCPExecutorStub, *nativeMCPSettingsService) {
	t.Helper()
	executor := &sessionMCPExecutorStub{}
	sources := []toolspkg.SourceRef{}
	for _, name := range []string{"linear", "sentry", "waiting"} {
		sources = append(
			sources,
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: name, RawServerName: name, Scope: "user"},
		)
	}
	provider, err := toolspkg.NewMCPProvider(
		toolspkg.MCPSourceListerFunc(func(context.Context) ([]toolspkg.SourceRef, error) { return sources, nil }),
		executor,
		nil,
		toolspkg.WithMCPConsent(func(_ context.Context, source toolspkg.SourceRef, _ string) (bool, error) {
			return source.Owner != "waiting", nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	inputs := nativeApproveAllPolicyInputs()
	inputs.ExternalDefault = toolspkg.ExternalDefaultEnabled
	registry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(inputs, toolspkg.ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	scope := toolspkg.Scope{
		SessionID:   "sess-1",
		ProfileID:   store.DefaultProfileID,
		WorkspaceID: "ws-auth",
		AgentName:   "coder",
	}
	views, err := registry.SessionProjection(t.Context(), scope)
	if err != nil || len(views) != 2 {
		t.Fatalf("warm projection=%+v, %v", views, err)
	}
	settings := &nativeMCPSettingsService{servers: []settingspkg.MCPServerItem{
		{
			Name:        "linear",
			RuntimeName: "linear",
			Owner:       "manual",
		},
		{Name: "sentry", RuntimeName: "sentry", Owner: "manual"},
		{
			Name:        "waiting",
			RuntimeName: "waiting",
			Owner:       "manual",
			Approval:    settingspkg.MCPApprovalState{State: "waiting"},
		},
	}}
	manager := &sessionMCPManagerStub{
		SessionManager: nativeTestSessionManager("ws-auth"),
		runtime: session.MCPRuntimeSnapshot{
			Mode:         "load",
			Bound:        true,
			BoundServers: map[string]int{"linear": 1, "sentry": 1},
			Inventory:    runtimemcp.Snapshot{Runtime: "claude"},
		},
	}
	state := &bootState{sessions: manager, mcpToolProvider: provider}
	state.deps.Settings = settings
	model := &sessionMCPReadModel{
		state:     state,
		authority: sessionMCPAuthorityStub{servers: []string{"linear", "sentry"}},
		registry:  registry,
		summaries: map[string]session.MCPSummary{},
	}
	manager.reader = model
	return model, manager, executor, settings
}

func newTestRuntimeMCPInventory(t *testing.T) *runtimeMCPInventory {
	t.Helper()
	inventory, err := newRuntimeMCPInventory(vault.NewMCPDigestKeyProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
