package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/gateway"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestNewHonorsOptionsAndDefaults(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := gin.New()
	startedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return startedAt.Add(time.Second) }
	customLoader := func(name string, _ compozyconfig.HomePaths) (compozyconfig.AgentDef, error) {
		return compozyconfig.AgentDef{Name: name, Provider: "fake", Prompt: "hello"}, nil
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
		t.Setenv("COMPOZY_HOME", processHome)
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

func TestGatewayTierServerConstruction(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an explicit zero port for OS assignment", func(t *testing.T) {
		t.Parallel()

		server, err := New(
			WithHomePaths(newTestHomePaths(t)),
			WithHost("127.0.0.1"),
			WithPort(0),
			WithSurfaceSet(SurfaceSetPrivate),
			WithGatewayChallenge(gateway.TierPrivate, gateway.NewChallengeRegistry()),
			WithGatewayService(gatewayHTTPServiceStub{}),
			WithDeviceAuth(gatewayHTTPAuthenticatorStub{}),
			WithSessionManager(stubSessionManager{}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if server.port != 0 {
			t.Fatalf("configured port = %d, want explicit 0", server.port)
		}
		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
			defer cancelShutdown()
			if err := server.Shutdown(shutdownCtx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		if server.Port() <= 0 {
			t.Fatalf("resolved port = %d, want OS-assigned port", server.Port())
		}
	})

	t.Run("Should reject every non-loopback bind for a tier listener", func(t *testing.T) {
		t.Parallel()

		_, err := New(
			WithHomePaths(newTestHomePaths(t)),
			WithHost("0.0.0.0"),
			WithSurfaceSet(SurfaceSetPrivate),
			WithGatewayService(gatewayHTTPServiceStub{}),
			WithDeviceAuth(gatewayHTTPAuthenticatorStub{}),
			WithSessionManager(stubSessionManager{}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("New(non-loopback tier) error = %v, want loopback refusal", err)
		}
	})

	t.Run("Should require Gateway on every operator tier listener", func(t *testing.T) {
		t.Parallel()
		for _, surfaceSet := range []SurfaceSet{SurfaceSetPrivate, SurfaceSetPublicOperator, SurfaceSetPublicCombined} {
			_, err := New(
				WithHomePaths(newTestHomePaths(t)),
				WithHost("127.0.0.1"),
				WithSurfaceSet(surfaceSet),
				WithDeviceAuth(gatewayHTTPAuthenticatorStub{}),
				WithSessionManager(stubSessionManager{}),
				WithTaskService(&stubTaskManager{}),
				WithObserver(stubObserver{}),
				WithWorkspaceResolver(stubWorkspaceService{}),
			)
			if err == nil || !strings.Contains(err.Error(), "gateway service") {
				t.Fatalf("New(%s without Gateway) error = %v, want gateway-service refusal", surfaceSet, err)
			}
		}
	})

	t.Run("Should require admission control on every non-local tier listener", func(t *testing.T) {
		t.Parallel()
		_, err := New(
			WithHomePaths(newTestHomePaths(t)),
			WithHost("127.0.0.1"),
			WithSurfaceSet(SurfaceSetPublicIngress),
			WithSessionManager(stubSessionManager{}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err == nil || !strings.Contains(err.Error(), "admission") {
			t.Fatalf("New(public ingress without admission) error = %v", err)
		}
	})
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
		t.Fatalf("http.DefaultClient.Do() error = %v", err)
	}
	defer func() {
		if resp.Body != nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("response body Close() error = %v", closeErr)
			}
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
	if err == nil {
		if respAfterShutdown != nil && respAfterShutdown.Body != nil {
			if closeErr := respAfterShutdown.Body.Close(); closeErr != nil {
				t.Errorf("response body Close() after shutdown error = %v", closeErr)
			}
		}
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
		shutdownServerForTest(t, server)
	}()
	if err := server.Start(context.Background()); err == nil {
		t.Fatal("Start(second) error = nil, want non-nil")
	}
}

func TestServerShutdownLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil context before mutating the active generation", func(t *testing.T) {
		t.Parallel()

		listener := newDelayedFirstCloseListener()
		listener.releaseFirst()
		serveDone := make(chan struct{})
		close(serveDone)
		server := &Server{
			generation: &serverGeneration{listener: listener, serveDone: serveDone},
			state:      serverStateRunning,
		}

		var nilCtx context.Context
		if err := server.Shutdown(nilCtx); err == nil {
			t.Fatal("Shutdown(nil) error = nil, want context validation failure")
		}
		if got := listener.closeCalls.Load(); got != 0 {
			t.Fatalf("listener close calls = %d, want 0", got)
		}
	})

	t.Run("Should reject start while a generation starts", func(t *testing.T) {
		t.Parallel()

		server := &Server{
			generation: &serverGeneration{startDone: make(chan struct{})},
			state:      serverStateStarting,
		}
		if err := server.Start(t.Context()); !errors.Is(err, errServerStartInProgress) {
			t.Fatalf("Start() error = %v, want start-in-progress error", err)
		}
	})

	t.Run("Should report a terminal serve error before rearming", func(t *testing.T) {
		t.Parallel()

		terminalErr := errors.New("terminal listener failure")
		listener := newTerminalServeListener(t)
		engine := gin.New()
		serveDone := make(chan struct{})
		generation := &serverGeneration{
			httpServer: &http.Server{
				Handler:           engine,
				ReadHeaderTimeout: defaultReadHeaderTimeout,
				ReadTimeout:       defaultReadTimeout,
				IdleTimeout:       defaultIdleTimeout,
			},
			listener:  listener,
			serveDone: serveDone,
		}
		server := &Server{
			host:       "127.0.0.1",
			port:       0,
			logger:     discardLogger(),
			engine:     engine,
			generation: generation,
			state:      serverStateRunning,
		}
		go server.serve(generation, listener.Addr().String())
		waitForServerSignal(t, listener.acceptStarted, "Serve to call Accept")
		listener.fail(terminalErr)
		waitForServerSignal(t, serveDone, "Serve to report its terminal error")

		shutdownCtx, cancelShutdown := context.WithTimeout(t.Context(), time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownCtx); !errors.Is(err, terminalErr) {
			t.Fatalf("Shutdown() error = %v, want terminal error", err)
		}

		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() after terminal shutdown error = %v", err)
		}
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancelCleanup()
		if err := server.Shutdown(cleanupCtx); err != nil {
			t.Fatalf("Shutdown() after rearm error = %v", err)
		}
	})

	t.Run("Should fence delayed generation cleanup from a rearmed generation", func(t *testing.T) {
		t.Parallel()

		server := newWindowManagerLifecycleServer(t)
		t.Cleanup(func() {
			cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
			defer cancelCleanup()
			if err := server.Shutdown(cleanupCtx); err != nil {
				t.Errorf("Shutdown() cleanup error = %v", err)
			}
		})

		listener := newDelayedFirstCloseListener()
		t.Cleanup(listener.releaseFirst)
		serveDone := make(chan struct{})
		close(serveDone)
		generation := &serverGeneration{listener: listener, serveDone: serveDone}
		server.mu.Lock()
		server.generation = generation
		server.state = serverStateRunning
		server.actualPort = 0
		server.mu.Unlock()

		firstShutdownDone := make(chan error, 1)
		go func() {
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelShutdown()
			firstShutdownDone <- server.Shutdown(shutdownCtx)
		}()
		waitForServerSignal(t, listener.firstCloseStarted, "first generation-A listener close")

		secondShutdownDone := make(chan error, 1)
		go func() {
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelShutdown()
			secondShutdownDone <- server.Shutdown(shutdownCtx)
		}()
		if err := waitForServerResult(t, secondShutdownDone, "second generation-A shutdown"); err != nil {
			t.Fatalf("second generation-A Shutdown() error = %v", err)
		}

		server.port = 0
		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() generation B error = %v", err)
		}
		listener.releaseFirst()
		if err := waitForServerResult(t, firstShutdownDone, "delayed generation-A shutdown"); err != nil {
			t.Fatalf("delayed generation-A Shutdown() error = %v", err)
		}

		assertWindowManagerGenerationFunctional(t, server)
	})

	t.Run("Should reject start while an active request drains and rearm after drain", func(t *testing.T) {
		t.Parallel()

		entered := make(chan struct{}, 1)
		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseRequest := func() {
			releaseOnce.Do(func() {
				close(release)
			})
		}
		server := newLifecycleServer(t, entered, release)
		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			releaseRequest()
			cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
			defer cancelCleanup()
			if err := server.Shutdown(cleanupCtx); err != nil {
				t.Errorf("Shutdown() cleanup error = %v", err)
			}
		})

		requestDone := requestLifecycleBlock(t.Context(), server)
		waitForServerSignal(t, entered, "in-flight session list request")

		shutdownStarted := registerServerShutdownSignal(t, server)
		shutdownDone := make(chan error, 1)
		go func() {
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelShutdown()
			shutdownDone <- server.Shutdown(shutdownCtx)
		}()
		waitForServerSignal(t, shutdownStarted, "HTTP shutdown to start draining")

		startErr := server.Start(t.Context())
		releaseRequest()

		if err := waitForServerResult(t, requestDone, "session list request"); err != nil {
			t.Fatal(err)
		}
		if err := waitForServerResult(t, shutdownDone, "shutdown"); err != nil {
			t.Fatal(err)
		}
		if startErr == nil {
			cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
			defer cancelCleanup()
			if err := server.Shutdown(cleanupCtx); err != nil {
				t.Errorf("Shutdown() after unexpected concurrent Start error = %v", err)
			}
			t.Fatal("Start() during shutdown drain error = nil, want non-nil")
		}

		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() after completed drain error = %v", err)
		}
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancelCleanup()
		if err := server.Shutdown(cleanupCtx); err != nil {
			t.Fatalf("Shutdown() after rearm error = %v", err)
		}
	})

	t.Run("Should let concurrent shutdown callers keep independent contexts", func(t *testing.T) {
		t.Parallel()

		entered := make(chan struct{}, 1)
		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseRequest := func() {
			releaseOnce.Do(func() {
				close(release)
			})
		}
		server := newLifecycleServer(t, entered, release)
		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			releaseRequest()
			cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
			defer cancelCleanup()
			if err := server.Shutdown(cleanupCtx); err != nil {
				t.Errorf("Shutdown() cleanup error = %v", err)
			}
		})

		requestDone := requestLifecycleBlock(t.Context(), server)
		waitForServerSignal(t, entered, "in-flight session list request")

		shutdownStarted := registerServerShutdownSignal(t, server)
		shortCtx, cancelShort := context.WithCancel(t.Context())
		defer cancelShort()
		firstShutdownDone := make(chan error, 1)
		go func() {
			firstShutdownDone <- server.Shutdown(shortCtx)
		}()
		waitForServerSignal(t, shutdownStarted, "first HTTP shutdown to start draining")
		cancelShort()

		secondShutdownDone := make(chan error, 1)
		go func() {
			secondCtx, cancelSecond := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelSecond()
			secondShutdownDone <- server.Shutdown(secondCtx)
		}()

		if err := waitForServerResult(t, firstShutdownDone, "first shutdown"); !errors.Is(err, context.Canceled) {
			t.Fatalf("first Shutdown() error = %v, want context canceled", err)
		}
		select {
		case err := <-secondShutdownDone:
			t.Fatalf("second Shutdown() returned before drain completed: %v", err)
		default:
		}

		releaseRequest()
		if err := waitForServerResult(t, requestDone, "session list request"); err != nil {
			t.Fatal(err)
		}
		if err := waitForServerResult(t, secondShutdownDone, "second shutdown"); err != nil {
			t.Fatal(err)
		}
	})
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
		installedReq   contract.InstallExtensionRequest
		enablementReq  contract.SetExtensionEnablementRequest
		enablementName string
	)
	extensionService := &stubExtensionService{
		InstallFn: func(_ context.Context, req contract.InstallExtensionRequest, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
			installedReq = req
			return contract.ExtensionPayload{Name: "demo", State: "registered"}, nil
		},
		SetEnablementFn: func(
			_ context.Context,
			name string,
			req contract.SetExtensionEnablementRequest,
			_ taskpkg.ActorContext,
		) (contract.ExtensionEnablementPayload, error) {
			enablementName = name
			enablementReq = req
			return contract.ExtensionEnablementPayload(req), nil
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
		shutdownServerForTest(t, server)
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
					Limits: contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
					Permissions: contract.SettingsPermissionsPayload{
						Mode: contract.SettingsPermissionModeApproveReads,
					},
					SessionTimeout: "30m",
					HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
					Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/compozy.sock"},
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
				Source: contract.InstallExtensionSourceLocalPath,
				Ref:    "/extensions/demo",
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if installedReq.Source != contract.InstallExtensionSourceLocalPath ||
					installedReq.Ref != "/extensions/demo" {
					t.Fatalf("installedReq = %#v", installedReq)
				}
			},
		},
		{
			name:       "Should set profile extension enablement on loopback HTTP",
			method:     http.MethodPut,
			path:       "/api/extensions/demo/enablement",
			body:       []byte(`{"profile":"finance","enabled":false}`),
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if enablementName != "demo" || enablementReq.Profile != "finance" || enablementReq.Enabled {
					t.Fatalf("enablement = name:%q req:%#v", enablementName, enablementReq)
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
				closeServerTestBody(t, resp.Body)
			}()
			if resp.StatusCode != tc.wantStatus {
				body := readServerResponseBody(t, resp.Body)
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
		shutdownServerForTest(t, server)
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
						ToolID:       "compozy__read",
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
				closeServerTestBody(t, resp.Body)
			}()
			if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
				body := readServerResponseBody(t, resp.Body)
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
		shutdownServerForTest(t, server)
	}()

	resp := doServerRequest(
		t,
		http.DefaultClient,
		http.MethodPost,
		mustURL("127.0.0.1", server.Port(), "/api/extensions"),
		mustJSONBody(t, contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath,
			Ref:    "/extensions/demo",
		}),
	)
	defer func() {
		closeServerTestBody(t, resp.Body)
	}()
	if got, want := resp.StatusCode, http.StatusConflict; got != want {
		body := readServerResponseBody(t, resp.Body)
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
		EnableFn: func(
			context.Context,
			string,
			contract.EnableExtensionRequest,
			taskpkg.ActorContext,
		) (contract.ExtensionEnableResult, error) {
			t.Fatal("Enable should not be called on non-loopback HTTP bind")
			return contract.ExtensionEnableResult{}, nil
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
		shutdownServerForTest(t, server)
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
				closeServerTestBody(t, resp.Body)
			}()
			if got, want := resp.StatusCode, http.StatusForbidden; got != want {
				body := readServerResponseBody(t, resp.Body)
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
			closeServerTestBody(t, resp.Body)
		}()
		if got, want := resp.StatusCode, http.StatusForbidden; got != want {
			body := readServerResponseBody(t, resp.Body)
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
			closeServerTestBody(t, resp.Body)
		}()
		if got, want := resp.StatusCode, http.StatusForbidden; got != want {
			body := readServerResponseBody(t, resp.Body)
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
			path:   "/api/tools/compozy__skill_view/approvals",
			body:   []byte(`{}`),
		},
		{
			name:   "Should block tool invocation",
			method: http.MethodPost,
			path:   "/api/tools/compozy__skill_view/invoke",
			body:   []byte(`{}`),
		},
		{
			name:   "Should block extension enablement writes",
			method: http.MethodPut,
			path:   "/api/extensions/demo/enablement",
			body:   []byte(`{"profile":"default","enabled":true}`),
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
				closeServerTestBody(t, resp.Body)
			}()
			if got, want := resp.StatusCode, http.StatusForbidden; got != want {
				body := readServerResponseBody(t, resp.Body)
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
		closeServerTestBody(t, resp.Body)
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
		shutdownServerForTest(t, first)
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

type terminalServeListener struct {
	net.Listener
	acceptStarted chan struct{}
	failure       chan error
	closed        chan struct{}
	acceptOnce    sync.Once
	closeOnce     sync.Once
	closeErr      error
}

func newTerminalServeListener(t *testing.T) *terminalServeListener {
	t.Helper()

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	wrapped := &terminalServeListener{
		Listener:      listener,
		acceptStarted: make(chan struct{}),
		failure:       make(chan error, 1),
		closed:        make(chan struct{}),
	}
	t.Cleanup(func() {
		if closeErr := wrapped.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("terminal listener Close() error = %v", closeErr)
		}
	})
	return wrapped
}

func (l *terminalServeListener) Accept() (net.Conn, error) {
	l.acceptOnce.Do(func() { close(l.acceptStarted) })
	select {
	case err := <-l.failure:
		return nil, err
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *terminalServeListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
		l.closeErr = l.Listener.Close()
	})
	return l.closeErr
}

func (l *terminalServeListener) fail(err error) {
	l.failure <- err
}

type delayedFirstCloseListener struct {
	firstCloseStarted chan struct{}
	release           chan struct{}
	releaseOnce       sync.Once
	closeCalls        atomic.Int64
}

func newDelayedFirstCloseListener() *delayedFirstCloseListener {
	return &delayedFirstCloseListener{
		firstCloseStarted: make(chan struct{}),
		release:           make(chan struct{}),
	}
}

func (l *delayedFirstCloseListener) Accept() (net.Conn, error) {
	return nil, net.ErrClosed
}

func (l *delayedFirstCloseListener) Close() error {
	if l.closeCalls.Add(1) == 1 {
		close(l.firstCloseStarted)
		<-l.release
	}
	return nil
}

func (l *delayedFirstCloseListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
}

func (l *delayedFirstCloseListener) releaseFirst() {
	l.releaseOnce.Do(func() { close(l.release) })
}

func newWindowManagerLifecycleServer(t *testing.T) *Server {
	t.Helper()

	manager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver("workspace-a"),
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("windowmanager.NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("window manager Close() error = %v", closeErr)
		}
	})

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{}),
		WithWorkspaceResolver(stubWorkspaceService{}),
		WithWindowManagerProvider(apitestutil.SingleProfileWindowManagers{Manager: manager}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return server
}

func assertWindowManagerGenerationFunctional(t *testing.T, server *Server) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
	defer cancel()
	streamURL := fmt.Sprintf(
		"ws://127.0.0.1:%d/api/workspaces/workspace-a/window-manager/stream",
		server.Port(),
	)
	connection, response, err := websocket.DefaultDialer.DialContext(ctx, streamURL, nil)
	if err != nil {
		dialErr := err
		var responseStatus string
		var responseBody []byte
		if response != nil {
			responseStatus = response.Status
			if response.Body != nil {
				var readErr error
				responseBody, readErr = io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr != nil {
					dialErr = errors.Join(dialErr, fmt.Errorf("read failed websocket response: %w", readErr))
				}
				if closeErr != nil {
					dialErr = errors.Join(dialErr, fmt.Errorf("close failed websocket response: %w", closeErr))
				}
			}
		}
		t.Fatalf("DialContext() error = %v, response = %q body = %q", dialErr, responseStatus, responseBody)
	}
	t.Cleanup(func() {
		if closeErr := connection.Close(); closeErr != nil {
			t.Errorf("window-manager websocket Close() error = %v", closeErr)
		}
	})
	if deadlineErr := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); deadlineErr != nil {
		t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
	}
	var frame contract.WindowManagerSnapshotFrame
	if readErr := connection.ReadJSON(&frame); readErr != nil {
		t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
	}
	if frame.Type != contract.WindowManagerFrameSnapshot || frame.WorkspaceID != "workspace-a" {
		t.Fatalf("snapshot frame = %+v", frame)
	}
}

func newLifecycleServer(t *testing.T, entered chan<- struct{}, release <-chan struct{}) *Server {
	t.Helper()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)
	engine := gin.New()
	engine.GET("/lifecycle-block", func(c *gin.Context) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		c.Status(http.StatusNoContent)
	})

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
		WithEngine(engine),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return server
}

func shutdownServerForTest(t *testing.T, server *Server) {
	t.Helper()

	if err := server.Shutdown(context.Background()); err != nil {
		t.Errorf("server.Shutdown() error = %v", err)
	}
}

func readServerResponseBody(t *testing.T, body io.Reader) []byte {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read server response body: %v", err)
	}
	return data
}

func closeServerTestBody(t *testing.T, body io.Closer) {
	t.Helper()

	if err := body.Close(); err != nil {
		t.Errorf("close server response body: %v", err)
	}
}

func requestLifecycleBlock(ctx context.Context, server *Server) <-chan error {
	result := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			mustURL("127.0.0.1", server.Port(), "/lifecycle-block"),
			http.NoBody,
		)
		if err != nil {
			result <- fmt.Errorf("new lifecycle request: %w", err)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			result <- fmt.Errorf("send lifecycle request: %w", err)
			return
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close lifecycle response: %w", closeErr))
		}
		if err == nil && resp.StatusCode != http.StatusNoContent {
			err = fmt.Errorf("lifecycle response status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
		result <- err
	}()
	return result
}

func registerServerShutdownSignal(t *testing.T, server *Server) <-chan struct{} {
	t.Helper()

	server.mu.Lock()
	generation := server.generation
	server.mu.Unlock()
	if generation == nil || generation.httpServer == nil {
		t.Fatal("HTTP server was not initialized")
	}
	started := make(chan struct{})
	var once sync.Once
	generation.httpServer.RegisterOnShutdown(func() {
		once.Do(func() {
			close(started)
		})
	})
	return started
}

func waitForServerSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
	defer cancel()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", name, ctx.Err())
	}
}

func waitForServerResult(t *testing.T, result <-chan error, name string) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
	defer cancel()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for %s: %w", name, ctx.Err())
	}
}
