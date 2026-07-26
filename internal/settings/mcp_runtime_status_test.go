package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestMCPServerItemsIncludeRuntimeStatusAndRemainIsolated(t *testing.T) {
	t.Run("Should attach daemon-backed runtime status to configured MCP servers", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]",
			"name = \"ready-docs\"",
			"command = \"docs-mcp\"",
			"",
			"[[mcp_servers]]",
			"name = \"linear\"",
			"transport = \"http\"",
			"url = \"https://mcp.linear.example/mcp\"",
			"",
			"[mcp_servers.auth]",
			"type = \"oauth2_pkce\"",
			"authorization_url = \"https://auth.linear.example/authorize\"",
			"token_url = \"https://auth.linear.example/token\"",
			"client_id = \"agh-desktop\"",
		}, "\n"))
		runtime := &fakeMCPRuntimeProvider{
			statuses: map[string]MCPServerRuntimeStatus{
				"ready-docs": {
					Configured:  true,
					Initialized: true,
					State:       MCPServerRuntimeStateReady,
					Probe:       MCPServerProbeSucceeded,
					ToolCount:   2,
				},
				"linear": {
					Configured: true,
					State:      MCPServerRuntimeStateAuthRequired,
					Probe:      MCPServerProbeSkipped,
					Reason:     "mcp_auth_required",
				},
			},
		}
		service := testService(t, homePaths, Dependencies{MCPRuntime: runtime})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(mcp) error = %v", err)
		}

		ready := findMCPItem(t, envelope.MCPServers, "ready-docs")
		if ready.RuntimeStatus == nil {
			t.Fatal("ready-docs RuntimeStatus = nil, want probe status")
		}
		if got, want := ready.RuntimeStatus.State, MCPServerRuntimeStateReady; got != want {
			t.Fatalf("ready-docs RuntimeStatus.State = %q, want %q", got, want)
		}
		if !ready.RuntimeStatus.Initialized || ready.RuntimeStatus.ToolCount != 2 {
			t.Fatalf("ready-docs RuntimeStatus = %#v, want initialized with 2 tools", ready.RuntimeStatus)
		}

		linear := findMCPItem(t, envelope.MCPServers, "linear")
		if linear.RuntimeStatus == nil {
			t.Fatal("linear RuntimeStatus = nil, want auth-blocked status")
		}
		if got, want := linear.RuntimeStatus.State, MCPServerRuntimeStateAuthRequired; got != want {
			t.Fatalf("linear RuntimeStatus.State = %q, want %q", got, want)
		}
		if got, want := linear.RuntimeStatus.Probe, MCPServerProbeSkipped; got != want {
			t.Fatalf("linear RuntimeStatus.Probe = %q, want %q", got, want)
		}
		if got, want := linear.RuntimeStatus.Reason, "mcp_auth_required"; got != want {
			t.Fatalf("linear RuntimeStatus.Reason = %q, want %q", got, want)
		}
	})

	t.Run("Should deep-clone every mutable MCP item field", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
		updatedAt := expiresAt.Add(-time.Minute)
		source := MCPServerItem{
			Args:          []string{"serve"},
			EnvKeys:       []string{"PROJECT"},
			SecretEnvKeys: []string{"TOKEN"},
			Auth: aghconfig.MCPAuthConfig{
				Scopes: []string{"read"},
			},
			AuthStatus: &mcpauth.Status{
				Scopes:    []string{"read"},
				ExpiresAt: &expiresAt,
				UpdatedAt: &updatedAt,
			},
			RuntimeStatus: &MCPServerRuntimeStatus{Reason: "ready"},
			SourceMetadata: SourceMetadata{
				ShadowedSources:  []SourceRef{{Kind: SourceKindGlobalConfig}},
				AvailableTargets: []WriteTargetKind{WriteTargetGlobalConfig},
			},
		}

		cloned := cloneMCPServerItem(source)
		cloned.Args[0] = "changed"
		cloned.EnvKeys[0] = "changed"
		cloned.SecretEnvKeys[0] = "changed"
		cloned.Auth.Scopes[0] = "changed"
		cloned.AuthStatus.Scopes[0] = "changed"
		*cloned.AuthStatus.ExpiresAt = time.Time{}
		*cloned.AuthStatus.UpdatedAt = time.Time{}
		cloned.RuntimeStatus.Reason = "changed"
		cloned.SourceMetadata.ShadowedSources[0].Kind = SourceKindGlobalMCPSidecar
		cloned.SourceMetadata.AvailableTargets[0] = WriteTargetGlobalMCPSidecar

		if source.Args[0] != "serve" || source.EnvKeys[0] != "PROJECT" ||
			source.SecretEnvKeys[0] != "TOKEN" || source.Auth.Scopes[0] != "read" {
			t.Fatalf("clone mutated source MCP config fields: %#v", source)
		}
		if source.AuthStatus.Scopes[0] != "read" || !source.AuthStatus.ExpiresAt.Equal(expiresAt) ||
			!source.AuthStatus.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("clone mutated source auth status: %#v", source.AuthStatus)
		}
		if source.RuntimeStatus.Reason != "ready" ||
			source.SourceMetadata.ShadowedSources[0].Kind != SourceKindGlobalConfig ||
			source.SourceMetadata.AvailableTargets[0] != WriteTargetGlobalConfig {
			t.Fatalf("clone mutated source runtime or metadata: %#v", source)
		}
	})

	t.Run("Should pass the effective workspace source target to the runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := t.TempDir()
		writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName), `
[[mcp_servers]]
name = "workspace-docs"
command = "docs-mcp"
`)
		runtime := &fakeMCPRuntimeProvider{
			statuses: map[string]MCPServerRuntimeStatus{
				"workspace-docs": {
					Configured: true,
					State:      MCPServerRuntimeStateDead,
					Probe:      MCPServerProbeSkipped,
					Reason:     "backend_dead",
				},
			},
			targets: make(map[string]mcpauth.Target),
		}
		service := testService(t, homePaths, Dependencies{
			MCPRuntime: runtime,
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-runtime": {
					Workspace: workspacepkg.Workspace{ID: "ws-runtime", RootDir: workspaceRoot},
				},
			}},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{
			Collection:  CollectionMCPServers,
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-runtime",
		})
		if err != nil {
			t.Fatalf("ListCollection(workspace mcp) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, "workspace-docs")
		if item.RuntimeStatus == nil || item.RuntimeStatus.State != MCPServerRuntimeStateDead {
			t.Fatalf("workspace runtime status = %#v, want dead", item.RuntimeStatus)
		}
		if got, want := runtime.targets["workspace-docs"], (mcpauth.Target{
			Scope:       mcpauth.ScopeWorkspace,
			WorkspaceID: "ws-runtime",
			ServerName:  "workspace-docs",
		}); got != want {
			t.Fatalf("runtime target = %#v, want %#v", got, want)
		}
	})
}

func TestMCPAuthOperationsResolveExactWorkspaceSidecarTarget(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve the exact workspace sidecar target for every auth operation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := t.TempDir()
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]",
			"name = \"linear\"",
			"transport = \"http\"",
			"url = \"https://global.linear.example/mcp\"",
			"",
			"[mcp_servers.auth]",
			"type = \"oauth2_pkce\"",
			"authorization_url = \"https://global.linear.example/authorize\"",
			"token_url = \"https://global.linear.example/token\"",
			"client_id = \"global-client\"",
		}, "\n"))
		writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.MCPJSONName), `{
  "mcpServers": {
    "linear": {
	  "transport": "http",
      "url": "https://workspace.linear.example/mcp",
      "auth": {
        "type": "oauth2_pkce",
        "authorization_url": "https://workspace.linear.example/authorize",
        "token_url": "https://workspace.linear.example/token",
        "client_id": "workspace-client"
      }
    }
  }
}`)
		runtime := &recordingMCPAuthRuntime{}
		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{
				resolved: map[string]workspacepkg.ResolvedWorkspace{
					"workspace-a": {
						Workspace: workspacepkg.Workspace{ID: "workspace-a", RootDir: workspaceRoot},
					},
				},
			},
			MCPAuth: runtime,
		})
		workspaceTarget := MCPAuthTargetRequest{
			Scope: ScopeWorkspace, WorkspaceID: "workspace-a", Name: "linear",
		}
		begin, err := service.BeginMCPAuth(ctx, MCPAuthBeginRequest{
			MCPAuthTargetRequest: workspaceTarget,
			CallbackURL:          "http://127.0.0.1:2123/api/mcp/oauth/callback",
		})
		if err != nil {
			t.Fatalf("BeginMCPAuth(workspace sidecar) error = %v", err)
		}
		if begin.AuthorizationURL != "https://workspace.linear.example/authorize?state=public" {
			t.Fatalf("BeginMCPAuth() = %#v", begin)
		}
		if runtime.beginTarget.Scope != mcpauth.ScopeWorkspace ||
			runtime.beginTarget.WorkspaceID != "workspace-a" ||
			runtime.beginServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.beginServer.Auth.ClientID != "workspace-client" {
			t.Fatalf("workspace begin resolution = target:%#v server:%#v", runtime.beginTarget, runtime.beginServer)
		}

		status, err := service.ExchangeMCPAuth(ctx, MCPAuthExchangeRequest{
			MCPAuthTargetRequest: workspaceTarget,
			RedirectURL:          "https://callback.example/?code=opaque&state=public",
		})
		if err != nil {
			t.Fatalf("ExchangeMCPAuth(workspace) error = %v", err)
		}
		if status.WorkspaceID != "workspace-a" || !status.TokenPresent {
			t.Fatalf("ExchangeMCPAuth(workspace) status = %#v", status)
		}
		if runtime.exchangeInput.Code != "" ||
			runtime.exchangeInput.RedirectURL != "https://callback.example/?code=opaque&state=public" {
			t.Fatalf("exchange input = %#v", runtime.exchangeInput)
		}
		if runtime.exchangeTarget != runtime.beginTarget ||
			runtime.exchangeServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.exchangeServer.Auth.ClientID != "workspace-client" {
			t.Fatalf(
				"workspace exchange resolution = target:%#v server:%#v",
				runtime.exchangeTarget,
				runtime.exchangeServer,
			)
		}

		if _, err := service.LogoutMCPAuth(ctx, workspaceTarget); err != nil {
			t.Fatalf("LogoutMCPAuth(workspace sidecar) error = %v", err)
		}
		if runtime.logoutTarget != runtime.beginTarget || runtime.logoutServer.URL != runtime.beginServer.URL {
			t.Fatalf("logout resolution = target:%#v server:%#v", runtime.logoutTarget, runtime.logoutServer)
		}

		if _, err := service.CompleteMCPAuthCallback(
			ctx,
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=opaque&state=public",
		); err != nil {
			t.Fatalf("CompleteMCPAuthCallback() error = %v", err)
		}
		if !strings.Contains(runtime.callbackURL, "code=opaque") {
			t.Fatalf("callback URL = %q", runtime.callbackURL)
		}
		if runtime.callbackTarget != runtime.beginTarget ||
			runtime.callbackServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.callbackServer.Auth.ClientID != "workspace-client" {
			t.Fatalf(
				"workspace callback resolution = target:%#v server:%#v",
				runtime.callbackTarget,
				runtime.callbackServer,
			)
		}
	})
}

type recordingMCPAuthRuntime struct {
	beginTarget    mcpauth.Target
	beginServer    aghconfig.MCPServer
	exchangeTarget mcpauth.Target
	exchangeServer aghconfig.MCPServer
	exchangeInput  mcpauth.ExchangeInput
	logoutTarget   mcpauth.Target
	logoutServer   aghconfig.MCPServer
	callbackTarget mcpauth.Target
	callbackServer aghconfig.MCPServer
	callbackURL    string
	invalidated    []mcpauth.Target
	invalidateErrs map[int]error
}

func (r *recordingMCPAuthRuntime) MCPAuthStatus(
	context.Context,
	mcpauth.Target,
	aghconfig.MCPServer,
) (mcpauth.Status, error) {
	return mcpauth.Status{}, nil
}

func (r *recordingMCPAuthRuntime) MCPAuthBegin(
	_ context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
	callbackURL string,
) (mcpauth.BeginResult, error) {
	r.beginTarget = target
	r.beginServer = server
	return mcpauth.BeginResult{
		AuthorizationURL: "https://workspace.linear.example/authorize?state=public",
		State:            "public",
		ExpiresAt:        time.Date(2026, 7, 13, 16, 5, 0, 0, time.UTC),
		CallbackURL:      callbackURL,
		ManualSupported:  true,
	}, nil
}

func (r *recordingMCPAuthRuntime) MCPAuthExchange(
	_ context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
	input mcpauth.ExchangeInput,
) (mcpauth.Status, error) {
	r.exchangeTarget = target
	r.exchangeServer = server
	r.exchangeInput = input
	return confirmedMCPAuthRuntimeStatus(target), nil
}

func (r *recordingMCPAuthRuntime) MCPAuthCallbackTarget(string) (mcpauth.Target, error) {
	return mcpauth.Target{
		Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear",
	}, nil
}

func (r *recordingMCPAuthRuntime) MCPAuthCompleteCallback(
	_ context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
	callbackURL string,
) (mcpauth.Status, error) {
	r.callbackTarget = target
	r.callbackServer = server
	r.callbackURL = callbackURL
	return confirmedMCPAuthRuntimeStatus(target), nil
}

func (r *recordingMCPAuthRuntime) MCPAuthInvalidate(target mcpauth.Target) error {
	r.invalidated = append(r.invalidated, target)
	if err := r.invalidateErrs[len(r.invalidated)]; err != nil {
		return err
	}
	return nil
}

func (r *recordingMCPAuthRuntime) MCPAuthLogout(
	_ context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
) (mcpauth.Status, error) {
	r.logoutTarget = target
	r.logoutServer = server
	status := confirmedMCPAuthRuntimeStatus(target)
	status.Status = mcpauth.StatusNeedsLogin
	status.TokenPresent = false
	return status, nil
}

func confirmedMCPAuthRuntimeStatus(target mcpauth.Target) mcpauth.Status {
	return mcpauth.Status{
		ServerName:   target.ServerName,
		Scope:        target.Scope,
		WorkspaceID:  target.WorkspaceID,
		Status:       mcpauth.StatusAuthenticated,
		TokenPresent: true,
	}
}

type fakeMCPRuntimeProvider struct {
	statuses map[string]MCPServerRuntimeStatus
	targets  map[string]mcpauth.Target
}

func (f *fakeMCPRuntimeProvider) MCPServerRuntimeStatus(
	_ context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
) (MCPServerRuntimeStatus, error) {
	if f.targets != nil {
		f.targets[strings.TrimSpace(server.Name)] = target
	}
	if status, ok := f.statuses[strings.TrimSpace(server.Name)]; ok {
		return status, nil
	}
	return MCPServerRuntimeStatus{
		Configured: true,
		State:      MCPServerRuntimeStateRuntimeUnavailable,
		Probe:      MCPServerProbeFailed,
		Reason:     "test_missing_runtime_status",
	}, nil
}
