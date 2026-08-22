package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	memorypkg "github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/procutil"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	toolspkg "github.com/compozy/compozy/internal/tools"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExtensionPaletteSettingsByName(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve effective and dormant contribution state", func(t *testing.T) {
		t.Parallel()

		projection := extensionpkg.CmdPaletteProjection{
			Commands: []extensionpkg.CmdPaletteProjectedCommand{
				{ID: "ext.notes.capture", Title: "Capture note", Extension: "notes"},
				{
					ID: "ext.notes.recent", Title: "Recent notes", Extension: "notes",
					UnavailableReason: "extension notes is unhealthy (crash loop)",
				},
			},
			Views: []extensionpkg.CmdPaletteProjectedView{{
				ID: "ext.notes.browse", Title: "Browse notes", Extension: "notes",
				UnavailableReason: "extension notes is unhealthy (crash loop)",
			}},
		}
		result := extensionPaletteSettingsByName(
			projection,
			map[string]windowmanager.ShortcutBinding{"ext.notes.capture": {"alt+shift+n"}},
			map[string]windowmanager.ExtensionDefaultStatus{
				"ext.notes.recent": {
					CommandID: "ext.notes.recent", Binding: windowmanager.ShortcutBinding{"mod+n"},
					Dormant: true, ConflictWith: "session.new",
				},
			},
		)

		palette := result["notes"]
		if palette == nil || len(palette.Commands) != 2 || len(palette.Views) != 1 {
			t.Fatalf("extensionPaletteSettingsByName() = %#v, want two commands and one view", result)
		}
		if got := palette.Commands[0].Bindings; !slices.Equal(got, []string{"alt+shift+n"}) {
			t.Fatalf("capture bindings = %v, want [alt+shift+n]", got)
		}
		recent := palette.Commands[1]
		if !recent.DefaultDormant || recent.ConflictWith != "session.new" || recent.Available {
			t.Fatalf("recent contribution = %#v, want dormant unavailable conflict", recent)
		}
		if palette.Views[0].Available || !strings.Contains(palette.Views[0].Reason, "crash loop") {
			t.Fatalf("view contribution = %#v, want unhealthy reason", palette.Views[0])
		}
	})
}

func TestSettingsRuntimeInstalledExtensionsPalette(t *testing.T) {
	t.Parallel()

	t.Run("Should attach populated and dormant palette contributions", func(t *testing.T) {
		t.Parallel()
		cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()})
		cfg.WindowManager.Shortcuts["session.new"] = windowmanager.ShortcutBinding{"meta+KeyN"}
		cfg.WindowManager.Shortcuts["ext.notes.capture"] = windowmanager.ShortcutBinding{"alt+shift+KeyN"}
		surface := &settingsRuntimeSurface{
			config: cfg,
			extensions: settingsExtensionListStub{items: []contract.ExtensionPayload{{
				Name: "notes", Version: "0.1.0", Enabled: true, State: "running",
			}}},
			extensionRuntime: func() extensionRuntime {
				return settingsPaletteRuntimeStub{projection: extensionpkg.CmdPaletteProjection{
					Commands: []extensionpkg.CmdPaletteProjectedCommand{
						{ID: "ext.notes.capture", Title: "Capture note", Extension: "notes"},
						{
							ID: "ext.notes.recent", Title: "Recent notes", Extension: "notes",
							UnavailableReason: "extension notes is unhealthy (crash loop)",
						},
					},
					Views: []extensionpkg.CmdPaletteProjectedView{{
						ID: "ext.notes.browse", Title: "Browse notes", Extension: "notes",
						UnavailableReason: "extension notes is unhealthy (crash loop)",
					}},
					Defaults: []extensionpkg.CmdPaletteDefaultShortcut{{
						CommandID: "ext.notes.recent", Chord: "meta+KeyN", Extension: "notes", Active: true,
					}},
				}}
			},
		}
		installed, err := surface.InstalledExtensions(t.Context())
		if err != nil {
			t.Fatalf("InstalledExtensions() error = %v", err)
		}
		if len(installed) != 1 || installed[0].Palette == nil {
			t.Fatalf("InstalledExtensions() = %#v, want one palette attachment", installed)
		}
		palette := installed[0].Palette
		if len(palette.Commands) != 2 || len(palette.Views) != 1 {
			t.Fatalf("palette = %#v, want two commands and one view", palette)
		}
		if !slices.Equal(palette.Commands[0].Bindings, []string{"alt+shift+KeyN"}) || !palette.Commands[0].Available {
			t.Fatalf("capture command = %#v, want populated binding", palette.Commands[0])
		}
		if !palette.Commands[1].DefaultDormant || palette.Commands[1].Available {
			t.Fatalf("recent command = %#v, want dormant unavailable contribution", palette.Commands[1])
		}
	})

	t.Run("Should wrap palette projection failures", func(t *testing.T) {
		t.Parallel()
		want := errors.New("palette projection failed")
		surface := &settingsRuntimeSurface{
			config: compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()}),
			extensions: settingsExtensionListStub{items: []contract.ExtensionPayload{{
				Name: "notes", Enabled: true,
			}}},
			extensionRuntime: func() extensionRuntime {
				return settingsPaletteRuntimeStub{err: want}
			},
		}
		_, err := surface.InstalledExtensions(t.Context())
		if !errors.Is(err, want) {
			t.Fatalf("InstalledExtensions() error = %v, want wrapped %v", err, want)
		}
	})
}

type settingsExtensionListStub struct {
	items []contract.ExtensionPayload
	err   error
}

func (s settingsExtensionListStub) List(context.Context) ([]contract.ExtensionPayload, error) {
	return s.items, s.err
}

type settingsPaletteRuntimeStub struct {
	projection extensionpkg.CmdPaletteProjection
	err        error
}

func (s settingsPaletteRuntimeStub) Start(context.Context) error  { return nil }
func (s settingsPaletteRuntimeStub) Stop(context.Context) error   { return nil }
func (s settingsPaletteRuntimeStub) Reload(context.Context) error { return nil }
func (s settingsPaletteRuntimeStub) Get(string) (*extensionpkg.Extension, error) {
	return nil, errors.New("unexpected Get")
}
func (s settingsPaletteRuntimeStub) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}
func (s settingsPaletteRuntimeStub) InspectPackageResources(
	context.Context,
	string,
) (*extensionpkg.Extension, error) {
	return nil, errors.New("unexpected InspectPackageResources")
}
func (s settingsPaletteRuntimeStub) CmdPaletteSettings(string) (extensionpkg.CmdPaletteProjection, error) {
	return s.projection, s.err
}

func TestSettingsRuntimeSurfaceMemoryHealthStatus(t *testing.T) {
	t.Run("Should count every valid global memory source header", func(t *testing.T) {
		t.Parallel()

		memoryStore := memorypkg.NewStore(filepath.Join(t.TempDir(), "memory"))
		for idx := range 205 {
			filename := fmt.Sprintf("settings-%03d.md", idx)
			if err := memoryStore.Write(t.Context(),
				memcontract.ScopeProfile,
				filename,
				[]byte(memoryDocument(
					fmt.Sprintf("Settings %03d", idx),
					"Settings health",
					memcontract.TypeReference,
					"body",
				))); err != nil {
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
				Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
				IssuerURL:    "https://issuer.example.com",
				ClientID:     "compozy-cli",
				Scopes:       []string{"mcp.read", "mcp.write"},
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

		manager, err := newSettingsMCPAuthManagerWithConfig(
			reopened,
			nil,
			nil,
			nil,
			compozyconfig.MCPOAuthConfig{},
		)
		if err != nil {
			t.Fatalf("newSettingsMCPAuthManagerWithConfig() error = %v", err)
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

func TestSettingsRuntimeSurfaceMCPAuthAllowsOperatorLoopback(t *testing.T) {
	t.Run("Should discover operator configured OAuth metadata on loopback", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		var authorizationServer *httptest.Server
		authorizationHandler := http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			requests.Add(1)
			if request.URL.Path != "/.well-known/oauth-authorization-server" {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			if _, err := fmt.Fprintf(
				writer,
				`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,`+
					`"code_challenge_methods_supported":["S256"]}`,
				authorizationServer.URL,
				authorizationServer.URL+"/authorize",
				authorizationServer.URL+"/token",
			); err != nil {
				t.Errorf("write authorization metadata: %v", err)
			}
		})
		authorizationServer = httptest.NewServer(authorizationHandler)
		defer authorizationServer.Close()

		ctx := t.Context()
		database, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		defer func() {
			if err := database.Close(context.Background()); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		}()
		manager, err := newSettingsMCPAuthManagerWithConfig(
			database,
			nil,
			database,
			nil,
			compozyconfig.MCPOAuthConfig{},
		)
		if err != nil {
			t.Fatalf("newSettingsMCPAuthManagerWithConfig() error = %v", err)
		}
		target := globalMCPTestTarget("local-oauth")
		server := compozyconfig.MCPServer{
			Name:      target.ServerName,
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       authorizationServer.URL + "/mcp",
			Auth: compozyconfig.MCPAuthConfig{
				Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
				IssuerURL:    authorizationServer.URL,
				ClientID:     "compozy-local",
			},
		}

		surface := &settingsRuntimeSurface{mcpAuthManager: manager}
		result, err := surface.MCPAuthBegin(
			ctx,
			target,
			server,
			"http://127.0.0.1:2123/api/mcp/oauth/callback",
		)
		if err != nil {
			t.Fatalf("MCPAuthBegin() error = %v", err)
		}
		if !strings.HasPrefix(result.AuthorizationURL, authorizationServer.URL+"/authorize?") {
			t.Fatalf("AuthorizationURL = %q, want local authorization endpoint", result.AuthorizationURL)
		}
		if got := requests.Load(); got != 1 {
			t.Fatalf("authorization metadata requests = %d, want 1", got)
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
				Registration:    compozyconfig.MCPAuthRegistrationPreRegistered,
				IssuerURL:       "https://issuer.example.com",
				ClientID:        "compozy-cli",
				ClientSecretRef: "vault:mcp/global/remote-docs/oauth/client-secret",
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
	t.Run("Should classify permission failures by error identity", func(t *testing.T) {
		t.Parallel()

		permissionStatus := runtimeStatusFromMCPProbeError(fmt.Errorf("launch MCP process: %w", os.ErrPermission))
		if got, want := permissionStatus.State, settingspkg.MCPServerRuntimeStatePermissionDenied; got != want {
			t.Fatalf("typed permission state = %q, want %q", got, want)
		}

		messageOnlyStatus := runtimeStatusFromMCPProbeError(errors.New("permission denied"))
		if got, want := messageOnlyStatus.State, settingspkg.MCPServerRuntimeStateRuntimeUnavailable; got != want {
			t.Fatalf("message-only permission state = %q, want %q", got, want)
		}
	})

	t.Run("Should probe a reachable MCP server independent of observability agent probe timeout", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return newSettingsMCPTestServer()
		}, &mcp.StreamableHTTPOptions{
			Stateless:                  true,
			JSONResponse:               true,
			DisableLocalhostProtection: true,
		}))
		t.Cleanup(server.Close)

		surface := &settingsRuntimeSurface{config: compozyconfig.Config{
			Observability: compozyconfig.ObservabilityConfig{
				AgentProbeTimeout: time.Nanosecond,
			},
		}}
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
		if got, want := status.ProtocolVersion, mcpfixture.ModernProtocolVersion; got != want {
			t.Fatalf("MCPServerRuntimeStatus().ProtocolVersion = %q, want %q", got, want)
		}
	})

	t.Run("Should bound runtime diagnostics independently of the operational MCP call timeout", func(t *testing.T) {
		t.Parallel()

		blockingServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			select {
			case <-request.Context().Done():
			case <-time.After(time.Second):
			}
		}))
		t.Cleanup(blockingServer.Close)
		surface := &settingsRuntimeSurface{mcpStatusTimeout: 20 * time.Millisecond}

		startedAt := time.Now()
		status, err := surface.MCPServerRuntimeStatus(
			t.Context(),
			globalMCPTestTarget("slow"),
			compozyconfig.MCPServer{
				Name:      "slow",
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       blockingServer.URL,
			},
		)
		elapsed := time.Since(startedAt)

		if err != nil {
			t.Fatalf("MCPServerRuntimeStatus(slow) error = %v", err)
		}
		if elapsed >= 250*time.Millisecond {
			t.Fatalf("MCPServerRuntimeStatus(slow) elapsed = %s, want a bounded diagnostic", elapsed)
		}
		if got, want := status.State, settingspkg.MCPServerRuntimeStateRuntimeUnavailable; got != want {
			t.Fatalf("MCPServerRuntimeStatus(slow).State = %q, want %q", got, want)
		}
		if got, want := status.Probe, settingspkg.MCPServerProbeFailed; got != want {
			t.Fatalf("MCPServerRuntimeStatus(slow).Probe = %q, want %q", got, want)
		}
		if got, want := status.Reason, string(toolspkg.ReasonCallTimedOut); got != want {
			t.Fatalf("MCPServerRuntimeStatus(slow).Reason = %q, want %q", got, want)
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
					Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://auth.linear.example",
					ClientID:     "compozy-desktop",
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
		listed, err := globalDB.ListDeadEntities(ctx, store.ReadScope{AllProfiles: true}, workspaceID)
		if err != nil {
			t.Fatalf("ListDeadEntities() error = %v", err)
		}
		if len(listed) != 1 || listed[0].EntityID != "dead-docs" {
			t.Fatalf("ListDeadEntities() = %#v, want one dead-docs mark", listed)
		}
	})
}

func globalMCPTestTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: serverName}
}

func newSettingsMCPTestServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "settings-test", Version: "1.0.0"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	inputSchema := json.RawMessage(`{
		"type":"object",
		"properties":{"query":{"type":"string"}},
		"required":["query"],
		"additionalProperties":false
	}`)
	server.AddTool(&mcp.Tool{
		Name:         "lookup",
		Description:  "Lookup documentation",
		InputSchema:  inputSchema,
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`),
		Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	return server
}

type stubSettingsUpdateManager struct {
	snapshotFn func(context.Context) (compozyupdate.MultiState, error)
	checkAllFn func(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error)
	planFn     func(context.Context, compozyupdate.Actor, []compozyupdate.Target, compozyupdate.Holder) (compozyupdate.OperationRequest, error)
	acquireFn  func(context.Context, compozyupdate.OperationRequest) (*compozyupdate.Operation, error)
	store      *compozyupdate.OperationStore
}

type settingsUpdateRoundTripFunc func(*http.Request) (*http.Response, error)

func (f settingsUpdateRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (s stubSettingsUpdateManager) Snapshot(ctx context.Context) (compozyupdate.MultiState, error) {
	if s.snapshotFn != nil {
		return s.snapshotFn(ctx)
	}
	return compozyupdate.MultiState{}, errors.New("unexpected Snapshot call")
}

func (s stubSettingsUpdateManager) CheckAll(
	ctx context.Context,
	opts compozyupdate.CheckOptions,
) (compozyupdate.MultiState, *compozyupdate.Release, error) {
	if s.checkAllFn != nil {
		return s.checkAllFn(ctx, opts)
	}
	return compozyupdate.MultiState{}, nil, errors.New("unexpected CheckAll call")
}

func (s stubSettingsUpdateManager) PlanOperation(
	ctx context.Context,
	actor compozyupdate.Actor,
	targets []compozyupdate.Target,
	holder compozyupdate.Holder,
) (compozyupdate.OperationRequest, error) {
	if s.planFn != nil {
		return s.planFn(ctx, actor, targets, holder)
	}
	return compozyupdate.OperationRequest{}, errors.New("unexpected PlanOperation call")
}

func (s stubSettingsUpdateManager) AcquireOperation(
	ctx context.Context,
	request compozyupdate.OperationRequest,
) (*compozyupdate.Operation, error) {
	if s.acquireFn != nil {
		return s.acquireFn(ctx, request)
	}
	if s.store != nil {
		return s.store.Acquire(ctx, request)
	}
	return nil, errors.New("unexpected AcquireOperation call")
}

func (s stubSettingsUpdateManager) OperationStore() *compozyupdate.OperationStore { return s.store }

func daemonSettingsUpdateStateFixture() compozyupdate.MultiState {
	return compozyupdate.MultiState{
		Aggregate: compozyupdate.StatusAvailable,
		Runtime: compozyupdate.RuntimeTrackState{
			Status:         compozyupdate.StatusAvailable,
			InstallMethod:  string(compozyupdate.InstallMethodDirectBinary),
			CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0",
			Recommendation: "Run compozy update.",
			ReleaseURL:     "https://github.com/compozy/compozy/releases/tag/v1.1.0",
			LastError:      "cached upstream failure",
		},
	}
}

func TestSettingsUpdateControllerGetUpdate(t *testing.T) {
	t.Run("Should expose desktop provenance through the settings HTTP handler", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		binDir := filepath.Join(homePaths.HomeDir, "bin")
		if err := os.MkdirAll(binDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(bin) error = %v", err)
		}
		binaryPath := filepath.Join(binDir, "compozy")
		binary := []byte("desktop-managed-runtime")
		if err := os.WriteFile(binaryPath, binary, 0o700); err != nil {
			t.Fatalf("WriteFile(runtime binary) error = %v", err)
		}
		if err := compozyupdate.WriteDesktopProvenance(
			homePaths,
			binaryPath,
			compozyupdate.DesktopProvenanceMetadata{
				AppVersion: "1.0.0", Channel: "beta", RuntimeVersion: "1.0.0",
			},
		); err != nil {
			t.Fatalf("WriteDesktopProvenance() error = %v", err)
		}
		releasePayload := `{
			"tag_name":"v1.1.0",
			"html_url":"https://example.com/v1.1.0",
			"published_at":"2026-08-10T03:00:00Z",
			"assets":[
				{"name":"compozy_linux_x86_64.tar.gz","browser_download_url":"https://example.com/archive"},
				{"name":"checksums.txt","browser_download_url":"https://example.com/checksums"},
				{"name":"checksums.txt.sigstore.json","browser_download_url":"https://example.com/bundle"}
			]
		}`

		releaseTransport := settingsUpdateRoundTripFunc(
			func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(releasePayload)),
				}, nil
			},
		)
		releaseClient := &http.Client{Transport: releaseTransport}
		manager, err := compozyupdate.NewManager(&compozyupdate.Config{
			HomePaths:       homePaths,
			CurrentVersion:  "v1.0.0",
			ExecutablePath:  func() (string, error) { return binaryPath, nil },
			ResolveSymlinks: func(path string) (string, error) { return path, nil },
			Getenv:          func(string) string { return "" },
			RuntimeOS:       "linux",
			RuntimeArch:     "amd64",
			HTTPClient:      releaseClient,
		})
		if err != nil {
			t.Fatalf("update.NewManager() error = %v", err)
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:  "api-core-http",
			SettingsUpdate: settingsUpdateController{manager: manager},
			HomePaths:      homePaths,
			Logger:         testutil.DiscardLogger(),
		})
		engine := gin.New()
		engine.GET("/api/settings/update", handlers.GetSettingsUpdate)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/settings/update",
			http.NoBody,
		)
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET /api/settings/update status = %d body=%s", response.Code, response.Body.String())
		}
		var payload contract.SettingsUpdateResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal(settings update response) error = %v", err)
		}
		if payload.Runtime.InstallMethod != contract.SettingsUpdateInstallDesktopApp ||
			payload.Runtime.Managed ||
			payload.Runtime.Recommendation != "Run `compozy update`." {
			t.Fatalf("settings update response = %#v, want self-applying desktop runtime", payload)
		}
	})

	t.Run("Should translate the cached update snapshot from the shared manager", func(t *testing.T) {
		t.Parallel()

		want := daemonSettingsUpdateStateFixture()
		controller := settingsUpdateController{
			manager: stubSettingsUpdateManager{
				snapshotFn: func(context.Context) (compozyupdate.MultiState, error) {
					return daemonSettingsUpdateStateFixture(), nil
				},
			},
		}

		got, err := controller.GetUpdate(context.Background())
		if err != nil {
			t.Fatalf("GetUpdate() error = %v", err)
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
				snapshotFn: func(context.Context) (compozyupdate.MultiState, error) {
					return compozyupdate.MultiState{}, wantErr
				},
			},
		}

		_, err := controller.GetUpdate(context.Background())
		if !errors.Is(err, wantErr) {
			t.Fatalf("GetUpdate() error = %v, want %v", err, wantErr)
		}
	})
}

func TestSettingsUpdateControllerMutations(t *testing.T) {
	t.Run("Should derive holder identity from daemon composition seams", func(t *testing.T) {
		t.Parallel()

		const pid = 4242
		startedAt := time.Date(2026, time.August, 16, 10, 0, 0, 0, time.UTC)
		now := startedAt.Add(time.Hour)
		daemon := &Daemon{
			pid: func() int { return pid },
			processStartedAt: func(gotPID int) (time.Time, error) {
				if gotPID != pid {
					t.Fatalf("processStartedAt PID = %d, want %d", gotPID, pid)
				}
				return startedAt, nil
			},
			now: func() time.Time { return now },
		}

		holder, err := newDaemonUpdateHolder(daemon, compozyupdate.ActorDaemon)
		if err != nil {
			t.Fatalf("newDaemonUpdateHolder() error = %v", err)
		}
		if holder.PID != pid || !holder.PIDStartTime.Equal(startedAt) ||
			holder.Surface != compozyupdate.ActorDaemon || holder.ExecutorGeneration == "" ||
			!holder.LeaseExpiresAt.Equal(now.Add(2*time.Minute)) {
			t.Fatalf("newDaemonUpdateHolder() = %#v, want injected identity and lease", holder)
		}
	})

	t.Run("Should persist before spawning and report acceptance", func(t *testing.T) {
		t.Parallel()

		store := newDaemonSettingsOperationStore(t)
		spawned := false
		manager := stubSettingsUpdateManager{
			store: store,
			planFn: func(
				_ context.Context,
				actor compozyupdate.Actor,
				targets []compozyupdate.Target,
				holder compozyupdate.Holder,
			) (compozyupdate.OperationRequest, error) {
				if actor != compozyupdate.ActorWeb || len(targets) != 1 || targets[0] != compozyupdate.TargetRuntime {
					t.Fatalf("PlanOperation() actor/targets = %q/%v", actor, targets)
				}
				return daemonSettingsOperationRequest(holder, targets), nil
			},
		}
		controller := settingsUpdateController{
			manager: manager,
			holder: func(compozyupdate.Actor) (compozyupdate.Holder, error) {
				return daemonSettingsHolder(t), nil
			},
			spawn: func(ctx context.Context, operation *compozyupdate.Operation) error {
				persisted, err := store.Read(ctx)
				if err != nil {
					return err
				}
				if persisted == nil || persisted.ID != operation.ID {
					return errors.New("operation was not persisted before spawn")
				}
				spawned = true
				return nil
			},
		}

		requestCtx, cancelRequest := context.WithCancel(t.Context())
		defer cancelRequest()
		controller.spawn = func(ctx context.Context, operation *compozyupdate.Operation) error {
			cancelRequest()
			if ctx.Err() != nil {
				return errors.New("spawn inherited request cancellation")
			}
			persisted, err := store.Read(ctx)
			if err != nil {
				return err
			}
			if persisted == nil || persisted.ID != operation.ID {
				return errors.New("operation was not persisted before spawn")
			}
			spawned = true
			return nil
		}
		result, err := controller.ApplyUpdate(requestCtx, compozyupdate.TargetRuntime)
		if err != nil {
			t.Fatalf("ApplyUpdate() error = %v", err)
		}
		if result.Status != compozyupdate.ApplyStatusAccepted || result.OperationID == "" || !spawned {
			t.Fatalf("ApplyUpdate() = %#v spawned=%t, want durable acceptance", result, spawned)
		}
	})

	t.Run("Should keep accepted truth when detached launch failure is journaled", func(t *testing.T) {
		t.Parallel()

		store := newDaemonSettingsOperationStore(t)
		manager := stubSettingsUpdateManager{
			store: store,
			planFn: func(
				_ context.Context,
				_ compozyupdate.Actor,
				targets []compozyupdate.Target,
				holder compozyupdate.Holder,
			) (compozyupdate.OperationRequest, error) {
				return daemonSettingsOperationRequest(holder, targets), nil
			},
		}
		controller := settingsUpdateController{
			manager: manager,
			holder: func(compozyupdate.Actor) (compozyupdate.Holder, error) {
				return daemonSettingsHolder(t), nil
			},
			spawn: func(context.Context, *compozyupdate.Operation) error {
				return errors.New("detached launch failed")
			},
		}
		result, err := controller.ApplyUpdate(t.Context(), compozyupdate.TargetRuntime)
		if err != nil {
			t.Fatalf("ApplyUpdate() error = %v", err)
		}
		if result.Status != compozyupdate.ApplyStatusAccepted || result.OperationID == "" {
			t.Fatalf("ApplyUpdate() = %#v, want accepted durable operation", result)
		}
		archived, err := store.ReadArchived(t.Context(), result.OperationID)
		if err != nil {
			t.Fatalf("ReadArchived() error = %v", err)
		}
		if archived == nil || archived.Runtime == nil || archived.Runtime.Phase != compozyupdate.PhaseFailed ||
			!strings.Contains(archived.LastError, "detached launch failed") {
			t.Fatalf("archived operation = %#v, want journaled launch failure", archived)
		}
	})

	for _, shellRunning := range []bool{false, true} {
		name := "Should stage an accepted app update while the shell is closed"
		if shellRunning {
			name = "Should stage an accepted app update while the shell is running"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newDaemonSettingsOperationStore(t)
			manager := stubSettingsUpdateManager{
				store: store,
				planFn: func(
					_ context.Context,
					actor compozyupdate.Actor,
					targets []compozyupdate.Target,
					holder compozyupdate.Holder,
				) (compozyupdate.OperationRequest, error) {
					if actor != compozyupdate.ActorWeb ||
						!slices.Equal(targets, []compozyupdate.Target{compozyupdate.TargetApp}) {
						t.Fatalf("PlanOperation() actor/targets = %q/%v", actor, targets)
					}
					return daemonSettingsOperationRequest(holder, targets), nil
				},
			}
			controller := settingsUpdateController{
				manager: manager,
				holder: func(compozyupdate.Actor) (compozyupdate.Holder, error) {
					return daemonSettingsHolder(t), nil
				},
				spawn: func(ctx context.Context, operation *compozyupdate.Operation) error {
					staged, err := store.Transition(
						ctx,
						operation.ID,
						operation.Holder.ExecutorGeneration,
						operation.Revision,
						compozyupdate.Transition{
							Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
							Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseStaged, Percent: 100,
						},
					)
					if err != nil {
						return err
					}
					_, err = store.Transition(ctx, staged.ID, staged.Holder.ExecutorGeneration, staged.Revision,
						compozyupdate.Transition{
							Kind: compozyupdate.TransitionWaitForApp, Actor: compozyupdate.ActorDaemon,
							Target: compozyupdate.TargetApp, Percent: -1,
						})
					return err
				},
			}

			result, err := controller.ApplyUpdate(t.Context(), compozyupdate.TargetApp)
			if err != nil {
				t.Fatalf("ApplyUpdate() error = %v", err)
			}
			if result.Status != compozyupdate.ApplyStatusAccepted || result.OperationID == "" {
				t.Fatalf("ApplyUpdate() = %#v, want accepted app operation", result)
			}

			operation, err := store.Read(t.Context())
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			app := &compozyupdate.AppTrackState{Running: shellRunning, Status: compozyupdate.StatusAvailable}
			projection := compozyupdate.ProjectMultiState(compozyupdate.State{}, app, operation)
			if operation == nil || operation.App == nil || operation.App.Phase != compozyupdate.PhaseStaged ||
				operation.Waiting != compozyupdate.WaitingForApp || projection.App.Status != compozyupdate.StatusStaged {
				t.Fatalf("operation/projection = %#v/%#v, want dormant staged app", operation, projection.App)
			}
		})
	}

	t.Run("Should return the existing holder when acquisition is blocked", func(t *testing.T) {
		t.Parallel()

		store := newDaemonSettingsOperationStore(t)
		request := daemonSettingsOperationRequest(
			daemonSettingsHolder(t),
			[]compozyupdate.Target{compozyupdate.TargetRuntime},
		)
		active, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire(seed) error = %v", err)
		}
		controller := settingsUpdateController{
			manager: stubSettingsUpdateManager{
				store: store,
				planFn: func(context.Context, compozyupdate.Actor, []compozyupdate.Target, compozyupdate.Holder) (compozyupdate.OperationRequest, error) {
					second := request
					second.Holder.ExecutorGeneration = "generation-2"
					return second, nil
				},
			},
			holder: func(compozyupdate.Actor) (compozyupdate.Holder, error) {
				return daemonSettingsHolder(t), nil
			},
			spawn: func(context.Context, *compozyupdate.Operation) error {
				t.Fatal("spawn called for blocked operation")
				return nil
			},
		}

		result, err := controller.ApplyUpdate(t.Context(), compozyupdate.TargetRuntime)
		if err != nil {
			t.Fatalf("ApplyUpdate() error = %v", err)
		}
		if result.Status != compozyupdate.ApplyStatusBlocked || result.OperationID != active.ID ||
			result.Holder == nil {
			t.Fatalf("ApplyUpdate() = %#v, want blocked holder", result)
		}
	})
}

func TestBackgroundUpdateRuntimeHonorsDaemonOwnedAppConfig(t *testing.T) {
	t.Run("Should disable both-track checks while keeping recovery active", func(t *testing.T) {
		t.Parallel()

		var checkCalls atomic.Int32
		recovered := make(chan struct{}, 1)
		runtime, err := newBackgroundUpdateRuntime(
			stubSettingsUpdateManager{
				checkAllFn: func(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error) {
					checkCalls.Add(1)
					return compozyupdate.MultiState{}, nil, nil
				},
			},
			compozyconfig.AppConfig{UpdateCheck: false, UpdateCheckInterval: 15 * time.Minute},
			testutil.DiscardLogger(),
			func(context.Context) error {
				recovered <- struct{}{}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("newBackgroundUpdateRuntime() error = %v", err)
		}
		if err := runtime.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		select {
		case <-recovered:
		case <-time.After(time.Second):
			t.Fatal("background recovery did not run")
		}
		if got := checkCalls.Load(); got != 0 {
			t.Fatalf("CheckAll() calls = %d, want zero when update_check=false", got)
		}
	})

	t.Run("Should perform one shared runtime and app check on startup", func(t *testing.T) {
		t.Parallel()

		checked := make(chan struct{}, 1)
		runtime, err := newBackgroundUpdateRuntime(
			stubSettingsUpdateManager{
				checkAllFn: func(_ context.Context, opts compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error) {
					if !opts.ForceRefresh || !opts.AllowCachedOnFailure {
						t.Fatalf("background CheckOptions = %#v", opts)
					}
					checked <- struct{}{}
					return compozyupdate.MultiState{}, nil, nil
				},
			},
			compozyconfig.AppConfig{UpdateCheck: true, UpdateCheckInterval: 15 * time.Minute},
			testutil.DiscardLogger(),
			func(context.Context) error { return nil },
		)
		if err != nil {
			t.Fatalf("newBackgroundUpdateRuntime() error = %v", err)
		}
		if err := runtime.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		select {
		case <-checked:
		case <-time.After(time.Second):
			t.Fatal("background check did not run")
		}
	})

	t.Run("Should fail an expired pre-swap operation without relaunching it", func(t *testing.T) {
		t.Parallel()

		store := newDaemonSettingsOperationStore(t)
		executor := daemonSettingsHolder(t)
		holder := daemonSettingsHolder(t)
		holder.PID = 99_999_999
		holder.LeaseExpiresAt = time.Now().UTC().Add(-time.Minute)
		request := daemonSettingsOperationRequest(holder, []compozyupdate.Target{compozyupdate.TargetRuntime})
		request.Deadline = time.Now().UTC().Add(-time.Minute)
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		daemon := &Daemon{
			now:              func() time.Time { return time.Now().UTC() },
			pid:              func() int { return executor.PID },
			processStartedAt: func(int) (time.Time, error) { return executor.PIDStartTime, nil },
			executable:       func() (string, error) { return "/bin/false", nil },
			startDetached: func(context.Context, detachedStartRequest) (restartProcess, error) {
				t.Fatal("expired pre-swap operation launched a coordinator")
				return nil, nil
			},
		}
		manager := stubSettingsUpdateManager{store: store}
		if err := recoverDaemonUpdateOperation(t.Context(), daemon, manager); err != nil {
			t.Fatalf("recoverDaemonUpdateOperation() error = %v", err)
		}
		archived, err := store.ReadArchived(t.Context(), operation.ID)
		if err != nil {
			t.Fatalf("ReadArchived() error = %v", err)
		}
		if archived == nil || archived.Runtime == nil || archived.Runtime.Phase != compozyupdate.PhaseFailed ||
			!strings.Contains(archived.LastError, "deadline expired") {
			t.Fatalf("archived operation = %#v, want deadline failure", archived)
		}
	})

	t.Run("Should relaunch a pending app coordinator after its holder dies", func(t *testing.T) {
		t.Parallel()

		store := newDaemonSettingsOperationStore(t)
		executor := daemonSettingsHolder(t)
		holder := daemonSettingsHolder(t)
		holder.PID = 99_999_999
		holder.LeaseExpiresAt = time.Now().UTC().Add(-time.Minute)
		operation, err := store.Acquire(
			t.Context(),
			daemonSettingsOperationRequest(holder, []compozyupdate.Target{compozyupdate.TargetApp}),
		)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		launched := make(chan detachedStartRequest, 1)
		daemon := &Daemon{
			now:              func() time.Time { return time.Now().UTC() },
			pid:              func() int { return executor.PID },
			processStartedAt: func(int) (time.Time, error) { return executor.PIDStartTime, nil },
			executable:       func() (string, error) { return "/bin/compozy", nil },
			startDetached: func(_ context.Context, request detachedStartRequest) (restartProcess, error) {
				launched <- request
				return restartProcessStub{pid: 4242}, nil
			},
		}
		manager := stubSettingsUpdateManager{store: store}
		if err := recoverDaemonUpdateOperation(t.Context(), daemon, manager); err != nil {
			t.Fatalf("recoverDaemonUpdateOperation() error = %v", err)
		}
		select {
		case request := <-launched:
			if request.binary != "/bin/compozy" || !slices.Contains(request.args, operation.ID) {
				t.Fatalf("detached coordinator request = %#v, want recovered app operation", request)
			}
		case <-time.After(time.Second):
			t.Fatal("pending app coordinator was not relaunched")
		}
		recovered, err := store.Read(t.Context())
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if recovered == nil || recovered.App == nil || recovered.App.Phase != compozyupdate.PhasePending ||
			recovered.Holder == nil || recovered.Holder.ExecutorGeneration == operation.Holder.ExecutorGeneration {
			t.Fatalf("recovered operation = %#v, want pending app with a new holder", recovered)
		}
	})
}

func newDaemonSettingsOperationStore(t *testing.T) *compozyupdate.OperationStore {
	t.Helper()
	store, err := compozyupdate.NewOperationStore(compozyconfig.HomePaths{HomeDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatalf("NewOperationStore() error = %v", err)
	}
	return store
}

func daemonSettingsHolder(t *testing.T) compozyupdate.Holder {
	t.Helper()
	now := time.Now().UTC()
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatalf("procutil.StartedAt() error = %v", err)
	}
	return compozyupdate.Holder{
		PID: os.Getpid(), PIDStartTime: startedAt, Surface: compozyupdate.ActorDaemon,
		ExecutorGeneration: "generation-1", LeaseExpiresAt: now.Add(time.Hour),
	}
}

func daemonSettingsOperationRequest(
	holder compozyupdate.Holder,
	targets []compozyupdate.Target,
) compozyupdate.OperationRequest {
	now := time.Now().UTC()
	request := compozyupdate.OperationRequest{
		RequestedBy: compozyupdate.ActorWeb, Targets: targets, Holder: holder, Deadline: now.Add(time.Hour),
	}
	if slices.Contains(targets, compozyupdate.TargetRuntime) {
		request.Runtime = &compozyupdate.RuntimeOperationState{
			ArtifactIdentity: compozyupdate.ArtifactIdentity{
				FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
				Asset: "runtime.tar.gz", Digest: "sha256:runtime",
			},
			InstallMethod: compozyupdate.InstallMethodDirectBinary, Phase: compozyupdate.PhasePending,
		}
	}
	if slices.Contains(targets, compozyupdate.TargetApp) {
		request.App = &compozyupdate.AppOperationState{
			ArtifactIdentity: compozyupdate.ArtifactIdentity{
				FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
				Asset: "desktop-app.zip", Digest: "sha256:app",
			},
			AttemptID: "attempt-1", Phase: compozyupdate.PhasePending,
		}
	}
	return request
}
