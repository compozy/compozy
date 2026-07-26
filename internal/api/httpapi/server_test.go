package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func TestNewHonorsOptionsAndDefaults(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := gin.New()
	startedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return startedAt.Add(time.Second) }
	customLoader := func(name string, _ aghconfig.HomePaths) (aghconfig.AgentDef, error) {
		return aghconfig.AgentDef{Name: name, Provider: "fake", Prompt: "hello"}, nil
	}
	store := memory.NewStore(filepath.Join(t.TempDir(), "memory"))
	dream := &stubDreamTrigger{}
	extensionService := &stubExtensionService{}
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithStartedAt(startedAt),
		WithNow(now),
		WithPollInterval(25*time.Millisecond),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithMemoryStore(store),
		WithDreamTrigger(dream),
		WithAgentLoader(customLoader),
		WithExtensionService(extensionService),
		WithEngine(engine),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if server.Port() != cfg.HTTP.Port {
		t.Fatalf("Port() = %d, want %d", server.Port(), cfg.HTTP.Port)
	}
	if server.engine != engine {
		t.Fatal("expected custom gin engine to be used")
	}
	if server.startedAt != startedAt {
		t.Fatalf("startedAt = %v, want %v", server.startedAt, startedAt)
	}
	if server.now == nil || !server.now().Equal(now()) {
		t.Fatalf("now() = %v, want %v", server.now(), now())
	}
	if server.pollInterval != 25*time.Millisecond {
		t.Fatalf("pollInterval = %v, want 25ms", server.pollInterval)
	}
	if server.handlers.AgentLoader == nil {
		t.Fatal("expected custom agent loader to be installed")
	}
	if server.handlers.MemoryStore != store {
		t.Fatal("expected memory store option to be installed")
	}
	if server.handlers.DreamTrigger != dream {
		t.Fatal("expected dream trigger option to be installed")
	}
	if server.handlers.Extensions != extensionService {
		t.Fatal("expected extension service option to be installed")
	}
	if server.extensions == nil || server.handlers.Extensions == nil {
		t.Fatal("expected extension service option to be installed")
	}
}

func TestPortHandlesNilServer(t *testing.T) {
	var server *Server
	if server.Port() != 0 {
		t.Fatalf("Port(nil) = %d, want 0", server.Port())
	}
}

func TestNewWithHomePathsRealignsDefaultConfig(t *testing.T) {
	t.Run("Should use overridden home paths for the default daemon socket", func(t *testing.T) {
		processHome := filepath.Join(t.TempDir(), "process-home")
		t.Setenv("AGH_HOME", processHome)
		homePaths := newTestHomePaths(t)

		server, err := New(
			WithHomePaths(homePaths),
			WithSessionManager(stubSessionManager{}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if got, want := server.config.Daemon.Socket, homePaths.DaemonSocket; got != want {
			t.Fatalf("config daemon socket = %q, want %q", got, want)
		}
	})
}

func TestNewRequiresSessionManagerTaskServiceObserverAndWorkspaceResolver(t *testing.T) {
	homePaths := newTestHomePaths(t)

	if _, err := New(WithHomePaths(homePaths), WithObserver(stubObserver{})); err == nil {
		t.Fatal("New() without session manager error = nil, want non-nil")
	}
	if _, err := New(WithHomePaths(homePaths), WithSessionManager(stubSessionManager{})); err == nil {
		t.Fatal("New() without task service error = nil, want non-nil")
	}
	if _, err := New(
		WithHomePaths(homePaths),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
	); err == nil {
		t.Fatal("New() without observer error = nil, want non-nil")
	}
	if _, err := New(
		WithHomePaths(homePaths),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
	); err == nil {
		t.Fatal("New() without workspace resolver error = nil, want non-nil")
	}
}

func TestNewRejectsResourceAuthWithoutResourceService(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	_, err := New(
		WithHomePaths(homePaths),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithResourceOperatorAuth(func(*gin.Context) {}),
	)
	if err == nil {
		t.Fatal("New() with resource auth and no resource service error = nil, want non-nil")
	}
}

func TestServerStartAndShutdownServeRequests(t *testing.T) {
	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) { return nil, nil },
		}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{
			HealthFn: func(context.Context) (observe.Health, error) { return observe.Health{Status: "ok"}, nil },
		}),
		WithWorkspaceResolver(stubWorkspaceService{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		mustURL(cfg.HTTP.Host, server.Port(), "/api/status"),
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("http.DefaultClient.Do() error = %v", err)
	}
	body := resp.Body
	defer func() {
		if body != nil {
			_ = body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	req, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		mustURL(cfg.HTTP.Host, server.Port(), "/api/status"),
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	respAfterShutdown, err := http.DefaultClient.Do(req)
	if respAfterShutdown != nil {
		_ = respAfterShutdown.Body.Close()
	}
	if err == nil {
		t.Fatal("expected request after shutdown to fail")
	}
}

func TestServerStartRejectsNilContextAndDuplicateStart(t *testing.T) {
	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	if err := server.Start(context.Background()); err == nil {
		t.Fatal("Start(second) error = nil, want non-nil")
	}
}

func TestLoopbackServerAllowsSettingsAndExtensionMutations(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	settingsService := &stubSettingsService{}
	restartController := &stubSettingsRestartController{}
	var (
		installedReq contract.InstallExtensionRequest
		enabledName  string
		disabledName string
	)
	extensionService := &stubExtensionService{
		InstallFn: func(_ context.Context, req contract.InstallExtensionRequest, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			installedReq = req
			return contract.ExtensionPayload{Name: "demo", State: "registered"}, nil
		},
		EnableFn: func(_ context.Context, name string, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			enabledName = name
			return contract.ExtensionPayload{Name: name, Enabled: true, State: "active"}, nil
		},
		DisableFn: func(_ context.Context, name string, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			disabledName = name
			return contract.ExtensionPayload{Name: name, Enabled: false, State: "inactive"}, nil
		},
	}

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithSettingsService(settingsService),
		WithSettingsRestartController(restartController),
		WithExtensionService(extensionService),
		WithVaultService(stubVaultService{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	testCases := []struct {
		name       string
		method     string
		path       string
		body       []byte
		wantStatus int
		assert     func(t *testing.T)
	}{
		{
			name:       "Should patch settings sections on loopback HTTP",
			method:     http.MethodPatch,
			path:       "/api/settings/general",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.UpdateSettingsGeneralRequest{
				Config: contract.SettingsGeneralConfigPayload{
					Defaults: contract.SettingsDefaultsPayload{Agent: "coder"},
					Limits:   contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
					Permissions: contract.SettingsPermissionsPayload{
						Mode: contract.SettingsPermissionModeApproveReads,
					},
					SessionTimeout: "30m",
					HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
					Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
				},
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastUpdateSectionRequest.Section != settingspkg.SectionGeneral {
					t.Fatalf(
						"LastUpdateSectionRequest.Section = %q, want %q",
						settingsService.LastUpdateSectionRequest.Section,
						settingspkg.SectionGeneral,
					)
				}
			},
		},
		{
			name:       "Should put MCP collection items on loopback HTTP",
			method:     http.MethodPut,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.PutSettingsMCPServerRequest{
				Server: contract.SettingsMCPServerPayload{Name: "server-a", Command: "mcpd"},
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastPutCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastPutCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastPutCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastPutCollectionRequest = %#v", settingsService.LastPutCollectionRequest)
				}
			},
		},
		{
			name:       "Should delete MCP collection items on loopback HTTP",
			method:     http.MethodDelete,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastDeleteCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastDeleteCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastDeleteCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastDeleteCollectionRequest = %#v", settingsService.LastDeleteCollectionRequest)
				}
			},
		},
		{
			name:       "Should trigger daemon restart actions on loopback HTTP",
			method:     http.MethodPost,
			path:       "/api/settings/actions/restart",
			body:       []byte(`{}`),
			wantStatus: http.StatusAccepted,
			assert: func(t *testing.T) {
				t.Helper()
				if restartController.RequestRestartCalls != 1 {
					t.Fatalf("RequestRestartCalls = %d, want 1", restartController.RequestRestartCalls)
				}
			},
		},
		{
			name:       "Should install extensions on loopback HTTP",
			method:     http.MethodPost,
			path:       "/api/extensions",
			wantStatus: http.StatusCreated,
			body: mustJSONBody(t, contract.InstallExtensionRequest{
				Path:     "/extensions/demo",
				Checksum: "sha256-demo",
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if installedReq.Path != "/extensions/demo" || installedReq.Checksum != "sha256-demo" {
					t.Fatalf("installedReq = %#v", installedReq)
				}
			},
		},
		{
			name:       "Should enable extensions on loopback HTTP",
			method:     http.MethodPost,
			path:       "/api/extensions/demo/enable",
			body:       []byte(`{}`),
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if enabledName != "demo" {
					t.Fatalf("enabledName = %q, want %q", enabledName, "demo")
				}
			},
		},
		{
			name:       "Should disable extensions on loopback HTTP",
			method:     http.MethodPost,
			path:       "/api/extensions/demo/disable",
			body:       []byte(`{}`),
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if disabledName != "demo" {
					t.Fatalf("disabledName = %q, want %q", disabledName, "demo")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doServerRequest(
				t,
				http.DefaultClient,
				tc.method,
				mustURL("127.0.0.1", server.Port(), tc.path),
				tc.body,
			)
			defer func() {
				_ = resp.Body.Close()
			}()
			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tc.method,
					tc.path,
					resp.StatusCode,
					tc.wantStatus,
					string(body),
				)
			}
			if tc.assert != nil {
				tc.assert(t)
			}
		})
	}
}

func TestLoopbackServerRejectsMismatchedSettingsItemNames(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithSettingsService(&stubSettingsService{}),
		WithSettingsRestartController(&stubSettingsRestartController{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	readOnly := true
	testCases := []struct {
		name      string
		path      string
		body      []byte
		wantError string
	}{
		{
			name: "Should reject MCP server body names that differ from the path key",
			path: "/api/settings/mcp-servers/server-a",
			body: mustJSONBody(t, contract.PutSettingsMCPServerRequest{
				Server: contract.SettingsMCPServerPayload{Name: "server-b", Command: "mcpd"},
			}),
			wantError: `settings validation error: mcp-servers.server.name must match path name "server-a"`,
		},
		{
			name: "Should reject hook declaration names that differ from the path key",
			path: "/api/settings/hooks/capture",
			body: mustJSONBody(t, contract.PutSettingsHookRequest{
				Declaration: contract.SettingsHookDeclarationPayload{
					Name:         "other",
					Event:        hookspkg.HookToolPreCall,
					Mode:         hookspkg.HookModeAsync,
					ExecutorKind: hookspkg.HookExecutorSubprocess,
					Command:      "/bin/capture",
					Matcher: hookspkg.HookMatcher{
						ToolID:       "agh__read",
						ToolReadOnly: &readOnly,
					},
				},
			}),
			wantError: `settings validation error: hooks.declaration.name must match path name "capture"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doServerRequest(
				t,
				http.DefaultClient,
				http.MethodPut,
				mustURL("127.0.0.1", server.Port(), tc.path),
				tc.body,
			)
			defer func() {
				_ = resp.Body.Close()
			}()
			if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("PUT %s status = %d, want %d; body=%s", tc.path, got, want, string(body))
			}

			var payload contract.ErrorPayload
			decodeServerJSON(t, resp, &payload)
			if got := payload.Error; got != tc.wantError {
				t.Fatalf("payload.Error = %q, want %q", got, tc.wantError)
			}
		})
	}
}

func TestLoopbackServerMapsDuplicateExtensionInstallToConflict(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithExtensionService(&stubExtensionService{
			InstallFn: func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				return contract.ExtensionPayload{}, extensionpkg.ErrExtensionExists
			},
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	resp := doServerRequest(
		t,
		http.DefaultClient,
		http.MethodPost,
		mustURL("127.0.0.1", server.Port(), "/api/extensions"),
		mustJSONBody(t, contract.InstallExtensionRequest{
			Path:     "/extensions/demo",
			Checksum: "sha256-demo",
		}),
	)
	defer func() {
		_ = resp.Body.Close()
	}()
	if got, want := resp.StatusCode, http.StatusConflict; got != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/extensions status = %d, want %d; body=%s", got, want, string(body))
	}
}

func TestNonLoopbackServerBlocksDaemonAPIRoutes(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "0.0.0.0"
	cfg.HTTP.Port = freeTCPPort(t)

	settingsService := &stubSettingsService{}
	restartController := &stubSettingsRestartController{}
	extensionService := &stubExtensionService{
		ListFn: func(context.Context) ([]contract.ExtensionPayload, error) {
			return []contract.ExtensionPayload{{Name: "demo", State: "registered"}}, nil
		},
		StatusFn: func(_ context.Context, name string) (contract.ExtensionPayload, error) {
			return contract.ExtensionPayload{Name: name, State: "registered"}, nil
		},
		InstallFn: func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			t.Fatal("Install should not be called on non-loopback HTTP bind")
			return contract.ExtensionPayload{}, nil
		},
		EnableFn: func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			t.Fatal("Enable should not be called on non-loopback HTTP bind")
			return contract.ExtensionPayload{}, nil
		},
		DisableFn: func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			t.Fatal("Disable should not be called on non-loopback HTTP bind")
			return contract.ExtensionPayload{}, nil
		},
	}

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithSettingsService(settingsService),
		WithSettingsRestartController(restartController),
		WithExtensionService(extensionService),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	readCases := []struct {
		name string
		path string
	}{
		{name: "Should block reading the general settings section", path: "/api/settings/general"},
		{name: "Should block reading restart status", path: "/api/settings/actions/restart/op-123"},
		{name: "Should block listing extensions", path: "/api/extensions"},
		{name: "Should block reading extension status", path: "/api/extensions/demo"},
	}
	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doServerRequest(
				t,
				http.DefaultClient,
				http.MethodGet,
				mustURL("127.0.0.1", server.Port(), tc.path),
				nil,
			)
			defer func() {
				_ = resp.Body.Close()
			}()
			if got, want := resp.StatusCode, http.StatusForbidden; got != want {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("GET %s status = %d, want %d; body=%s", tc.path, got, want, string(body))
			}
			var payload contract.ErrorPayload
			decodeServerJSON(t, resp, &payload)
			if got, want := payload.Error, errLoopbackAPIRequired.Error(); got != want {
				t.Fatalf("payload.Error = %q, want %q", got, want)
			}
		})
	}

	t.Run("Should block daemon log tail reads on non-loopback HTTP", func(t *testing.T) {
		resp := doServerRequest(
			t,
			http.DefaultClient,
			http.MethodGet,
			mustURL("127.0.0.1", server.Port(), "/api/settings/observability/log-tail"),
			nil,
		)
		defer func() {
			_ = resp.Body.Close()
		}()
		if got, want := resp.StatusCode, http.StatusForbidden; got != want {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET /api/settings/observability/log-tail status = %d, want %d; body=%s", got, want, string(body))
		}
		var payload contract.ErrorPayload
		decodeServerJSON(t, resp, &payload)
		if got, want := payload.Error, errLoopbackAPIRequired.Error(); got != want {
			t.Fatalf("payload.Error = %q, want %q", got, want)
		}
	})

	t.Run("Should block vault metadata reads on non-loopback HTTP", func(t *testing.T) {
		resp := doServerRequest(
			t,
			http.DefaultClient,
			http.MethodGet,
			mustURL("127.0.0.1", server.Port(), "/api/vault/secrets?namespace=sessions"),
			nil,
		)
		defer func() {
			_ = resp.Body.Close()
		}()
		if got, want := resp.StatusCode, http.StatusForbidden; got != want {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET /api/vault/secrets status = %d, want %d; body=%s", got, want, string(body))
		}
		var payload contract.ErrorPayload
		decodeServerJSON(t, resp, &payload)
		if got, want := payload.Error, errLoopbackAPIRequired.Error(); got != want {
			t.Fatalf("payload.Error = %q, want %q", got, want)
		}
	})

	mutationCases := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "Should block section patches",
			method: http.MethodPatch,
			path:   "/api/settings/general",
			body:   []byte(`{}`),
		},
		{
			name:   "Should block provider writes",
			method: http.MethodPut,
			path:   "/api/settings/providers/demo",
			body:   []byte(`{}`),
		},
		{name: "Should block hook deletions", method: http.MethodDelete, path: "/api/settings/hooks/capture"},
		{
			name:   "Should block restart actions",
			method: http.MethodPost,
			path:   "/api/settings/actions/restart",
			body:   []byte(`{}`),
		},
		{name: "Should block extension installs", method: http.MethodPost, path: "/api/extensions", body: []byte(`{}`)},
		{
			name:   "Should block tool approvals",
			method: http.MethodPost,
			path:   "/api/tools/agh__skill_view/approvals",
			body:   []byte(`{}`),
		},
		{
			name:   "Should block tool invocation",
			method: http.MethodPost,
			path:   "/api/tools/agh__skill_view/invoke",
			body:   []byte(`{}`),
		},
		{
			name:   "Should block extension enables",
			method: http.MethodPost,
			path:   "/api/extensions/demo/enable",
			body:   []byte(`{}`),
		},
	}
	for _, tc := range mutationCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doServerRequest(
				t,
				http.DefaultClient,
				tc.method,
				mustURL("127.0.0.1", server.Port(), tc.path),
				tc.body,
			)
			defer func() {
				_ = resp.Body.Close()
			}()
			if got, want := resp.StatusCode, http.StatusForbidden; got != want {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("%s %s status = %d, want %d; body=%s", tc.method, tc.path, got, want, string(body))
			}
			var payload contract.ErrorPayload
			decodeServerJSON(t, resp, &payload)
			if got, want := payload.Error, errLoopbackAPIRequired.Error(); got != want {
				t.Fatalf("payload.Error = %q, want %q", got, want)
			}
		})
	}
}

func TestWaitForServeDone(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if err := waitForServeDone(context.Background(), done); err != nil {
		t.Fatalf("waitForServeDone(done) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := waitForServeDone(ctx, make(chan struct{})); err == nil {
		t.Fatal("waitForServeDone(timeout) error = nil, want non-nil")
	}
}

func doServerRequest(t *testing.T, client *http.Client, method, url string, body []byte) *http.Response {
	t.Helper()

	var reader io.Reader = http.NoBody
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reader)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	return resp
}

func decodeServerJSON(t *testing.T, resp *http.Response, dest any) {
	t.Helper()
	defer func() {
		_ = resp.Body.Close()
	}()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}
}

func TestServerStartReportsListenFailure(t *testing.T) {
	homePaths := newTestHomePaths(t)
	port := freeTCPPort(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = port

	first, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
	)
	if err != nil {
		t.Fatalf("first New() error = %v", err)
	}
	if err := first.Start(context.Background()); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	defer func() {
		_ = first.Shutdown(context.Background())
	}()

	second, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
	)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	if err := second.Start(context.Background()); err == nil {
		t.Fatal("second Start() error = nil, want non-nil")
	}
}

func TestShutdownNilServerIsSafe(t *testing.T) {
	var server *Server
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown(nil) error = %v", err)
	}
}

func TestShutdownTimeoutIsReported(t *testing.T) {
	server := &Server{serveDone: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := server.Shutdown(ctx)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown(timeout) error = %v, want deadline exceeded", err)
	}
}
