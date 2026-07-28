package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	memorypkg "github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	compozyupdate "github.com/compozy/compozy/internal/update"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

func TestSettingsRuntimeSurfaceMemoryHealthStatus(t *testing.T) {
	t.Run("Should count every valid global memory source header", func(t *testing.T) {
		t.Parallel()

		memoryStore := memorypkg.NewStore(filepath.Join(t.TempDir(), "memory"))
		for idx := range 205 {
			filename := fmt.Sprintf("settings-%03d.md", idx)
			if err := memoryStore.Write(
				memcontract.ScopeGlobal,
				filename,
				[]byte(memoryDocument(
					fmt.Sprintf("Settings %03d", idx),
					"Settings health",
					memcontract.TypeReference,
					"body",
				)),
			); err != nil {
				t.Fatalf("Write(%q) error = %v", filename, err)
			}
		}

		surface := &settingsRuntimeSurface{memoryStore: memoryStore}
		status, err := surface.MemoryHealthStatus(t.Context())
		if err != nil {
			t.Fatalf("MemoryHealthStatus() error = %v", err)
		}
		if !status.Available || status.FileCount != 205 {
			t.Fatalf("MemoryHealthStatus() = %#v, want available with 205 files", status)
		}
	})
}

func TestSettingsRuntimeSurfaceTransportParityStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		host                   string
		wantHTTPMutationParity bool
	}{
		{
			name:                   "loopback ipv4",
			host:                   "127.0.0.1",
			wantHTTPMutationParity: true,
		},
		{
			name:                   "localhost",
			host:                   "localhost",
			wantHTTPMutationParity: true,
		},
		{
			name:                   "wildcard ipv4",
			host:                   "0.0.0.0",
			wantHTTPMutationParity: false,
		},
		{
			name:                   "non loopback",
			host:                   "192.168.1.25",
			wantHTTPMutationParity: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			surface := &settingsRuntimeSurface{
				config: compozyconfig.Config{
					HTTP: compozyconfig.HTTPConfig{Host: tc.host},
				},
			}

			status, err := surface.TransportParityStatus(context.Background())
			if err != nil {
				t.Fatalf("TransportParityStatus() error = %v", err)
			}

			want := settingspkg.TransportParityStatus{
				Known:          true,
				SettingsHTTP:   tc.wantHTTPMutationParity,
				SettingsUDS:    true,
				ExtensionsHTTP: tc.wantHTTPMutationParity,
				ExtensionsUDS:  true,
			}
			if status != want {
				t.Fatalf("TransportParityStatus() = %#v, want %#v", status, want)
			}
		})
	}
}

func TestSettingsRuntimeSurfaceMCPAuthStatusSurvivesStoreReopen(t *testing.T) {
	t.Run("Should preserve MCP auth status after reopening the backing store", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		first, err := globaldb.OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}

		expiresAt := time.Date(2126, 5, 1, 12, 0, 0, 0, time.UTC)
		target := globalMCPTestTarget("remote-docs")
		server := compozyconfig.MCPServer{
			Name: "remote-docs", Transport: compozyconfig.MCPServerTransportHTTP, URL: "https://mcp.example.com",
			Auth: compozyconfig.MCPAuthConfig{
				Type: compozyconfig.MCPAuthTypeOAuth2PKCE, ClientID: "compozy-cli",
				AuthorizationURL: "https://issuer.example.com/oauth/authorize",
				TokenURL:         "https://issuer.example.com/oauth/token",
				Scopes:           []string{"mcp.read", "mcp.write"},
			},
		}
		cfg, err := mcpauth.ServerConfigFromMCP(ctx, target, server, nil)
		if err != nil {
			t.Fatalf("ServerConfigFromMCP() error = %v", err)
		}
		fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		if err := first.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: fingerprint, Issuer: "https://issuer.example.com",
			ClientID: "compozy-cli", Scopes: []string{"mcp.read", "mcp.write"},
			AccessToken: "access-secret", RefreshToken: "refresh-secret", TokenType: "Bearer",
			ExpiresAt: expiresAt, ObtainedAt: expiresAt.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := globaldb.OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		defer func() {
			if err := reopened.Close(ctx); err != nil {
				t.Fatalf("Close(reopened) error = %v", err)
			}
		}()

		manager, err := newSettingsMCPAuthManager(reopened, nil)
		if err != nil {
			t.Fatalf("newSettingsMCPAuthManager() error = %v", err)
		}
		surface := &settingsRuntimeSurface{mcpAuthStore: reopened, mcpAuthManager: manager}
		status, err := surface.MCPAuthStatus(ctx, target, server)
		if err != nil {
			t.Fatalf("MCPAuthStatus() error = %v", err)
		}
		if status.Status != mcpauth.StatusAuthenticated {
			t.Fatalf("MCPAuthStatus().Status = %q, want %q", status.Status, mcpauth.StatusAuthenticated)
		}
		if !status.TokenPresent || !status.Refreshable {
			t.Fatalf("MCPAuthStatus() = %#v, want token present and refreshable", status)
		}
		if status.ExpiresAt == nil || !status.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("MCPAuthStatus().ExpiresAt = %v, want %v", status.ExpiresAt, expiresAt)
		}
	})
}

func TestSettingsRuntimeSurfaceMCPAuthStatusResolvesClientSecretRef(t *testing.T) {
	t.Run("Should resolve MCP client_secret_ref before computing auth status", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		called := false
		surface := &settingsRuntimeSurface{
			secretResolver: func(_ context.Context, ref string) (string, error) {
				called = true
				if ref != "vault:mcp/global/remote-docs/oauth/client-secret" {
					t.Fatalf("secret resolver ref = %q, want remote-docs client secret ref", ref)
				}
				return "client-secret", nil
			},
		}

		status, err := surface.MCPAuthStatus(ctx, globalMCPTestTarget("remote-docs"), compozyconfig.MCPServer{
			Name:      "remote-docs",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       "https://mcp.example.com",
			Auth: compozyconfig.MCPAuthConfig{
				Type:             compozyconfig.MCPAuthTypeOAuth2PKCE,
				ClientID:         "compozy-cli",
				ClientSecretRef:  "vault:mcp/global/remote-docs/oauth/client-secret",
				AuthorizationURL: "https://issuer.example.com/oauth/authorize",
				TokenURL:         "https://issuer.example.com/oauth/token",
			},
		})
		if err != nil {
			t.Fatalf("MCPAuthStatus() error = %v", err)
		}
		if !called {
			t.Fatal("MCPAuthStatus() did not resolve client_secret_ref")
		}
		if got, want := status.Status, mcpauth.StatusNeedsLogin; got != want {
			t.Fatalf("MCPAuthStatus().Status = %q, want %q", got, want)
		}
	})
}

func TestSettingsRuntimeSurfaceMCPServerRuntimeStatus(t *testing.T) {
	t.Run("Should default MCP runtime probe timeout to five seconds", func(t *testing.T) {
		t.Parallel()

		surface := &settingsRuntimeSurface{}
		if got, want := surface.mcpProbeTimeout(), 5*time.Second; got != want {
			t.Fatalf("mcpProbeTimeout() = %s, want %s", got, want)
		}
	})

	t.Run("Should use the configured observability probe timeout for MCP runtime probes", func(t *testing.T) {
		t.Parallel()

		surface := &settingsRuntimeSurface{
			config: compozyconfig.Config{
				Observability: compozyconfig.ObservabilityConfig{
					AgentProbeTimeout: 9 * time.Second,
				},
			},
		}
		if got, want := surface.mcpProbeTimeout(), 9*time.Second; got != want {
			t.Fatalf("mcpProbeTimeout() = %s, want %s", got, want)
		}
	})

	t.Run("Should probe a reachable MCP server through the real executor", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		server := mcpsrv.NewTestStreamableHTTPServer(newSettingsMCPTestServer())
		t.Cleanup(server.Close)

		surface := &settingsRuntimeSurface{}
		status, err := surface.MCPServerRuntimeStatus(ctx, globalMCPTestTarget("docs"), compozyconfig.MCPServer{
			Name:      "docs",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       server.URL,
		})
		if err != nil {
			t.Fatalf("MCPServerRuntimeStatus() error = %v", err)
		}
		if got, want := status.State, settingspkg.MCPServerRuntimeStateReady; got != want {
			t.Fatalf("MCPServerRuntimeStatus().State = %q, want %q", got, want)
		}
		if got, want := status.Probe, settingspkg.MCPServerProbeSucceeded; got != want {
			t.Fatalf("MCPServerRuntimeStatus().Probe = %q, want %q", got, want)
		}
		if !status.Initialized || status.ToolCount != 1 {
			t.Fatalf("MCPServerRuntimeStatus() = %#v, want initialized with one tool", status)
		}
	})

	t.Run("Should skip probing when remote MCP auth needs login", func(t *testing.T) {
		t.Parallel()

		surface := &settingsRuntimeSurface{}
		status, err := surface.MCPServerRuntimeStatus(
			context.Background(),
			globalMCPTestTarget("linear"),
			compozyconfig.MCPServer{
				Name:      "linear",
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.linear.example/mcp",
				Auth: compozyconfig.MCPAuthConfig{
					Type:             compozyconfig.MCPAuthTypeOAuth2PKCE,
					AuthorizationURL: "https://auth.linear.example/authorize",
					TokenURL:         "https://auth.linear.example/token",
					ClientID:         "compozy-desktop",
				},
			},
		)
		if err != nil {
			t.Fatalf("MCPServerRuntimeStatus(auth) error = %v", err)
		}
		if got, want := status.State, settingspkg.MCPServerRuntimeStateAuthRequired; got != want {
			t.Fatalf("MCPServerRuntimeStatus(auth).State = %q, want %q", got, want)
		}
		if got, want := status.Probe, settingspkg.MCPServerProbeSkipped; got != want {
			t.Fatalf("MCPServerRuntimeStatus(auth).Probe = %q, want %q", got, want)
		}
		if status.Initialized || status.ToolCount != 0 {
			t.Fatalf("MCPServerRuntimeStatus(auth) = %#v, want no initialization or tools", status)
		}
	})

	t.Run("Should report config errors without fabricating a probe", func(t *testing.T) {
		t.Parallel()

		surface := &settingsRuntimeSurface{}
		status, err := surface.MCPServerRuntimeStatus(
			context.Background(),
			globalMCPTestTarget("broken"),
			compozyconfig.MCPServer{
				Name:      "broken",
				Transport: compozyconfig.MCPServerTransportHTTP,
			},
		)
		if err != nil {
			t.Fatalf("MCPServerRuntimeStatus(config error) error = %v", err)
		}
		if got, want := status.State, settingspkg.MCPServerRuntimeStateConfigError; got != want {
			t.Fatalf("MCPServerRuntimeStatus(config error).State = %q, want %q", got, want)
		}
		if got, want := status.Probe, settingspkg.MCPServerProbeSkipped; got != want {
			t.Fatalf("MCPServerRuntimeStatus(config error).Probe = %q, want %q", got, want)
		}
		if status.Diagnostic == "" {
			t.Fatal("MCPServerRuntimeStatus(config error).Diagnostic is empty")
		}
	})

	t.Run("Should project a workspace MCP server as dead after five permanent failures", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		globalDB, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(ctx); err != nil {
				t.Errorf("GlobalDB.Close() error = %v", err)
			}
		})
		workspaceID := "ws-dead-settings"
		now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
		if err := globalDB.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        workspaceID,
			Name:      "dead-settings",
			RootDir:   t.TempDir(),
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		surface := &settingsRuntimeSurface{deadEntities: deadentity.New(globalDB)}
		target := mcpauth.Target{
			Scope:       mcpauth.ScopeWorkspace,
			WorkspaceID: workspaceID,
			ServerName:  "dead-docs",
		}
		invalidServer := compozyconfig.MCPServer{Name: "dead-docs", Transport: "bogus"}

		for attempt := 1; attempt < deadentity.DefaultPermanentFailureThreshold; attempt++ {
			status, err := surface.MCPServerRuntimeStatus(ctx, target, invalidServer)
			if err != nil {
				t.Fatalf("MCPServerRuntimeStatus(attempt %d) error = %v", attempt, err)
			}
			if status.State != settingspkg.MCPServerRuntimeStateConfigError {
				t.Fatalf("MCPServerRuntimeStatus(attempt %d).State = %q, want config_error", attempt, status.State)
			}
		}
		status, err := surface.MCPServerRuntimeStatus(ctx, target, invalidServer)
		if err != nil {
			t.Fatalf("MCPServerRuntimeStatus(threshold) error = %v", err)
		}
		if status.State != settingspkg.MCPServerRuntimeStateDead ||
			status.Probe != settingspkg.MCPServerProbeSkipped ||
			status.Reason != "backend_dead" {
			t.Fatalf("MCPServerRuntimeStatus(threshold) = %#v, want dead/skipped/backend_dead", status)
		}
		listed, err := globalDB.ListDeadEntities(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ListDeadEntities() error = %v", err)
		}
		if len(listed) != 1 || listed[0].EntityID != "dead-docs" {
			t.Fatalf("ListDeadEntities() = %#v, want one dead-docs mark", listed)
		}
	})
}

func globalMCPTestTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: serverName}
}

func newSettingsMCPTestServer() *mcpsrv.MCPServer {
	server := mcpsrv.NewMCPServer("settings-test", "1.0.0", mcpsrv.WithToolCapabilities(true))
	server.AddTool(
		mcpsdk.NewTool(
			"lookup",
			mcpsdk.WithDescription("Lookup documentation"),
			mcpsdk.WithString("query"),
			mcpsdk.WithRawOutputSchema(json.RawMessage(
				"{\"type\":\"object\",\"properties\":{\"answer\":{\"type\":\"string\"}}}",
			)),
			mcpsdk.WithReadOnlyHintAnnotation(true),
		),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ok"), nil
		},
	)
	return server
}

type stubSettingsUpdateManager struct {
	checkFn func(context.Context, compozyupdate.CheckOptions) (compozyupdate.State, *compozyupdate.Release, error)
}

func (s stubSettingsUpdateManager) Check(
	ctx context.Context,
	opts compozyupdate.CheckOptions,
) (compozyupdate.State, *compozyupdate.Release, error) {
	if s.checkFn != nil {
		return s.checkFn(ctx, opts)
	}
	return compozyupdate.State{}, nil, nil
}

func TestSettingsUpdateControllerGetUpdate(t *testing.T) {
	t.Run("Should translate the cached update snapshot from the shared manager", func(t *testing.T) {
		t.Parallel()

		checkedAt := time.Date(2026, 5, 3, 19, 0, 0, 0, time.UTC)
		controller := settingsUpdateController{
			manager: stubSettingsUpdateManager{
				checkFn: func(_ context.Context, opts compozyupdate.CheckOptions) (compozyupdate.State, *compozyupdate.Release, error) {
					if opts.ForceRefresh {
						t.Fatal("CheckOptions.ForceRefresh = true, want false")
					}
					if !opts.AllowCachedOnFailure {
						t.Fatal("CheckOptions.AllowCachedOnFailure = false, want true")
					}
					return compozyupdate.State{
						Supported:      true,
						Managed:        false,
						InstallMethod:  string(compozyupdate.InstallMethodDirectBinary),
						CurrentVersion: "v1.0.0",
						LatestVersion:  "v1.1.0",
						Available:      true,
						Status:         compozyupdate.StatusAvailable,
						Recommendation: "Run compozy update.",
						ReleaseURL:     "https://github.com/compozy/compozy/releases/tag/v1.1.0",
						CheckedAt:      &checkedAt,
						LastError:      "cached upstream failure",
					}, &compozyupdate.Release{Version: "v1.1.0"}, nil
				},
			},
		}

		got, err := controller.GetUpdate(context.Background())
		if err != nil {
			t.Fatalf("GetUpdate() error = %v", err)
		}

		want := core.SettingsUpdateStatus{
			Supported:      true,
			Managed:        false,
			InstallMethod:  string(compozyupdate.InstallMethodDirectBinary),
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.1.0",
			Available:      true,
			Status:         string(compozyupdate.StatusAvailable),
			Recommendation: "Run compozy update.",
			ReleaseURL:     "https://github.com/compozy/compozy/releases/tag/v1.1.0",
			CheckedAt:      &checkedAt,
			LastError:      "cached upstream failure",
		}
		if got != want {
			t.Fatalf("GetUpdate() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject a missing settings update manager", func(t *testing.T) {
		t.Parallel()

		_, err := (settingsUpdateController{}).GetUpdate(context.Background())
		if err == nil {
			t.Fatal("GetUpdate() error = nil, want missing manager error")
		}
	})

	t.Run("Should surface raw manager errors when no state message is available", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("upstream unavailable")
		controller := settingsUpdateController{
			manager: stubSettingsUpdateManager{
				checkFn: func(context.Context, compozyupdate.CheckOptions) (compozyupdate.State, *compozyupdate.Release, error) {
					return compozyupdate.State{}, nil, wantErr
				},
			},
		}

		_, err := controller.GetUpdate(context.Background())
		if !errors.Is(err, wantErr) {
			t.Fatalf("GetUpdate() error = %v, want %v", err, wantErr)
		}
	})
}
