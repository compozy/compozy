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

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
	t.Run("Should keep the revision stable when multiple unchanged contexts expire together", func(t *testing.T) {
		t.Parallel()
		inventory := newTestRuntimeMCPInventory(t)
		now := time.Now()
		inventory.now = func() time.Time { return now }
		root := t.TempDir()
		requests := []runtimemcp.Request{
			{
				Runtime: "claude",
				Context: runtimemcp.ReadContext{Home: root, SystemRoot: root, CWD: filepath.Join(root, "one")},
			},
			{
				Runtime: "claude",
				Context: runtimemcp.ReadContext{Home: root, SystemRoot: root, CWD: filepath.Join(root, "two")},
			},
		}
		for _, request := range requests {
			if _, err := inventory.List(t.Context(), request); err != nil {
				t.Fatal(err)
			}
		}
		before := inventory.Revision()
		now = now.Add(runtimeMCPCacheTTL)
		inventory.Poll()
		for _, request := range requests {
			if _, err := inventory.List(t.Context(), request); err != nil {
				t.Fatal(err)
			}
		}
		if after := inventory.Revision(); after != before {
			t.Fatalf("unchanged inventory revision = %d, want %d", after, before)
		}
	})
	// Invariant: observing a second context cannot hide the first dependency.
	// Owner: runtime inventory; canonical suite: runtime MCP inventory.
	t.Run("Should reject a dependency observation spanning two runtime contexts", func(t *testing.T) {
		t.Parallel()
		inventory := newTestRuntimeMCPInventory(t)
		var observation runtimeMCPObservation
		ctx := context.WithValue(t.Context(), runtimeMCPObservationKey{}, &observation)
		root := t.TempDir()
		req := runtimemcp.Request{Runtime: "claude", Context: runtimemcp.ReadContext{
			Home: root, SystemRoot: root, CWD: filepath.Join(root, "first"),
		}}
		for range 2 {
			if _, err := inventory.List(ctx, req); err != nil {
				t.Fatal(err)
			}
		}
		if !inventory.matches(observation) {
			t.Fatal("repeated reads of one context should remain cacheable")
		}
		req.Context.CWD = filepath.Join(root, "second")
		if _, err := inventory.List(ctx, req); err != nil {
			t.Fatal(err)
		}
		if inventory.matches(observation) {
			t.Fatal("the second runtime context hid the first dependency")
		}
	})
	// Invariant: cache expiry alone does not publish an inventory change; file edits still do.
	// Owner: daemon runtime inventory; canonical suite: runtime MCP inventory.
	t.Run("Should retain the revision across an unchanged TTL refresh", func(t *testing.T) {
		t.Parallel()
		inventory := newTestRuntimeMCPInventory(t)
		now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
		inventory.now = func() time.Time { return now }
		root := t.TempDir()
		request := runtimemcp.Request{
			Runtime: "claude", Context: runtimemcp.ReadContext{Home: root, SystemRoot: filepath.Join(root, "system")},
		}
		if _, err := inventory.List(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		revision := inventory.Revision()
		now = now.Add(runtimeMCPCacheTTL + time.Second)
		inventory.Poll()
		if inventory.Revision() != revision {
			t.Fatal("expiry published an inventory change")
		}
		if _, err := inventory.List(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		if inventory.Revision() != revision {
			t.Fatal("unchanged refresh published an inventory change")
		}
		if err := os.WriteFile(filepath.Join(root, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		inventory.Poll()
		if inventory.Revision() <= revision {
			t.Fatal("file creation did not publish an inventory change")
		}
	})

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
	// Invariant: idle instance and triage reads reuse both projections and changes
	// invalidate their owners. Owner: daemon MCP projection; canonical suite: session MCP reads.
	t.Run("Should refresh only the bound instance policy that changed", func(t *testing.T) {
		t.Parallel()
		fixture := newInstanceMCPProjectionFixture(t, 4, 3, 2, false)
		if err := fixture.model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		tick := func(want ...string) {
			t.Helper()
			clear(fixture.policyReads)
			clear(fixture.sessions.reads)
			if err := fixture.model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			for _, id := range fixture.ids {
				if got := fixture.sessions.reads[id]; (got != 0) != slices.Contains(want, id) {
					t.Fatalf("summary reads = %v, changed sessions = %v", fixture.sessions.reads, want)
				}
			}
			clear(fixture.policyReads)
			fixture.now = fixture.now.Add(fixture.hosted.StreamRefreshInterval())
			for i, bind := range fixture.binds {
				result, err := fixture.hosted.StreamProjection(t.Context(), bind, fixture.peer, fixture.digests[i])
				if err != nil {
					t.Fatal(err)
				}
				fixture.digests[i] = result.Digest
			}
			for _, id := range fixture.ids {
				if got := fixture.policyReads[id]; (got != 0) != slices.Contains(want, id) {
					t.Fatalf("hosted policy reads = %v, changed sessions = %v", fixture.policyReads, want)
				}
			}
		}
		tick()
		repo := fixture.db.Instances()
		if _, err := repo.Pause(t.Context(), fixture.instanceIDs[1], "manual"); err != nil {
			t.Fatal(err)
		}
		tick(fixture.ids[1])
		if _, err := repo.Resume(t.Context(), fixture.instanceIDs[1], 5, true); err != nil {
			t.Fatal(err)
		}
		tick(fixture.ids[1])
		if _, err := repo.MarkSeen(
			t.Context(),
			fixture.instanceIDs[1],
			agentinstance.LocalOperator,
			fixture.now,
		); err != nil {
			t.Fatal(err)
		}
		tick()
		const writable = "mcp__fixture__tool_000"
		assertWritable := func(expected bool) {
			t.Helper()
			current, err := fixture.hosted.StreamSnapshot(t.Context(), fixture.binds[2], fixture.peer)
			if err != nil {
				t.Fatal(err)
			}
			exposed := slices.ContainsFunc(current.Tools, func(view toolspkg.ToolView) bool {
				return view.Descriptor.ID.String() == writable
			})
			if exposed != expected {
				views, err := fixture.model.registry.DiagnosticProjection(t.Context(), toolspkg.Scope{
					SessionID: fixture.ids[2], AgentName: "fixture-0",
					ProfileID: store.DefaultProfileID, WorkspaceID: "ws-instance-perf",
				})
				if err != nil {
					t.Fatal(err)
				}
				for _, view := range views {
					if view.Descriptor.ID.String() == writable {
						t.Logf("check-in write decision: %+v", view.Decision)
					}
				}
				t.Fatalf("check-in writable tool listed = %v, want %v", exposed, expected)
			}
		}
		assertWritable(false)
		dir := filepath.Join(fixture.agentRoot, "fixture-0")
		content := []byte("---\nversion: 1\ncheck_ins:\n  allow_writes: [mcp__fixture__tool_000]\n---\n")
		if err := os.WriteFile(filepath.Join(dir, "HEARTBEAT.md"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		tick(fixture.ids[2])
		canonicalDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := heartbeat.AuthorityDigest(heartbeat.CheckInsPolicy{AllowWrites: []string{writable}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		noticed, err := repo.MarkAuthorityNotice(t.Context(), store.DefaultProfileID, canonicalDir, digest)
		if err != nil || !noticed {
			t.Fatalf("record authority notice = %v, %v", noticed, err)
		}
		tick()
		if err := repo.ApproveAuthority(
			t.Context(),
			store.DefaultProfileID,
			canonicalDir,
			digest,
			"http",
			fixture.now,
		); err != nil {
			t.Fatal(err)
		}
		tick(fixture.ids[2])
		assertWritable(true)
		if err := repo.RevokeAuthority(t.Context(), store.DefaultProfileID, canonicalDir); err != nil {
			t.Fatal(err)
		}
		tick(fixture.ids[2])
		assertWritable(false)
		tick()
	})
	// Invariant: durable adoption and archive invalidate warm hosted binds for
	// the adopted root and its descendants. Owner: daemon MCP policy projection.
	t.Run("Should observe instance adoption and archive across an open lineage", func(t *testing.T) {
		t.Parallel()
		fixture := newInstanceMCPProjectionFixture(t, 3, 0, 2, true)
		root, descendant, unrelated := fixture.ids[0], fixture.ids[1], fixture.ids[2]
		info, err := fixture.sessions.StoredStatus(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		wake := func(want bool) {
			t.Helper()
			clear(fixture.policyReads)
			fixture.now = fixture.now.Add(fixture.hosted.StreamRefreshInterval())
			for i, bind := range fixture.binds {
				result, err := fixture.hosted.StreamProjection(t.Context(), bind, fixture.peer, fixture.digests[i])
				if err != nil {
					t.Fatal(err)
				}
				fixture.digests[i] = result.Digest
			}
			if (fixture.policyReads[root] != 0) != want || (fixture.policyReads[descendant] != 0) != want ||
				fixture.policyReads[unrelated] != 0 {
				t.Fatalf("want changed lineage=%v, policy reads=%v", want, fixture.policyReads)
			}
		}
		assertApproval := func(bind int, expected bool) {
			t.Helper()
			current, err := fixture.hosted.StreamSnapshot(t.Context(), fixture.binds[bind], fixture.peer)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []toolspkg.ToolID{toolspkg.ToolIDSessionApprove, toolspkg.ToolIDSessionClarifyAnswer} {
				index := slices.IndexFunc(current.Tools, func(view toolspkg.ToolView) bool {
					return view.Descriptor.ID == id
				})
				if index < 0 || current.Tools[index].Decision.ApprovalRequired != expected {
					t.Fatalf("tool %s approval requirement want=%v, present=%v", id, expected, index >= 0)
				}
			}
		}
		wake(false)
		assertApproval(0, true)
		assertApproval(1, true)
		adopted, err := fixture.db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID, AgentName: info.AgentName,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, root, 5)
		if err != nil {
			t.Fatal(err)
		}
		wake(true)
		assertApproval(0, false)
		assertApproval(1, false)
		assertApproval(2, true)
		wake(false)
		if _, err := fixture.db.Instances().Archive(t.Context(), adopted.ID); err != nil {
			t.Fatal(err)
		}
		wake(true)
		assertApproval(0, false)
		assertApproval(1, false)
		wake(false)
		// A direct committed write bypasses the daemon repository's mutation path.
		if _, err := fixture.db.DB().ExecContext(
			t.Context(), "UPDATE sessions SET instance_id = NULL WHERE id = ?", root,
		); err != nil {
			t.Fatal(err)
		}
		wake(true)
		assertApproval(0, true)
		assertApproval(1, true)
		assertApproval(2, true)
		wake(false)
	})

	// Invariant: bound-manager summaries record one durable event per changed result.
	// Owner: daemon MCP summary; canonical suite: session MCP reads.
	t.Run("Should publish bound summaries after policy auth catalog and redaction changes", func(t *testing.T) {
		fixture := newInstanceMCPProjectionFixture(t, 1, 0, 1, false)
		ctx := t.Context()
		id := fixture.ids[0]
		active, found := fixture.sessions.Get(id)
		if !found {
			t.Fatal("fixture did not bind a session")
		}
		info, err := fixture.sessions.StoredStatus(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		scope := toolspkg.Scope{
			SessionID:   id,
			ProfileID:   info.ProfileID,
			WorkspaceID: info.WorkspaceID,
			AgentName:   info.AgentName,
		}
		query := store.EventQuery{Type: session.EventTypeMCPChanged, Limit: 50}
		eventCount := 0
		publish := func(label string) {
			t.Helper()
			eventCount++
			for range 2 {
				if err := fixture.model.refresh(ctx); err != nil {
					t.Fatal(err)
				}
			}
			events, err := fixture.sessions.Events(ctx, id, query)
			if err != nil || len(events) != eventCount {
				t.Fatalf("%s: durable summary events = %d, want %d, error=%v", label, len(events), eventCount, err)
			}
		}
		publish("initial and idle")
		assertIdle := func() {
			t.Helper()
			reads := fixture.sessions.reads[id]
			if err := fixture.model.refresh(ctx); err != nil {
				t.Fatal(err)
			}
			if got := fixture.sessions.reads[id]; got != reads {
				t.Fatalf("idle summary rebuilt: reads %d → %d", reads, got)
			}
		}
		assertIdle()

		agentFile := filepath.Join(fixture.agentRoot, info.AgentName, "AGENT.md")
		writePolicy := func(deny string) {
			t.Helper()
			body := fmt.Sprintf("---\nname: %s\nprovider: acpmock\nmodel: fixture\n%s---\n\nFixture agent.\n",
				info.AgentName, deny)
			if err := os.WriteFile(agentFile, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			fixture.model.state.workspaceResolver.Invalidate(info.WorkspaceID)
		}
		writePolicy("deny_tools: [mcp__fixture__tool_000]\n")
		publish("deny_tools")
		writePolicy("")
		publish("agent policy restored")

		permissions := active.EffectivePermissions
		active.EffectivePermissions = string(compozyconfig.PermissionModeDenyAll)
		publish("permission mode")
		active.EffectivePermissions = permissions
		publish("permission mode restored")

		lineage := active.Lineage
		limited := store.CloneSessionLineage(store.NormalizeSessionLineage(id, lineage))
		limited.PermissionPolicy.MCPServersMode = store.PermissionList
		limited.PermissionPolicy.MCPServers = nil
		active.Lineage = limited
		publish("lineage ceiling")
		active.Lineage = lineage
		publish("lineage restored")

		target := mcpauth.Target{ServerName: "fixture", Owner: "manual", Scope: mcpauth.ScopeUser}
		setAuth := func(status string) {
			t.Helper()
			fixture.auth.status = status
			fixture.settings.servers[0].AuthStatus = &mcpauth.Status{Status: mcpauth.StatusValue(status)}
			if _, err := fixture.model.state.mcpAuthGeneration.Advance(target); err != nil {
				t.Fatal(err)
			}
		}
		setAuth("needs_login")
		publish("auth revision")
		setAuth("unconfigured")
		publish("auth restored")

		descriptors := fixture.executor.descriptors
		refreshCatalog := func(entries []toolspkg.MCPToolDescriptor) {
			t.Helper()
			fixture.executor.descriptors = entries
			if _, err := fixture.provider.List(ctx, scope); err != nil {
				t.Fatal(err)
			}
		}
		refreshCatalog([]toolspkg.MCPToolDescriptor{})
		publish("catalog removed")
		refreshCatalog(descriptors)
		publish("catalog restored")

		release := redact.RegisterRequiredSecret("mcp__fixture__tool_000")
		t.Cleanup(release)
		publish("redaction revision")
		assertIdle()
	})

	// Invariant: a runtime-owned file change rebuilds only dependencies of that file and publishes once.
	// Owner: daemon session MCP summaries; canonical suite: session MCP reads.
	t.Run("Should rebuild only the session whose runtime MCP file changed", func(t *testing.T) {
		t.Parallel()
		model, manager, _, _ := newSessionMCPReadFixture(t)
		inventory := newTestRuntimeMCPInventory(t)
		root := t.TempDir()
		manager.boundIDs = []string{"sess-1", "sess-2"}
		model.state.runtimeMCPInventory = inventory
		model.state.sessions = &inventorySessionMCPManager{
			sessionMCPManagerStub: manager,
			inventory:             inventory,
			root:                  root,
		}

		for range 2 {
			if err := model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		reads := manager.reads
		if len(manager.changes) != 2 {
			t.Fatalf("initial summaries = %v", manager.changes)
		}
		dir := filepath.Join(root, "sess-2")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, ".claude.json"),
			[]byte(`{"mcpServers":{"local":{"command":"echo"}}}`),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		inventory.Poll()
		for range 2 {
			if err := model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if manager.reads != reads+1 || len(manager.changes) != 3 || manager.changes[2].Usable != 3 {
			t.Fatalf("runtime edit: projections before=%d after=%d, changes=%v", reads, manager.reads, manager.changes)
		}
	})
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
		if len(manager.changes) != 1 || manager.reads != 1 {
			t.Fatalf("unchanged notifications=%+v, projection reads=%d", manager.changes, manager.reads)
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
	// Invariant: runtime activation and authority fences invalidate otherwise equal summaries.
	// Owner: daemon summary read model; canonical suite: session MCP reads.
	t.Run("Should refresh summaries after pending reload, lineage and epoch changes", func(t *testing.T) {
		t.Parallel()
		model, manager, _, _ := newSessionMCPReadFixture(t)
		info, err := manager.Status(t.Context(), "sess-1")
		if err != nil {
			t.Fatal(err)
		}
		manager.SessionManager = &apitest.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return info, nil
			},
		}
		model.state.toolProjectionEpoch = mcppkg.NewProjectionEpoch()
		for index, change := range []func(){
			func() {},
			func() { manager.runtime.Pending = true },
			func() { info.EffectivePermissions = "deny-all" },
			func() { info.Lineage = &store.SessionLineage{ParentSessionID: "parent"} },
			func() { model.state.toolProjectionEpoch.Advance() },
		} {
			change()
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); err != nil {
				t.Fatal(err)
			}
			if manager.reads != index+1 {
				t.Fatalf("projection reads after change %d = %d", index, manager.reads)
			}
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); err != nil {
				t.Fatal(err)
			}
			if manager.reads != index+1 {
				t.Fatal("unchanged summary rebuilt")
			}
		}
	})
	// Invariant: later sessions never reuse discovery from before an epoch or policy-only withdrawal.
	// Owner: daemon summary cache; canonical suite: session MCP reads.
	for _, policyOnly := range []bool{false, true} {
		name := "Should isolate refresh reads across an epoch change"
		if policyOnly {
			name = "Should invalidate summaries across a policy-only withdrawal"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			model, manager, executor, _ := newSessionMCPReadFixture(t)
			model.state.toolProjectionEpoch = mcppkg.NewProjectionEpoch()
			home := testHomePaths(t)
			resolver, err := workspacepkg.NewResolver(&recordingRegistry{}, workspacepkg.WithHomePaths(home))
			if err != nil {
				t.Fatal(err)
			}
			model.state.workspaceResolver = resolver
			withdrawn := false
			provider, err := toolspkg.NewMCPProvider(
				toolspkg.MCPSourceListerFunc(func(context.Context) ([]toolspkg.SourceRef, error) {
					if withdrawn {
						return nil, nil
					}
					return []toolspkg.SourceRef{
						{Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear", Scope: "user"},
						{Kind: toolspkg.SourceMCP, Owner: "sentry", RawServerName: "sentry", Scope: "user"},
					}, nil
				}),
				executor,
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			inputs := nativeApproveAllPolicyInputs()
			inputs.ExternalDefault = toolspkg.ExternalDefaultEnabled
			model.registry, err = toolspkg.NewRegistry(
				toolspkg.WithProviders(provider),
				toolspkg.WithPolicyInputs(inputs, toolspkg.ToolsetCatalog{}),
			)
			if err != nil {
				t.Fatal(err)
			}
			views, err := model.registry.SessionProjection(t.Context(), toolspkg.Scope{
				SessionID: "sess-1", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-auth", AgentName: "coder",
			})
			if err != nil || len(views) != 2 {
				t.Fatalf("warm replacement projection = %+v, %v", views, err)
			}
			info, err := manager.Status(t.Context(), "sess-1")
			if err != nil {
				t.Fatal(err)
			}
			manager.SessionManager = &apitest.StubSessionManager{
				StatusFn: func(_ context.Context, id string) (*session.Info, error) {
					current := *info
					current.ID = id
					return &current, nil
				},
			}
			manager.boundIDs = []string{"sess-1", "sess-2"}
			manager.onRecord = func(id string) {
				if id != "sess-1" {
					return
				}
				withdrawn = true
				if policyOnly {
					resolver.InvalidateAll()
				} else {
					model.state.toolProjectionEpoch.Advance()
				}
			}
			if err := model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(manager.changes) != 2 || manager.changes[0].Usable != 2 || manager.changes[1].Usable != 0 {
				t.Fatalf("summaries across withdrawal = %+v", manager.changes)
			}
			reads := manager.reads
			summary, err := model.SessionMCPSummary(t.Context(), "sess-2")
			if err != nil || summary == nil || summary.Usable != 0 || manager.reads != reads {
				t.Fatalf(
					"post-withdrawal cached summary = %+v, %v; reads=%d, want %d",
					summary,
					err,
					manager.reads,
					reads,
				)
			}
		})
	}
	// Invariant: status cannot seed a stale summary after an authored policy withdrawal.
	// Owner: daemon summary read model; canonical suite: session MCP reads.
	t.Run("Should use current agent policy after status warms the shared context", func(t *testing.T) {
		t.Parallel()
		model, manager, _, _ := newSessionMCPReadFixture(t)
		home := testHomePaths(t)
		cfg := testConfig(t, home)
		cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
		cfg.Tools.Policy.ExternalDefault = compozyconfig.ToolsExternalDefaultEnabled
		cfg.MCPServers = []compozyconfig.MCPServer{
			{Name: "linear", Command: "fixture"},
			{Name: "sentry", Command: "fixture"},
		}
		workspace := workspacepkg.Workspace{ID: "ws-auth", Name: "project", RootDir: t.TempDir()}
		workspaceStore := openDaemonTestGlobalDB(t)
		if err := workspaceStore.InsertWorkspace(t.Context(), workspace); err != nil {
			t.Fatal(err)
		}
		resolver, err := workspacepkg.NewResolver(
			workspaceStore,
			workspacepkg.WithHomePaths(home),
			workspacepkg.WithProfileConfigLoader(
				func(string, string) (compozyconfig.Config, error) { return cfg, nil },
			),
		)
		if err != nil {
			t.Fatal(err)
		}
		agentFile := filepath.Join(home.AgentsDir, "coder", "AGENT.md")
		if err := os.MkdirAll(filepath.Dir(agentFile), 0o755); err != nil {
			t.Fatal(err)
		}
		writeAgent := func(policy string) {
			t.Helper()
			data := []byte("---\nname: coder\nprovider: codex\n" + policy + "---\n\nYou are coder.\n")
			if err := os.WriteFile(agentFile, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		writeAgent("")
		model.state.cfg = cfg
		model.state.workspaceResolver = resolver
		model.state.profiles, err = profilepkg.NewManager(
			profilepkg.WithStore(workspaceStore),
			profilepkg.WithHomePaths(home),
		)
		if err != nil {
			t.Fatal(err)
		}
		authority, err := newNativeToolPolicyResolverForBoot(model.state)
		if err != nil {
			t.Fatal(err)
		}
		model.authority = authority
		provider, ok := model.state.mcpToolProvider.(*toolspkg.MCPProvider)
		if !ok {
			t.Fatal("fixture has no MCP provider")
		}
		model.registry, err = toolspkg.NewRegistry(
			toolspkg.WithProviders(provider),
			toolspkg.WithPolicyInputResolver(authority, toolspkg.ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatal(err)
		}
		authority.scopeTools = model.registry.PolicyProjection
		ctx := model.registry.ReadContext(
			toolspkg.EnsureProjectionListings(toolspkg.WithCachedMCPProjection(t.Context())),
		)
		info, err := manager.Status(ctx, "sess-1")
		if err != nil {
			t.Fatal(err)
		}
		if err := session.PopulateEffectiveMCPAuthority(ctx, authority, info); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(info.Lineage.EffectiveMCPServers, []string{"linear", "sentry"}) {
			t.Fatalf("status authority = %v", info.Lineage.EffectiveMCPServers)
		}
		warm, err := model.SessionMCPSummary(ctx, info.ID)
		if err != nil || warm == nil || warm.Usable != 2 {
			t.Fatalf("summary before policy withdrawal = %+v, %v", warm, err)
		}
		writeAgent("deny_tools: [mcp__linear__echo]\n")
		resolver.Invalidate(workspace.ID)
		current, err := model.SessionMCPSummary(ctx, info.ID)
		if err != nil || current == nil || current.Usable != 1 {
			t.Errorf("summary on the shared status context = %+v, %v, want one usable server", current, err)
		}
		reads := manager.reads
		current, err = model.SessionMCPSummary(t.Context(), info.ID)
		if err != nil || current == nil || current.Usable != 1 || manager.reads != reads {
			t.Errorf("cached summary = %+v, %v, reads = %d, want one usable server and %d reads",
				current, err, manager.reads, reads)
		}
	})
	// Invariant: detail-only snapshots stay warm while read and leave the cache after inactivity or deletion.
	// Owner: daemon summary cache; canonical suite: session MCP reads.
	t.Run(
		"Should evict idle and deleted detail-only sessions independently of published summaries",
		func(t *testing.T) {
			t.Parallel()
			model, manager, _, _ := newSessionMCPReadFixture(t)
			manager.boundIDs = []string{}
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); err != nil {
				t.Fatal(err)
			}
			if err := model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); err != nil {
				t.Fatal(err)
			}
			if manager.reads != 1 {
				t.Fatalf("active unbound session rebuilt %d times", manager.reads)
			}
			model.mu.Lock()
			snapshot := model.snapshots["sess-1"]
			snapshot.lastRead = time.Now().Add(-model.summaryIdleTTL())
			model.snapshots["sess-1"] = snapshot
			model.mu.Unlock()
			if err := model.refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); err != nil {
				t.Fatal(err)
			}
			if manager.reads != 2 {
				t.Fatalf("idle snapshot survived eviction: reads=%d", manager.reads)
			}
			manager.SessionManager = &apitest.StubSessionManager{
				StatusFn: func(context.Context, string) (*session.Info, error) { return nil, session.ErrSessionNotFound },
			}
			if _, err := model.SessionMCPSummary(t.Context(), "sess-1"); !errors.Is(err, session.ErrSessionNotFound) {
				t.Fatalf("deleted session error = %v", err)
			}
			if len(model.snapshots) != 0 {
				t.Fatalf("deleted snapshots retained: %d", len(model.snapshots))
			}
		},
	)
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
	runtime  session.MCPRuntimeSnapshot
	reader   session.MCPReader
	changes  []session.MCPSummary
	boundIDs []string
	onRecord func(string)
	reads    int
}

var _ session.MCPReader = (*sessionMCPManagerStub)(nil)

func (s *sessionMCPManagerStub) BoundMCPSessionIDs() []string {
	if s.boundIDs != nil {
		return s.boundIDs
	}
	return []string{"sess-1"}
}
func (s *sessionMCPManagerStub) RecordMCPChanged(_ context.Context, id string, summary session.MCPSummary) error {
	s.changes = append(s.changes, summary)
	if s.onRecord != nil {
		s.onRecord(id)
	}
	return nil
}

func (s *sessionMCPManagerStub) MCPReadSnapshot(
	context.Context,
	string,
	toolspkg.AdvertisedCatalog,
) (session.MCPRuntimeSnapshot, error) {
	s.reads++
	return s.runtime, nil
}
func (s *sessionMCPManagerStub) MCPReadIdentity(string) (string, error) {
	data, err := json.Marshal(s.runtime)
	return string(data), err
}
func (s *sessionMCPManagerStub) ListSessionMCP(ctx context.Context, id string) (session.MCP, error) {
	return s.reader.ListSessionMCP(ctx, id)
}

type sessionMCPExecutorStub struct {
	discoveries int
	fail        bool
	descriptors []toolspkg.MCPToolDescriptor
}

var _ toolspkg.MCPCallExecutor = (*sessionMCPExecutorStub)(nil)

func (s *sessionMCPExecutorStub) ListTools(context.Context, toolspkg.SourceRef) ([]toolspkg.MCPToolDescriptor, error) {
	s.discoveries++
	if s.fail {
		return nil, errors.New("MCP server unavailable")
	}
	if s.descriptors != nil {
		return s.descriptors, nil
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
	t testing.TB,
) (*sessionMCPReadModel, *sessionMCPManagerStub, *sessionMCPExecutorStub, *nativeMCPSettingsService) {
	return newSessionMCPReadFixtureWithExecutor(t, &sessionMCPExecutorStub{}, 2)
}

func newSessionMCPReadFixtureWithExecutor(
	t testing.TB, executor *sessionMCPExecutorStub, toolCount int,
) (*sessionMCPReadModel, *sessionMCPManagerStub, *sessionMCPExecutorStub, *nativeMCPSettingsService) {
	t.Helper()
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
	if err != nil || len(views) != toolCount {
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

type inventorySessionMCPManager struct {
	*sessionMCPManagerStub
	inventory *runtimeMCPInventory
	root      string
}

func (m *inventorySessionMCPManager) MCPReadSnapshot(
	ctx context.Context, id string, catalog toolspkg.AdvertisedCatalog,
) (session.MCPRuntimeSnapshot, error) {
	inventory, err := m.inventory.List(ctx, runtimemcp.Request{
		Runtime: "claude",
		Context: runtimemcp.ReadContext{
			Home:       filepath.Join(m.root, id),
			SystemRoot: m.root,
			CWD:        filepath.Join(m.root, id),
		},
	})
	if err != nil {
		return session.MCPRuntimeSnapshot{}, err
	}
	result, err := m.sessionMCPManagerStub.MCPReadSnapshot(ctx, id, catalog)
	if len(inventory.Runtimes) != 0 {
		result.Inventory = inventory.Runtimes[0]
	}
	return result, err
}

func newTestRuntimeMCPInventory(t testing.TB) *runtimeMCPInventory {
	t.Helper()
	inventory, err := newRuntimeMCPInventory(vault.NewMCPDigestKeyProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
