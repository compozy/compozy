package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/pkg/compozy/events"
	webassets "github.com/compozy/compozy/web"
)

func TestHTTPAndUDSRegisterMatchingRoutes(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	handlers := core.NewHandlers(&core.HandlerConfig{TransportName: "test"})
	httpEngine := gin.New()
	udsEngine := gin.New()

	httpapi.RegisterRoutes(httpEngine, handlers)
	udsapi.RegisterRoutes(udsEngine, handlers)

	httpRoutes := routeKeys(httpEngine.Routes())
	udsRoutes := routeKeys(udsEngine.Routes())
	if diff := diffRoutes(httpRoutes, udsRoutes); diff != "" {
		t.Fatalf("route parity mismatch:\n%s", diff)
	}
}

func TestSharedRouteRegistrationIncludesBrowserEndpoints(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	handlers := core.NewHandlers(&core.HandlerConfig{TransportName: "test"})
	httpEngine := gin.New()
	httpapi.RegisterRoutes(httpEngine, handlers)

	routes := routeKeys(httpEngine.Routes())
	required := []string{
		"GET /api/ui/dashboard",
		"GET /api/tasks",
		"GET /api/tasks/:slug",
		"GET /api/tasks/:slug/spec",
		"GET /api/tasks/:slug/memory",
		"GET /api/tasks/:slug/memory/files/:file_id",
		"GET /api/tasks/:slug/board",
		"GET /api/tasks/:slug/items/:task_id",
		"GET /api/reviews/:slug/rounds/:round/issues/:issue_id",
		"POST /api/tasks/:slug/runs",
		"POST /api/tasks/:slug/archive",
		"POST /api/reviews/:slug/rounds/:round/runs",
		"POST /api/sync",
		"GET /api/workspaces",
	}

	routeSet := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		routeSet[route] = struct{}{}
	}
	for _, route := range required {
		if _, ok := routeSet[route]; !ok {
			t.Fatalf("required route %q missing from registration", route)
		}
	}
}

func TestHTTPServesEmbeddedSPAAndPreservesAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	daemonSvc := &fakeDaemonService{
		status: core.DaemonStatus{
			PID:       42,
			Version:   "transport-test",
			StartedAt: time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
		},
		health: core.DaemonHealth{Ready: true},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName: "http",
		Daemon:        daemonSvc,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	statusCode, _, rootBody := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/", nil, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", statusCode, http.StatusOK)
	}
	assertSPAShell(t, rootBody)

	assetPath, expectedAsset := firstEmbeddedBundleAsset(t)
	statusCode, _, assetBody := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+assetPath, nil, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", assetPath, statusCode, http.StatusOK)
	}
	if !bytes.Equal(assetBody, expectedAsset) {
		t.Fatalf("GET %s body mismatch", assetPath)
	}

	statusCode, _, apiBody := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/api/daemon/status", nil, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("GET /api/daemon/status status = %d, want %d", statusCode, http.StatusOK)
	}
	if !bytes.Contains(apiBody, []byte(`"pid":42`)) {
		t.Fatalf("daemon status body = %s, want pid field", apiBody)
	}
}

func TestHTTPFallbackServesDeepLinksWithoutShadowingAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName: "http",
		Daemon: &fakeDaemonService{
			status: core.DaemonStatus{PID: 42},
			health: core.DaemonHealth{Ready: true},
		},
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	statusCode, _, deepLinkBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/workflows/daemon/tasks/task_08",
		nil,
		nil,
	)
	if statusCode != http.StatusOK {
		t.Fatalf("GET deep link status = %d, want %d", statusCode, http.StatusOK)
	}
	assertSPAShell(t, deepLinkBody)

	statusCode, _, apiBody := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/api/missing", nil, nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("GET /api/missing status = %d, want %d", statusCode, http.StatusNotFound)
	}
	assertNotSPAHTML(t, apiBody)

	statusCode, _, assetBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/assets/does-not-exist.js",
		nil,
		nil,
	)
	if statusCode != http.StatusNotFound {
		t.Fatalf("GET missing asset status = %d, want %d", statusCode, http.StatusNotFound)
	}
	assertNotSPAHTML(t, assetBody)
}

func TestUDSRemainsAPIOnlyWhileHTTPServesSPA(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "transport",
		Daemon: &fakeDaemonService{
			status: core.DaemonStatus{PID: 42},
			health: core.DaemonHealth{Ready: true},
		},
	})

	httpServer, baseURL := startHTTPServer(t, handlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	socketPath := newShortSocketPath(t)
	udsServer, udsClient := startUDSServer(t, handlers, socketPath)
	defer func() {
		_ = udsServer.Shutdown(context.Background())
	}()

	statusCode, _, httpRootBody := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/", nil, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("HTTP GET / status = %d, want %d", statusCode, http.StatusOK)
	}
	assertSPAShell(t, httpRootBody)

	statusCode, _, udsRootBody := mustRequest(t, udsClient, http.MethodGet, "http://unix/", nil, nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("UDS GET / status = %d, want %d", statusCode, http.StatusNotFound)
	}
	assertNotSPAHTML(t, udsRootBody)

	statusCode, _, udsAPIBody := mustRequest(t, udsClient, http.MethodGet, "http://unix/api/daemon/status", nil, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("UDS GET /api/daemon/status status = %d, want %d", statusCode, http.StatusOK)
	}
	if !bytes.Contains(udsAPIBody, []byte(`"pid":42`)) {
		t.Fatalf("UDS daemon status body = %s, want pid field", udsAPIBody)
	}
}

func TestHTTPServerPersistsActualPortInDaemonInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	homeRoot := filepath.Join(t.TempDir(), ".compozy")
	paths, err := compozyconfig.ResolveHomePathsFrom(homeRoot)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	startResult, err := daemon.Start(context.Background(), daemon.StartOptions{
		HomePaths: paths,
		Version:   "transport-test",
	})
	if err != nil {
		t.Fatalf("daemon.Start() error = %v", err)
	}
	defer func() {
		_ = startResult.Host.Close(context.Background())
	}()

	handlers := core.NewHandlers(&core.HandlerConfig{TransportName: "http"})
	server, err := httpapi.New(
		httpapi.WithHandlers(handlers),
		httpapi.WithPortUpdater(startResult.Host),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}

	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	info, err := daemon.ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if server.Port() == 0 {
		t.Fatal("server.Port() = 0, want non-zero")
	}
	if info.HTTPPort != server.Port() {
		t.Fatalf("info.HTTPPort = %d, want %d", info.HTTPPort, server.Port())
	}
}

func TestHTTPServerRejectsConcurrentStartBeforePortPersistenceReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	updater := &blockingPortUpdater{
		entered: make(chan int, 2),
		release: make(chan struct{}),
	}
	server, err := httpapi.New(
		httpapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "http"})),
		httpapi.WithPort(0),
		httpapi.WithPortUpdater(updater),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- server.Start(context.Background())
	}()

	select {
	case <-updater.entered:
	case <-time.After(time.Second):
		t.Fatal("first start did not reach port persistence")
	}

	secondErrCh := make(chan error, 1)
	go func() {
		secondErrCh <- server.Start(context.Background())
	}()

	select {
	case err := <-secondErrCh:
		if err == nil || !strings.Contains(err.Error(), "already started") {
			t.Fatalf("second Start() error = %v, want already started", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second Start() did not return while first start was in progress")
	}

	close(updater.release)

	select {
	case err := <-firstErrCh:
		if err != nil {
			t.Fatalf("first Start() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first Start() did not finish after port persistence was released")
	}

	defer func() {
		_ = server.Shutdown(context.Background())
	}()
}

func TestHTTPServerNewHonorsInjectedLoggerAndEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server, err := httpapi.New(
		httpapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "http"})),
		httpapi.WithLogger(logger),
		httpapi.WithEngine(engine),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
}

func TestHTTPServerRejectsNonLoopbackHost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := httpapi.New(
		httpapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "http"})),
		httpapi.WithHost("0.0.0.0"),
	)
	if err == nil || !strings.Contains(err.Error(), "host must be 127.0.0.1") {
		t.Fatalf("httpapi.New() error = %v, want loopback host validation", err)
	}
}

func TestHTTPServerStartRejectsCancelledContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server, err := httpapi.New(
		httpapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "http"})),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := server.Start(cancelledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("server.Start() error = %v, want context.Canceled", err)
	}
}

func TestHTTPBrowserWorkspaceHeaderSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskSvc := &capturingTaskService{}
	syncSvc := &fakeSyncService{}
	validRoot := t.TempDir()
	missingRoot := filepath.Join(t.TempDir(), "missing")
	workspaces := &fakeWorkspaceService{
		items: map[string]core.Workspace{
			"ws-header":  {ID: "ws-header", RootDir: validRoot, Name: "Header"},
			"ws-missing": {ID: "ws-missing", RootDir: missingRoot, Name: "Missing"},
		},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		Tasks:      taskSvc,
		Sync:       syncSvc,
		Workspaces: workspaces,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	statusCode, _, _ := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/ui/dashboard",
		nil,
		map[string]string{core.HeaderActiveWorkspaceID: "ws-header"},
	)
	if statusCode != http.StatusOK {
		t.Fatalf("header dashboard status = %d, want 200", statusCode)
	}
	if got := taskSvc.dashboardWorkspace(); got != "ws-header" {
		t.Fatalf("dashboard workspace = %q, want ws-header", got)
	}

	statusCode, _, _ = mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/ui/dashboard?workspace=ws-legacy",
		nil,
		nil,
	)
	if statusCode != http.StatusPreconditionFailed {
		t.Fatalf("legacy dashboard status = %d, want 412", statusCode)
	}

	statusCode, _, body := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/ui/dashboard",
		nil,
		nil,
	)
	if statusCode != http.StatusPreconditionFailed {
		t.Fatalf("missing workspace status = %d, want 412", statusCode)
	}
	assertTransportCode(t, body, "workspace_context_missing")

	statusCode, _, body = mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/ui/dashboard",
		nil,
		map[string]string{core.HeaderActiveWorkspaceID: "ws-stale"},
	)
	if statusCode != http.StatusPreconditionFailed {
		t.Fatalf("stale workspace status = %d, want 412", statusCode)
	}
	assertTransportCode(t, body, "workspace_context_stale")

	statusCode, _, _ = mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/ui/dashboard",
		nil,
		map[string]string{core.HeaderActiveWorkspaceID: "ws-missing"},
	)
	if statusCode != http.StatusOK {
		t.Fatalf("missing root workspace status = %d, want 200", statusCode)
	}
	if got := taskSvc.dashboardWorkspace(); got != "ws-missing" {
		t.Fatalf("missing root dashboard workspace = %q, want ws-missing", got)
	}

	statusCode, _, body = mustRequest(
		t,
		http.DefaultClient,
		http.MethodPost,
		baseURL+"/api/sync",
		[]byte(`{"workflow_slug":"wf-1"}`),
		map[string]string{
			"Content-Type":               "application/json",
			core.HeaderActiveWorkspaceID: "ws-missing",
		},
	)
	if statusCode != http.StatusPreconditionFailed {
		t.Fatalf("missing root sync status = %d, want 412", statusCode)
	}
	assertTransportCode(t, body, "workspace_path_missing")

	statusCode, _, _ = mustRequest(
		t,
		http.DefaultClient,
		http.MethodPost,
		baseURL+"/api/sync",
		[]byte(`{"workflow_slug":"wf-1"}`),
		map[string]string{
			"Content-Type":               "application/json",
			core.HeaderActiveWorkspaceID: "ws-header",
		},
	)
	if statusCode != http.StatusOK {
		t.Fatalf("header sync status = %d, want 200", statusCode)
	}
	if got := syncSvc.workspace(); got != "ws-header" {
		t.Fatalf("sync workspace = %q, want ws-header", got)
	}

	statusCode, _, body = mustRequest(
		t,
		http.DefaultClient,
		http.MethodPost,
		baseURL+"/api/sync",
		[]byte(`{"workflow_slug":"wf-1"}`),
		map[string]string{"Content-Type": "application/json"},
	)
	if statusCode != http.StatusPreconditionFailed {
		t.Fatalf("missing sync target status = %d, want 412", statusCode)
	}
	assertTransportCode(t, body, "workspace_context_missing")
}

func TestHTTPBrowserSecurityAndUDSCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	daemonSvc := &fakeDaemonService{
		status: core.DaemonStatus{
			PID:       7,
			StartedAt: time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
		},
		health: core.DaemonHealth{Ready: true},
	}
	runSvc := &fakeRunService{
		runs: map[string]core.Run{
			"run-1": {RunID: "run-1", Status: "running"},
		},
	}
	handlers := core.NewHandlers(&core.HandlerConfig{
		Daemon: daemonSvc,
		Runs:   runSvc,
	})

	httpServer, baseURL := startHTTPServer(t, handlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	evilHostRequest, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		baseURL+"/api/daemon/status",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest(invalid host) error = %v", err)
	}
	evilHostRequest.Host = "evil.example:" + strconv.Itoa(httpServer.Port())
	evilHostResponse, err := http.DefaultClient.Do(evilHostRequest)
	if err != nil {
		t.Fatalf("Do(invalid host) error = %v", err)
	}
	defer evilHostResponse.Body.Close()
	evilHostBody, err := io.ReadAll(evilHostResponse.Body)
	if err != nil {
		t.Fatalf("ReadAll(invalid host) error = %v", err)
	}
	if evilHostResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("invalid host status = %d, want 403", evilHostResponse.StatusCode)
	}
	assertTransportCode(t, evilHostBody, "host_invalid")

	statusCode, _, originBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/daemon/status",
		nil,
		map[string]string{"Origin": "http://evil.example:" + strconv.Itoa(httpServer.Port())},
	)
	if statusCode != http.StatusForbidden {
		t.Fatalf("invalid origin status = %d, want 403", statusCode)
	}
	assertTransportCode(t, originBody, "origin_invalid")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	browserClient := &http.Client{Jar: jar}

	statusCode, headers, _ := mustRequest(
		t,
		browserClient,
		http.MethodGet,
		baseURL+"/api/daemon/status",
		nil,
		nil,
	)
	if statusCode != http.StatusOK {
		t.Fatalf("csrf bootstrap status = %d, want 200", statusCode)
	}
	csrfToken := strings.TrimSpace(headers.Get(core.HeaderCSRF))
	if csrfToken == "" {
		t.Fatal("csrf bootstrap header = empty, want token")
	}

	statusCode, _, csrfMissingBody := mustRequest(
		t,
		browserClient,
		http.MethodPost,
		baseURL+"/api/runs/run-1/cancel",
		nil,
		map[string]string{"Origin": baseURL},
	)
	if statusCode != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d, want 403", statusCode)
	}
	assertTransportCode(t, csrfMissingBody, "csrf_missing")

	statusCode, _, _ = mustRequest(
		t,
		browserClient,
		http.MethodPost,
		baseURL+"/api/runs/run-1/cancel",
		nil,
		map[string]string{
			"Origin":        baseURL,
			core.HeaderCSRF: csrfToken,
		},
	)
	if statusCode != http.StatusAccepted {
		t.Fatalf("valid csrf status = %d, want 202", statusCode)
	}

	socketPath := newShortSocketPath(t)
	udsServer, udsClient := startUDSServer(t, handlers, socketPath)
	defer func() {
		_ = udsServer.Shutdown(context.Background())
	}()

	statusCode, _, _ = mustRequest(
		t,
		udsClient,
		http.MethodPost,
		"http://unix/api/runs/run-1/cancel",
		nil,
		nil,
	)
	if statusCode != http.StatusAccepted {
		t.Fatalf("uds cancel status = %d, want 202", statusCode)
	}
}
func TestHTTPServerStartRollsBackAfterPortPersistenceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	updater := &flakyPortUpdater{failuresRemaining: 1}
	server, err := httpapi.New(
		httpapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "http"})),
		httpapi.WithPort(0),
		httpapi.WithPortUpdater(updater),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}

	err = server.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "persist http port") {
		t.Fatalf("first Start() error = %v, want persist http port failure", err)
	}

	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
}

func TestUDSServerCreates0600Socket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	socketPath := newShortSocketPath(t)
	server, err := udsapi.New(
		udsapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "uds"})),
		udsapi.WithSocketPath(socketPath),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", socketPath, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("socket perm = %o, want 600", info.Mode().Perm())
	}
}

func TestUDSServerNewPrefersExplicitSocketPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	socketPath := filepath.Join(t.TempDir(), "explicit.sock")
	server, err := udsapi.New(
		udsapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "uds"})),
		udsapi.WithSocketPath(socketPath),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	if server.Path() != socketPath {
		t.Fatalf("server.Path() = %q, want %q", server.Path(), socketPath)
	}
}

func TestUDSServerStartRollsBackAfterParentSetupFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	blockedParentFile, err := os.CreateTemp("/tmp", "compozy-uds-blocked-*")
	if err != nil {
		t.Fatalf("CreateTemp(/tmp) error = %v", err)
	}
	blockedParent := blockedParentFile.Name()
	if err := blockedParentFile.Close(); err != nil {
		t.Fatalf("Close(%s) error = %v", blockedParent, err)
	}

	socketPath := filepath.Join(blockedParent, "daemon.sock")
	server, err := udsapi.New(
		udsapi.WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "uds"})),
		udsapi.WithSocketPath(socketPath),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}

	err = server.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create socket directory") {
		t.Fatalf("first Start() error = %v, want socket directory failure", err)
	}

	if err := os.Remove(blockedParent); err != nil {
		t.Fatalf("Remove(%s) error = %v", blockedParent, err)
	}
	if err := os.MkdirAll(blockedParent, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", blockedParent, err)
	}

	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
}

func TestHealthTransitionsOverHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	daemonSvc := &fakeDaemonService{
		health: core.DaemonHealth{
			Ready: false,
			Details: []core.HealthDetail{{
				Code:    "starting",
				Message: "daemon still starting",
			}},
		},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName: "http",
		Daemon:        daemonSvc,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	status, _, body := mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/api/daemon/health", nil, nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("initial status = %d, want %d", status, http.StatusServiceUnavailable)
	}

	var initial map[string]any
	decodeJSON(t, body, &initial)
	if health, _ := initial["health"].(map[string]any); health["ready"] != false {
		t.Fatalf("initial health payload = %#v, want ready=false", initial)
	}

	daemonSvc.setHealth(core.DaemonHealth{Ready: true})

	status, _, body = mustRequest(t, http.DefaultClient, http.MethodGet, baseURL+"/api/daemon/health", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", status, http.StatusOK)
	}

	var ready map[string]any
	decodeJSON(t, body, &ready)
	if health, _ := ready["health"].(map[string]any); health["ready"] != true {
		t.Fatalf("ready health payload = %#v, want ready=true", ready)
	}
}

func TestHTTPAndUDSServeMatchingStatusSnapshotAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runID := "run-1"
	workspaceID := "ws-1"
	startedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	nextCursor := core.StreamCursor{Timestamp: startedAt.Add(time.Second), Sequence: 2}

	daemonSvc := &fakeDaemonService{
		status: core.DaemonStatus{
			PID:            1234,
			Version:        "test",
			StartedAt:      startedAt,
			ActiveRunCount: 1,
			WorkspaceCount: 2,
		},
		health: core.DaemonHealth{
			Ready:               true,
			Degraded:            true,
			UptimeSeconds:       30,
			ActiveRunCount:      1,
			ActiveRunsByMode:    []core.DaemonModeCount{{Mode: "task", Count: 1}},
			WorkspaceCount:      2,
			IntegrityIssueCount: 1,
			Databases: core.DaemonDatabaseDiagnostics{
				GlobalBytes: 64,
				RunDBBytes:  128,
			},
			Reconcile: core.DaemonReconcileDiagnostics{
				ReconciledRuns:     2,
				CrashEventAppended: 1,
				CrashEventMissing:  1,
				LastRunID:          runID,
			},
			Details: []core.HealthDetail{{
				Code:     "run_integrity_issues",
				Message:  "1 run(s) have persisted integrity issues",
				Severity: "warning",
			}},
		},
		metrics: core.MetricsPayload{Body: strings.Join([]string{
			"# HELP daemon_active_runs Current live runs owned by the daemon",
			"# TYPE daemon_active_runs gauge",
			"daemon_active_runs 1",
			`daemon_run_terminal_total{mode="task",status="completed"} 1`,
		}, "\n") + "\n"},
	}
	runSvc := &fakeRunService{
		runs: map[string]core.Run{
			runID: {
				RunID:            runID,
				WorkspaceID:      workspaceID,
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        startedAt,
			},
		},
		snapshots: map[string]core.RunSnapshot{
			runID: {
				Run: core.Run{
					RunID:            runID,
					WorkspaceID:      workspaceID,
					Mode:             "task",
					Status:           "running",
					PresentationMode: "stream",
					StartedAt:        startedAt,
				},
				Jobs: []core.RunJobState{{
					JobID:     "job-1",
					Status:    "running",
					UpdatedAt: startedAt.Add(time.Second),
				}},
				Transcript: []core.RunTranscriptMessage{{
					Sequence:  1,
					Stream:    "session",
					Role:      "assistant",
					Content:   "hello",
					Timestamp: startedAt.Add(time.Second),
				}},
				Incomplete:        true,
				IncompleteReasons: []string{"transcript_gap"},
				NextCursor:        &nextCursor,
			},
		},
		eventPages: map[string]core.RunEventPage{
			runID: {
				Events: []events.Event{
					newRunEvent(
						runID,
						1,
						events.EventKindRunStarted,
						startedAt,
						`{"status":"started"}`,
					),
					newRunEvent(
						runID,
						2,
						events.EventKindSessionUpdate,
						startedAt.Add(time.Second),
						`{"delta":"hello"}`,
					),
				},
				NextCursor: &nextCursor,
				HasMore:    true,
			},
		},
	}
	workspaceSvc := &fakeWorkspaceService{
		deleteErr: core.NewProblem(
			http.StatusConflict,
			"active_runs",
			"workspace has active runs",
			map[string]any{"workspace_id": workspaceID},
			nil,
		),
	}

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "shared",
		Daemon:        daemonSvc,
		Workspaces:    workspaceSvc,
		Runs:          runSvc,
	})

	httpServer, baseURL := startHTTPServer(t, handlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	socketPath := newShortSocketPath(t)
	udsServer, udsClient := startUDSServer(t, handlers, socketPath)
	defer func() {
		_ = udsServer.Shutdown(context.Background())
	}()

	daemonSvc.setStatus(func(status core.DaemonStatus) core.DaemonStatus {
		status.HTTPPort = httpServer.Port()
		status.SocketPath = udsServer.Path()
		return status
	})

	httpStatusCode, _, httpStatusBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/daemon/status",
		nil,
		nil,
	)
	udsStatusCode, _, udsStatusBody := mustRequest(
		t,
		udsClient,
		http.MethodGet,
		"http://unix/api/daemon/status",
		nil,
		nil,
	)
	if httpStatusCode != http.StatusOK || udsStatusCode != http.StatusOK {
		t.Fatalf("status codes = (%d, %d), want (200, 200)", httpStatusCode, udsStatusCode)
	}
	assertJSONEqual(t, httpStatusBody, udsStatusBody)

	httpSnapshotCode, _, httpSnapshotBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/runs/"+runID+"/snapshot",
		nil,
		nil,
	)
	udsSnapshotCode, _, udsSnapshotBody := mustRequest(
		t,
		udsClient,
		http.MethodGet,
		"http://unix/api/runs/"+runID+"/snapshot",
		nil,
		nil,
	)
	if httpSnapshotCode != http.StatusOK || udsSnapshotCode != http.StatusOK {
		t.Fatalf("snapshot codes = (%d, %d), want (200, 200)", httpSnapshotCode, udsSnapshotCode)
	}
	assertJSONEqual(t, httpSnapshotBody, udsSnapshotBody)

	httpEventsCode, _, httpEventsBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/runs/"+runID+"/events?limit=2",
		nil,
		nil,
	)
	udsEventsCode, _, udsEventsBody := mustRequest(
		t,
		udsClient,
		http.MethodGet,
		"http://unix/api/runs/"+runID+"/events?limit=2",
		nil,
		nil,
	)
	if httpEventsCode != http.StatusOK || udsEventsCode != http.StatusOK {
		t.Fatalf("events codes = (%d, %d), want (200, 200)", httpEventsCode, udsEventsCode)
	}
	assertJSONEqual(t, httpEventsBody, udsEventsBody)

	httpConflictCode, _, httpConflictBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodDelete,
		baseURL+"/api/workspaces/"+workspaceID,
		nil,
		nil,
	)
	udsConflictCode, _, udsConflictBody := mustRequest(
		t,
		udsClient,
		http.MethodDelete,
		"http://unix/api/workspaces/"+workspaceID,
		nil,
		nil,
	)
	if httpConflictCode != http.StatusConflict || udsConflictCode != http.StatusConflict {
		t.Fatalf("conflict codes = (%d, %d), want (409, 409)", httpConflictCode, udsConflictCode)
	}
	assertJSONEqualIgnoringRequestID(t, httpConflictBody, udsConflictBody)
}

func TestHTTPAndUDSServeCanonicalParityAcrossRouteGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 20, 19, 0, 0, 0, time.UTC)
	workspaceRoot := t.TempDir()
	workspace := core.Workspace{
		ID:        "ws-1",
		RootDir:   workspaceRoot,
		Name:      "workspace",
		CreatedAt: now,
		UpdatedAt: now,
	}
	taskRun := core.Run{
		RunID:            "task-run-1",
		WorkspaceID:      workspace.ID,
		WorkflowSlug:     "daemon",
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-task",
	}
	reviewRun := core.Run{
		RunID:            "review-run-1",
		WorkspaceID:      workspace.ID,
		WorkflowSlug:     "daemon",
		Mode:             "review",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-review",
	}
	execRun := core.Run{
		RunID:            "exec-run-1",
		WorkspaceID:      workspace.ID,
		Mode:             "exec",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-exec",
	}
	nextCursor := core.StreamCursor{Timestamp: now.Add(time.Second), Sequence: 2}

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "shared",
		Daemon: &fakeDaemonService{
			health: core.DaemonHealth{
				Ready: true,
				Details: []core.HealthDetail{{
					Code:    "healthy",
					Message: "daemon is ready",
				}},
			},
		},
		Workspaces: &fakeWorkspaceService{
			workspaces: []core.Workspace{workspace},
			workspace:  workspace,
		},
		Tasks: &fakeTaskService{
			run: taskRun,
		},
		Reviews: &fakeReviewService{
			run: reviewRun,
		},
		Runs: &fakeRunService{
			runs: map[string]core.Run{
				taskRun.RunID: taskRun,
			},
			snapshots: map[string]core.RunSnapshot{
				taskRun.RunID: {
					Run: taskRun,
					Jobs: []core.RunJobState{{
						JobID:     "job-1",
						Status:    "running",
						UpdatedAt: now.Add(time.Second),
					}},
					Transcript: []core.RunTranscriptMessage{{
						Sequence:  1,
						Stream:    "session",
						Role:      "assistant",
						Content:   "hello",
						Timestamp: now.Add(time.Second),
					}},
					Incomplete:        true,
					IncompleteReasons: []string{"transcript_gap"},
					NextCursor:        &nextCursor,
				},
			},
		},
		Sync: &fakeSyncService{
			result: core.SyncResult{
				WorkspaceID:  workspace.ID,
				WorkflowSlug: "daemon",
				SyncedAt:     ptrTimeHTTP(now),
				SyncedPaths:  []string{workspace.RootDir},
			},
		},
		Exec: &fakeExecService{
			run: execRun,
		},
	})

	httpServer, baseURL := startHTTPServer(t, handlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	socketPath := newShortSocketPath(t)
	udsServer, udsClient := startUDSServer(t, handlers, socketPath)
	defer func() {
		_ = udsServer.Shutdown(context.Background())
	}()

	testCases := []struct {
		name       string
		method     string
		path       string
		body       []byte
		headers    map[string]string
		requestID  string
		wantStatus int
		assertBody func(*testing.T, []byte, []byte)
	}{
		{
			name:       "daemon health",
			method:     http.MethodGet,
			path:       "/api/daemon/health",
			requestID:  "req-daemon-health",
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.DaemonHealthResponse
				var udsPayload contract.DaemonHealthResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("health payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
		{
			name:       "daemon metrics",
			method:     http.MethodGet,
			path:       "/api/daemon/metrics",
			requestID:  "req-daemon-metrics",
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				if !bytes.Equal(httpBody, udsBody) {
					t.Fatalf("metrics payload mismatch\nhttp: %s\nuds:  %s", httpBody, udsBody)
				}
			},
		},
		{
			name:       "workspace list",
			method:     http.MethodGet,
			path:       "/api/workspaces",
			requestID:  "req-workspaces",
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.WorkspaceListResponse
				var udsPayload contract.WorkspaceListResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("workspace payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
		{
			name:       "task run",
			method:     http.MethodPost,
			path:       "/api/tasks/daemon/runs",
			body:       []byte(`{"workspace":"ws-1","presentation_mode":"stream"}`),
			headers:    map[string]string{core.HeaderActiveWorkspaceID: workspace.ID},
			requestID:  "req-task-run",
			wantStatus: http.StatusCreated,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.RunResponse
				var udsPayload contract.RunResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("task run payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
		{
			name:       "review run",
			method:     http.MethodPost,
			path:       "/api/reviews/daemon/rounds/1/runs",
			body:       []byte(`{"workspace":"ws-1","presentation_mode":"stream"}`),
			headers:    map[string]string{core.HeaderActiveWorkspaceID: workspace.ID},
			requestID:  "req-review-run",
			wantStatus: http.StatusCreated,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.RunResponse
				var udsPayload contract.RunResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("review run payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
		{
			name:       "run snapshot",
			method:     http.MethodGet,
			path:       "/api/runs/" + taskRun.RunID + "/snapshot",
			requestID:  "req-run-snapshot",
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.RunSnapshotResponse
				var udsPayload contract.RunSnapshotResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("snapshot payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
				if got, want := httpPayload.IncompleteReasons, []string{
					"transcript_gap",
				}; !reflect.DeepEqual(
					got,
					want,
				) {
					t.Fatalf("snapshot incomplete reasons = %#v, want %#v", got, want)
				}
			},
		},
		{
			name:       "sync",
			method:     http.MethodPost,
			path:       "/api/sync",
			body:       []byte(`{"workspace":"ws-1","workflow_slug":"daemon"}`),
			headers:    map[string]string{core.HeaderActiveWorkspaceID: workspace.ID},
			requestID:  "req-sync",
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.SyncResponse
				var udsPayload contract.SyncResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("sync payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
		{
			name:       "exec",
			method:     http.MethodPost,
			path:       "/api/exec",
			body:       []byte(`{"workspace_path":"/tmp/workspace","prompt":"run","presentation_mode":"stream"}`),
			requestID:  "req-exec",
			wantStatus: http.StatusCreated,
			assertBody: func(t *testing.T, httpBody []byte, udsBody []byte) {
				t.Helper()

				var httpPayload contract.RunResponse
				var udsPayload contract.RunResponse
				decodeJSON(t, httpBody, &httpPayload)
				decodeJSON(t, udsBody, &udsPayload)
				if !reflect.DeepEqual(httpPayload, udsPayload) {
					t.Fatalf("exec payload mismatch\nhttp: %#v\nuds:  %#v", httpPayload, udsPayload)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			httpHeadersMap := map[string]string{core.HeaderRequestID: tc.requestID}
			udsHeadersMap := map[string]string{core.HeaderRequestID: tc.requestID}
			for key, value := range tc.headers {
				httpHeadersMap[key] = value
				udsHeadersMap[key] = value
			}

			httpStatus, httpHeaders, httpBody := mustRequest(
				t,
				http.DefaultClient,
				tc.method,
				baseURL+tc.path,
				tc.body,
				httpHeadersMap,
			)
			udsStatus, udsHeaders, udsBody := mustRequest(
				t,
				udsClient,
				tc.method,
				"http://unix"+tc.path,
				tc.body,
				udsHeadersMap,
			)

			if httpStatus != tc.wantStatus || udsStatus != tc.wantStatus {
				t.Fatalf(
					"status codes = (%d, %d), want (%d, %d)",
					httpStatus,
					udsStatus,
					tc.wantStatus,
					tc.wantStatus,
				)
			}
			if got := strings.TrimSpace(httpHeaders.Get(core.HeaderRequestID)); got != tc.requestID {
				t.Fatalf("http X-Request-Id = %q, want %q", got, tc.requestID)
			}
			if got := strings.TrimSpace(udsHeaders.Get(core.HeaderRequestID)); got != tc.requestID {
				t.Fatalf("uds X-Request-Id = %q, want %q", got, tc.requestID)
			}

			tc.assertBody(t, httpBody, udsBody)
		})
	}
}

func TestHTTPAndUDSEmitEquivalentCanonicalSSEStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 20, 19, 30, 0, 0, time.UTC)
	runID := "run-1"
	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName:     "shared",
		HeartbeatInterval: 15 * time.Millisecond,
		Now: func() time.Time {
			return now.Add(2 * time.Second)
		},
		Runs: &fakeRunService{
			openStreamFn: func(ctx context.Context, gotRunID string, after core.StreamCursor) (core.RunStream, error) {
				if gotRunID != runID {
					return nil, globaldb.ErrRunNotFound
				}
				stream := newChannelRunStream()
				go func() {
					defer close(stream.events)
					defer close(stream.errors)

					event := newRunEvent(runID, 1, events.EventKindSessionUpdate, now, `{"delta":"hello"}`)
					if core.EventAfterCursor(event, after) {
						item := event
						stream.events <- core.RunStreamItem{Event: &item}
					}

					timer := time.NewTimer(35 * time.Millisecond)
					defer timer.Stop()
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
					}

					stream.events <- core.RunStreamItem{
						Overflow: &core.RunStreamOverflow{Reason: "subscriber_dropped_messages"},
					}
				}()
				return stream, nil
			},
		},
	})

	httpServer, baseURL := startHTTPServer(t, handlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	socketPath := newShortSocketPath(t)
	udsServer, udsClient := startUDSServer(t, handlers, socketPath)
	defer func() {
		_ = udsServer.Shutdown(context.Background())
	}()

	httpResponse := mustStreamRequest(
		t,
		http.DefaultClient,
		baseURL+"/api/runs/"+runID+"/stream",
		"req-http-stream",
	)
	defer httpResponse.Body.Close()

	udsResponse := mustStreamRequest(
		t,
		udsClient,
		"http://unix/api/runs/"+runID+"/stream",
		"req-uds-stream",
	)
	defer udsResponse.Body.Close()

	httpFrames, err := testutil.ReadSSEFramesUntil(
		httpResponse.Body,
		2*time.Second,
		func(frames []testutil.SSEFrame) bool {
			return hasCanonicalSSEFrame(frames, core.RunOverflowSSEEvent)
		},
	)
	if err != nil {
		t.Fatalf("ReadSSEFramesUntil(http) error = %v", err)
	}
	udsFrames, err := testutil.ReadSSEFramesUntil(
		udsResponse.Body,
		2*time.Second,
		func(frames []testutil.SSEFrame) bool {
			return hasCanonicalSSEFrame(frames, core.RunOverflowSSEEvent)
		},
	)
	if err != nil {
		t.Fatalf("ReadSSEFramesUntil(uds) error = %v", err)
	}
	httpFrames = normalizeCanonicalSSEFrames(httpFrames)
	udsFrames = normalizeCanonicalSSEFrames(udsFrames)

	if len(httpFrames) != 3 || len(udsFrames) != 3 {
		t.Fatalf("unexpected normalized frames\nhttp: %#v\nuds:  %#v", httpFrames, udsFrames)
	}
	if !reflect.DeepEqual(httpFrames, udsFrames) {
		t.Fatalf("stream frame mismatch\nhttp: %#v\nuds:  %#v", httpFrames, udsFrames)
	}

	var eventPayload events.Event
	if err := json.Unmarshal(httpFrames[0].Data, &eventPayload); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	if eventPayload.Kind != events.EventKindSessionUpdate || eventPayload.RunID != runID {
		t.Fatalf("event payload = %#v", eventPayload)
	}

	var heartbeatPayload contract.HeartbeatPayload
	if err := json.Unmarshal(httpFrames[1].Data, &heartbeatPayload); err != nil {
		t.Fatalf("json.Unmarshal(heartbeat) error = %v", err)
	}
	if heartbeatPayload.RunID != runID || heartbeatPayload.Cursor != core.FormatCursor(now, 1) {
		t.Fatalf("heartbeat payload = %#v", heartbeatPayload)
	}

	var overflowPayload contract.OverflowPayload
	if err := json.Unmarshal(httpFrames[2].Data, &overflowPayload); err != nil {
		t.Fatalf("json.Unmarshal(overflow) error = %v", err)
	}
	if overflowPayload.RunID != runID ||
		overflowPayload.Cursor != core.FormatCursor(now, 1) ||
		overflowPayload.Reason != "subscriber_dropped_messages" {
		t.Fatalf("overflow payload = %#v", overflowPayload)
	}
}

func TestUDSShutdownDoesNotCancelHTTPStreamsWhenHandlersAreShared(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sharedHandlers := core.NewHandlers(&core.HandlerConfig{
		TransportName:     "shared",
		HeartbeatInterval: 150 * time.Millisecond,
		Runs: &fakeRunService{
			openStreamFn: func(_ context.Context, _ string, _ core.StreamCursor) (core.RunStream, error) {
				return newChannelRunStream(), nil
			},
		},
	})

	httpServer, baseURL := startHTTPServer(t, sharedHandlers)
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	socketPath := newShortSocketPath(t)
	udsServer, _ := startUDSServer(t, sharedHandlers, socketPath)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		baseURL+"/api/runs/run-1/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()

	linesCh, errCh := startSSEScan(response.Body)
	waitForSSELine(t, linesCh, errCh, "event: "+core.RunHeartbeatSSEEvent, time.Second)

	if err := udsServer.Shutdown(context.Background()); err != nil {
		t.Fatalf("udsServer.Shutdown() error = %v", err)
	}

	waitForSSELine(t, linesCh, errCh, "event: "+core.RunHeartbeatSSEEvent, time.Second)
}

func TestHTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseTS := time.Date(2026, 4, 17, 12, 5, 0, 0, time.UTC)
	runID := "run-1"
	eventOne := newRunEvent(runID, 1, events.EventKindRunStarted, baseTS, `{"status":"started"}`)
	eventTwo := newRunEvent(runID, 2, events.EventKindSessionUpdate, baseTS.Add(time.Second), `{"delta":"hello"}`)

	runSvc := &fakeRunService{
		openStreamFn: func(ctx context.Context, gotRunID string, after core.StreamCursor) (core.RunStream, error) {
			if gotRunID != runID {
				return nil, globaldb.ErrRunNotFound
			}
			stream := newChannelRunStream()
			go func() {
				defer close(stream.events)
				defer close(stream.errors)
				for _, item := range []events.Event{eventOne, eventTwo} {
					if core.EventAfterCursor(item, after) {
						itemCopy := item
						stream.events <- core.RunStreamItem{Event: &itemCopy}
					}
				}
				<-ctx.Done()
			}()
			return stream, nil
		},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName:     "http",
		HeartbeatInterval: 10 * time.Millisecond,
		Runs:              runSvc,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		baseURL+"/api/runs/"+runID+"/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest(first stream) error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do(first stream) error = %v", err)
	}
	firstLines := readSSELinesUntil(t, response.Body, 500*time.Millisecond, func(lines []string) bool {
		return containsLine(lines, "event: "+core.RunEventSSEEvent) &&
			containsSubstring(lines, `"kind":"run.started"`)
	})
	firstID := firstEventID(firstLines)
	if firstID == "" {
		t.Fatalf("first stream missing id:\n%v", firstLines)
	}
	_ = response.Body.Close()

	request, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		baseURL+"/api/runs/"+runID+"/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest(second stream) error = %v", err)
	}
	request.Header.Set("Last-Event-ID", firstID)
	secondResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do(second stream) error = %v", err)
	}
	secondLines := readSSELinesUntil(t, secondResponse.Body, 750*time.Millisecond, func(lines []string) bool {
		return containsSubstring(lines, `"kind":"session.update"`) &&
			containsLine(lines, "event: "+core.RunHeartbeatSSEEvent)
	})
	_ = secondResponse.Body.Close()

	if containsSubstring(secondLines, `"kind":"run.started"`) {
		t.Fatalf("second stream replayed the acknowledged event:\n%v", secondLines)
	}
	if !containsSubstring(secondLines, `"kind":"session.update"`) {
		t.Fatalf("second stream missing resumed event:\n%v", secondLines)
	}
	if !containsLine(secondLines, "event: "+core.RunHeartbeatSSEEvent) {
		t.Fatalf("second stream missing heartbeat:\n%v", secondLines)
	}
}

func TestHTTPStreamRejectsInvalidAndStaleCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseTS := time.Date(2026, 4, 17, 12, 10, 0, 0, time.UTC)
	staleCursor := core.FormatCursor(baseTS, 1)
	runSvc := &fakeRunService{
		openStreamFn: func(_ context.Context, _ string, after core.StreamCursor) (core.RunStream, error) {
			if after.Sequence > 0 {
				return nil, core.NewProblem(
					http.StatusUnprocessableEntity,
					"stale_cursor",
					"cursor is older than retained history",
					map[string]any{"cursor": staleCursor},
					nil,
				)
			}
			return newChannelRunStream(), nil
		},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName: "http",
		Runs:          runSvc,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	testCases := []struct {
		name       string
		lastEvent  string
		wantCode   int
		wantReason string
	}{
		{
			name:       "Should reject invalid cursor syntax",
			lastEvent:  "bad",
			wantCode:   http.StatusUnprocessableEntity,
			wantReason: "invalid_cursor",
		},
		{
			name:       "Should reject stale cursors from trimmed history",
			lastEvent:  staleCursor,
			wantCode:   http.StatusUnprocessableEntity,
			wantReason: "stale_cursor",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				baseURL+"/api/runs/run-1/stream",
				http.NoBody,
			)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			request.Header.Set("Last-Event-ID", tc.lastEvent)

			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if response.StatusCode != tc.wantCode {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.wantCode)
			}

			var payload core.TransportError
			decodeJSON(t, body, &payload)
			if payload.Code != tc.wantReason {
				t.Fatalf("payload.Code = %q, want %q", payload.Code, tc.wantReason)
			}
		})
	}
}

func TestMetricsAndTerminalStreamRemainObservable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	startedAt := time.Date(2026, 4, 17, 12, 20, 0, 0, time.UTC)
	daemonSvc := &fakeDaemonService{
		status: core.DaemonStatus{
			PID:            42,
			Version:        "test",
			StartedAt:      startedAt,
			ActiveRunCount: 0,
			WorkspaceCount: 1,
		},
		health: core.DaemonHealth{Ready: true},
		metrics: core.MetricsPayload{
			Body:        "daemon_active_runs 0\ndaemon_registered_workspaces 1\n",
			ContentType: "text/plain; version=0.0.4; charset=utf-8",
		},
	}
	runSvc := &fakeRunService{
		openStreamFn: func(_ context.Context, runID string, _ core.StreamCursor) (core.RunStream, error) {
			stream := newChannelRunStream()
			go func() {
				defer close(stream.events)
				defer close(stream.errors)
				if runID == "terminal" {
					event := newRunEvent(
						runID,
						3,
						events.EventKindRunCompleted,
						startedAt.Add(time.Second),
						`{"status":"completed"}`,
					)
					stream.events <- core.RunStreamItem{Event: &event}
				}
			}()
			return stream, nil
		},
	}

	httpServer, baseURL := startHTTPServer(t, core.NewHandlers(&core.HandlerConfig{
		TransportName:     "http",
		HeartbeatInterval: 10 * time.Millisecond,
		Daemon:            daemonSvc,
		Runs:              runSvc,
	}))
	defer func() {
		_ = httpServer.Shutdown(context.Background())
	}()

	statusCode, headers, metricsBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/daemon/metrics",
		nil,
		nil,
	)
	if statusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", statusCode)
	}
	if got := strings.TrimSpace(headers.Get(core.HeaderRequestID)); got == "" {
		t.Fatal("metrics X-Request-Id = empty, want non-empty")
	}
	if !strings.Contains(string(metricsBody), "daemon_active_runs 0") {
		t.Fatalf("metrics body = %q, want daemon_active_runs line", metricsBody)
	}

	terminalRequest, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		baseURL+"/api/runs/terminal/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest(terminal stream) error = %v", err)
	}
	terminalResponse, err := http.DefaultClient.Do(terminalRequest)
	if err != nil {
		t.Fatalf("Do(terminal stream) error = %v", err)
	}
	defer terminalResponse.Body.Close()

	streamBody, err := io.ReadAll(terminalResponse.Body)
	if err != nil {
		t.Fatalf("ReadAll(terminal stream) error = %v", err)
	}
	if !strings.Contains(string(streamBody), "event: "+core.RunEventSSEEvent) ||
		!strings.Contains(string(streamBody), `"kind":"run.completed"`) {
		t.Fatalf("terminal stream missing run.event completed payload:\n%s", streamBody)
	}

	statusCode, _, statusBody := mustRequest(
		t,
		http.DefaultClient,
		http.MethodGet,
		baseURL+"/api/daemon/status",
		nil,
		nil,
	)
	if statusCode != http.StatusOK {
		t.Fatalf("status after terminal stream = %d, want 200", statusCode)
	}
	if !bytes.Contains(statusBody, []byte(`"pid":42`)) {
		t.Fatalf("status body = %s, want pid field", statusBody)
	}
}

type fakeDaemonService struct {
	mu      sync.Mutex
	status  core.DaemonStatus
	health  core.DaemonHealth
	metrics core.MetricsPayload
	stopErr error
}

func (f *fakeDaemonService) Status(context.Context) (core.DaemonStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, nil
}

func (f *fakeDaemonService) Health(context.Context) (core.DaemonHealth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.health, nil
}

func (f *fakeDaemonService) Metrics(context.Context) (core.MetricsPayload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.metrics, nil
}

func (f *fakeDaemonService) Stop(context.Context, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopErr
}

func (f *fakeDaemonService) setHealth(health core.DaemonHealth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.health = health
}

func (f *fakeDaemonService) setStatus(update func(core.DaemonStatus) core.DaemonStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = update(f.status)
}

type fakeWorkspaceService struct {
	workspaces []core.Workspace
	workspace  core.Workspace
	items      map[string]core.Workspace
	deleteErr  error
	getErr     error
}

func (f *fakeWorkspaceService) Register(context.Context, string, string) (core.WorkspaceRegisterResult, error) {
	return core.WorkspaceRegisterResult{Workspace: f.workspace, Created: true}, nil
}

func (f *fakeWorkspaceService) List(context.Context) ([]core.Workspace, error) {
	if len(f.workspaces) > 0 {
		return append([]core.Workspace(nil), f.workspaces...), nil
	}
	if len(f.items) == 0 {
		return nil, nil
	}
	items := make([]core.Workspace, 0, len(f.items))
	ids := make([]string, 0, len(f.items))
	for id := range f.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		items = append(items, f.items[id])
	}
	return items, nil
}

func (f *fakeWorkspaceService) Get(_ context.Context, workspaceID string) (core.Workspace, error) {
	if f.getErr != nil {
		return core.Workspace{}, f.getErr
	}
	if item, ok := f.items[workspaceID]; ok {
		return item, nil
	}
	if f.workspace.ID != "" {
		return f.workspace, nil
	}
	return core.Workspace{}, globaldb.ErrWorkspaceNotFound
}

func (f *fakeWorkspaceService) Update(context.Context, string, core.WorkspaceUpdateInput) (core.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeWorkspaceService) Delete(context.Context, string) error {
	return f.deleteErr
}

func (f *fakeWorkspaceService) Resolve(context.Context, string) (core.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeWorkspaceService) Sync(context.Context) (core.WorkspaceSyncResult, error) {
	return core.WorkspaceSyncResult{Checked: len(f.workspaces), Synced: len(f.workspaces)}, nil
}

type fakeTaskService struct {
	run core.Run
}

func (*fakeTaskService) Dashboard(context.Context, string) (core.DashboardPayload, error) {
	return core.DashboardPayload{}, nil
}

func (*fakeTaskService) ListWorkflows(context.Context, string) ([]core.WorkflowSummary, error) {
	return nil, nil
}

func (*fakeTaskService) GetWorkflow(context.Context, string, string) (core.WorkflowSummary, error) {
	return core.WorkflowSummary{}, nil
}

func (*fakeTaskService) WorkflowOverview(context.Context, string, string) (core.WorkflowOverviewPayload, error) {
	return core.WorkflowOverviewPayload{}, nil
}

func (*fakeTaskService) ListItems(context.Context, string, string) ([]core.TaskItem, error) {
	return nil, nil
}

func (*fakeTaskService) TaskBoard(context.Context, string, string) (core.TaskBoardPayload, error) {
	return core.TaskBoardPayload{}, nil
}

func (*fakeTaskService) WorkflowSpec(context.Context, string, string) (core.WorkflowSpecDocument, error) {
	return core.WorkflowSpecDocument{}, nil
}

func (*fakeTaskService) WorkflowMemoryIndex(context.Context, string, string) (core.WorkflowMemoryIndex, error) {
	return core.WorkflowMemoryIndex{}, nil
}

func (*fakeTaskService) WorkflowMemoryFile(context.Context, string, string, string) (core.MarkdownDocument, error) {
	return core.MarkdownDocument{}, nil
}

func (*fakeTaskService) TaskDetail(context.Context, string, string, string) (core.TaskDetailPayload, error) {
	return core.TaskDetailPayload{}, nil
}

func (*fakeTaskService) Validate(context.Context, string, string) (core.ValidationSuccess, error) {
	return core.ValidationSuccess{Valid: true}, nil
}

func (f *fakeTaskService) StartRun(context.Context, string, string, core.TaskRunRequest) (core.Run, error) {
	return f.run, nil
}

func (*fakeTaskService) Archive(context.Context, string, string) (core.ArchiveResult, error) {
	return core.ArchiveResult{Archived: true}, nil
}

type fakeReviewService struct {
	run core.Run
}

func (*fakeReviewService) Fetch(
	context.Context,
	string,
	string,
	core.ReviewFetchRequest,
) (core.ReviewFetchResult, error) {
	return core.ReviewFetchResult{}, nil
}

func (*fakeReviewService) GetLatest(context.Context, string, string) (core.ReviewSummary, error) {
	return core.ReviewSummary{}, nil
}

func (*fakeReviewService) GetRound(context.Context, string, string, int) (core.ReviewRound, error) {
	return core.ReviewRound{}, nil
}

func (*fakeReviewService) ListIssues(context.Context, string, string, int) ([]core.ReviewIssue, error) {
	return nil, nil
}

func (*fakeReviewService) ReviewDetail(context.Context, string, string, int, string) (core.ReviewDetailPayload, error) {
	return core.ReviewDetailPayload{}, nil
}

func (f *fakeReviewService) StartRun(context.Context, string, string, int, core.ReviewRunRequest) (core.Run, error) {
	return f.run, nil
}

type fakeSyncService struct {
	mu      sync.Mutex
	lastReq core.SyncRequest
	result  core.SyncResult
}

func (f *fakeSyncService) Sync(_ context.Context, req core.SyncRequest) (core.SyncResult, error) {
	f.mu.Lock()
	f.lastReq = req
	f.mu.Unlock()
	if f.result.WorkspaceID == "" && f.result.WorkflowSlug == "" && len(f.result.SyncedPaths) == 0 &&
		f.result.SyncedAt == nil {
		return core.SyncResult{WorkspaceID: req.Workspace, WorkflowSlug: req.WorkflowSlug}, nil
	}
	return f.result, nil
}

func (f *fakeSyncService) workspace() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastReq.Workspace
}

type fakeExecService struct {
	run core.Run
}

func (f *fakeExecService) Start(context.Context, core.ExecRequest) (core.Run, error) {
	return f.run, nil
}

type capturingTaskService struct {
	mu                sync.Mutex
	lastDashboardWork string
}

func (s *capturingTaskService) Dashboard(_ context.Context, workspace string) (core.DashboardPayload, error) {
	s.mu.Lock()
	s.lastDashboardWork = workspace
	s.mu.Unlock()
	return core.DashboardPayload{}, nil
}

func (s *capturingTaskService) dashboardWorkspace() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastDashboardWork
}

func (*capturingTaskService) ListWorkflows(context.Context, string) ([]core.WorkflowSummary, error) {
	return nil, nil
}

func (*capturingTaskService) GetWorkflow(context.Context, string, string) (core.WorkflowSummary, error) {
	return core.WorkflowSummary{}, nil
}

func (*capturingTaskService) WorkflowOverview(context.Context, string, string) (core.WorkflowOverviewPayload, error) {
	return core.WorkflowOverviewPayload{}, nil
}

func (*capturingTaskService) ListItems(context.Context, string, string) ([]core.TaskItem, error) {
	return nil, nil
}

func (*capturingTaskService) TaskBoard(context.Context, string, string) (core.TaskBoardPayload, error) {
	return core.TaskBoardPayload{}, nil
}

func (*capturingTaskService) WorkflowSpec(context.Context, string, string) (core.WorkflowSpecDocument, error) {
	return core.WorkflowSpecDocument{}, nil
}

func (*capturingTaskService) WorkflowMemoryIndex(context.Context, string, string) (core.WorkflowMemoryIndex, error) {
	return core.WorkflowMemoryIndex{}, nil
}

func (*capturingTaskService) WorkflowMemoryFile(
	context.Context,
	string,
	string,
	string,
) (core.MarkdownDocument, error) {
	return core.MarkdownDocument{}, nil
}

func (*capturingTaskService) TaskDetail(context.Context, string, string, string) (core.TaskDetailPayload, error) {
	return core.TaskDetailPayload{}, nil
}

func (*capturingTaskService) Validate(context.Context, string, string) (core.ValidationSuccess, error) {
	return core.ValidationSuccess{Valid: true}, nil
}

func (*capturingTaskService) StartRun(context.Context, string, string, core.TaskRunRequest) (core.Run, error) {
	return core.Run{}, nil
}

func (*capturingTaskService) Archive(context.Context, string, string) (core.ArchiveResult, error) {
	return core.ArchiveResult{}, nil
}

type fakeRunService struct {
	runs         map[string]core.Run
	snapshots    map[string]core.RunSnapshot
	eventPages   map[string]core.RunEventPage
	openStreamFn func(context.Context, string, core.StreamCursor) (core.RunStream, error)
}

func (f *fakeRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	if len(f.runs) == 0 {
		return nil, nil
	}
	items := make([]core.Run, 0, len(f.runs))
	runIDs := make([]string, 0, len(f.runs))
	for runID := range f.runs {
		runIDs = append(runIDs, runID)
	}
	sort.Strings(runIDs)
	for _, runID := range runIDs {
		items = append(items, f.runs[runID])
	}
	return items, nil
}

func (f *fakeRunService) Get(_ context.Context, runID string) (core.Run, error) {
	item, ok := f.runs[runID]
	if !ok {
		return core.Run{}, globaldb.ErrRunNotFound
	}
	return item, nil
}

func (f *fakeRunService) Snapshot(_ context.Context, runID string) (core.RunSnapshot, error) {
	item, ok := f.snapshots[runID]
	if !ok {
		return core.RunSnapshot{}, globaldb.ErrRunNotFound
	}
	return item, nil
}

func (f *fakeRunService) RunDetail(_ context.Context, runID string) (core.RunDetailPayload, error) {
	item, ok := f.snapshots[runID]
	if !ok {
		return core.RunDetailPayload{}, globaldb.ErrRunNotFound
	}
	run, ok := f.runs[runID]
	if !ok {
		return core.RunDetailPayload{}, globaldb.ErrRunNotFound
	}
	return core.RunDetailPayload{Run: run, Snapshot: item}, nil
}

func (f *fakeRunService) Events(_ context.Context, runID string, _ core.RunEventPageQuery) (core.RunEventPage, error) {
	if item, ok := f.eventPages[runID]; ok {
		return item, nil
	}
	return core.RunEventPage{}, nil
}

func (f *fakeRunService) OpenStream(
	ctx context.Context,
	runID string,
	after core.StreamCursor,
) (core.RunStream, error) {
	if f.openStreamFn == nil {
		return nil, globaldb.ErrRunNotFound
	}
	return f.openStreamFn(ctx, runID, after)
}

func (f *fakeRunService) Cancel(context.Context, string) error {
	return nil
}

type channelRunStream struct {
	events chan core.RunStreamItem
	errors chan error
}

type blockingPortUpdater struct {
	entered chan int
	release chan struct{}
}

func (u *blockingPortUpdater) SetHTTPPort(_ context.Context, port int) error {
	u.entered <- port
	<-u.release
	return nil
}

type flakyPortUpdater struct {
	failuresRemaining int
}

func (u *flakyPortUpdater) SetHTTPPort(context.Context, int) error {
	if u.failuresRemaining > 0 {
		u.failuresRemaining--
		return errors.New("persist failed")
	}
	return nil
}

func newChannelRunStream() *channelRunStream {
	return &channelRunStream{
		events: make(chan core.RunStreamItem, 8),
		errors: make(chan error, 1),
	}
}

func (c *channelRunStream) Events() <-chan core.RunStreamItem {
	return c.events
}

func (c *channelRunStream) Errors() <-chan error {
	return c.errors
}

func (c *channelRunStream) Close() error {
	return nil
}

func startHTTPServer(t *testing.T, handlers *core.Handlers) (*httpapi.Server, string) {
	t.Helper()

	server, err := httpapi.New(
		httpapi.WithHandlers(handlers),
		httpapi.WithPort(0),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	return server, "http://127.0.0.1:" + strconv.Itoa(server.Port())
}

func startUDSServer(t *testing.T, handlers *core.Handlers, socketPath string) (*udsapi.Server, *http.Client) {
	t.Helper()

	server, err := udsapi.New(
		udsapi.WithHandlers(handlers),
		udsapi.WithSocketPath(socketPath),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return server, client
}

func newShortSocketPath(t *testing.T) string {
	t.Helper()

	file, err := os.CreateTemp("", "compozy-uds-*.sock")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatalf("Close(temp socket file) error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(temp socket file) error = %v", err)
	}
	return path
}

func routeKeys(routes gin.RoutesInfo) []string {
	items := make([]string, 0, len(routes))
	for _, route := range routes {
		items = append(items, route.Method+" "+route.Path)
	}
	sort.Strings(items)
	return items
}

func diffRoutes(left []string, right []string) string {
	if strings.Join(left, "\n") == strings.Join(right, "\n") {
		return ""
	}
	return "left:\n" + strings.Join(left, "\n") + "\nright:\n" + strings.Join(right, "\n")
}

func mustRequest(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body []byte,
	headers map[string]string,
) (int, http.Header, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, rawURL, reader)
	if err != nil {
		t.Fatalf("NewRequest(%s %s) error = %v", method, rawURL, err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(%s %s) error = %v", method, rawURL, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(%s %s) error = %v", method, rawURL, err)
	}
	return response.StatusCode, response.Header.Clone(), responseBody
}

func assertTransportCode(t *testing.T, body []byte, want string) {
	t.Helper()

	var payload core.TransportError
	decodeJSON(t, body, &payload)
	if payload.Code != want {
		t.Fatalf("payload.Code = %q, want %q (body=%s)", payload.Code, want, body)
	}
}

func assertSPAShell(t *testing.T, body []byte) {
	t.Helper()

	if !bytes.Contains(body, []byte(`<div id="app"></div>`)) {
		t.Fatalf("body = %q, want SPA shell", body)
	}
}

func assertNotSPAHTML(t *testing.T, body []byte) {
	t.Helper()

	if bytes.Contains(body, []byte("<!doctype html>")) || bytes.Contains(body, []byte(`<div id="app"></div>`)) {
		t.Fatalf("body = %q, want non-SPA response", body)
	}
}

func firstEmbeddedBundleAsset(t *testing.T) (string, []byte) {
	t.Helper()

	staticFS, err := fs.Sub(webassets.DistFS, "dist")
	if err != nil {
		t.Fatalf("fs.Sub(dist) error = %v", err)
	}

	entries, err := fs.ReadDir(staticFS, "assets")
	if err != nil {
		t.Fatalf("ReadDir(assets) error = %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch path.Ext(entry.Name()) {
		case ".js", ".css":
			assetPath := path.Join("assets", entry.Name())
			assetBody, err := fs.ReadFile(staticFS, assetPath)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", assetPath, err)
			}
			return "/" + assetPath, assetBody
		}
	}

	t.Fatal("expected at least one embedded .js or .css asset")
	return "", nil
}

func readSSELinesUntil(
	t *testing.T,
	body io.ReadCloser,
	timeout time.Duration,
	stop func([]string) bool,
) []string {
	t.Helper()

	linesCh := make(chan string, 64)
	errCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		errCh <- scanner.Err()
		close(linesCh)
	}()

	deadline := time.After(timeout)
	lines := make([]string, 0)
	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				select {
				case err := <-errCh:
					if err != nil {
						t.Fatalf("scanner error = %v", err)
					}
				default:
				}
				return lines
			}
			lines = append(lines, line)
			if stop(lines) {
				return lines
			}
		case err := <-errCh:
			if err != nil {
				t.Fatalf("scanner error = %v", err)
			}
			return lines
		case <-deadline:
			t.Fatalf("timeout reading SSE lines; collected %v", lines)
		}
	}
}

func startSSEScan(body io.ReadCloser) (<-chan string, <-chan error) {
	linesCh := make(chan string, 64)
	errCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		errCh <- scanner.Err()
		close(linesCh)
	}()
	return linesCh, errCh
}

func hasCanonicalSSEFrame(frames []testutil.SSEFrame, event string) bool {
	for _, frame := range frames {
		if frame.Event == event {
			return true
		}
	}
	return false
}

func normalizeCanonicalSSEFrames(frames []testutil.SSEFrame) []testutil.SSEFrame {
	eventsOfInterest := []string{core.RunEventSSEEvent, core.RunHeartbeatSSEEvent, core.RunOverflowSSEEvent}
	normalized := make([]testutil.SSEFrame, 0, len(eventsOfInterest))
	for _, eventName := range eventsOfInterest {
		for _, frame := range frames {
			if frame.Event == eventName {
				normalized = append(normalized, frame)
				break
			}
		}
	}
	return normalized
}

func mustStreamRequest(t *testing.T, client *http.Client, rawURL string, requestID string) *http.Response {
	t.Helper()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest(%s) error = %v", rawURL, err)
	}
	request.Header.Set(core.HeaderRequestID, requestID)

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(%s) error = %v", rawURL, err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatalf("ReadAll(%s) error = %v", rawURL, readErr)
		}
		t.Fatalf("status = %d, want 200; body=%s", response.StatusCode, body)
	}
	if got := strings.TrimSpace(response.Header.Get(core.HeaderRequestID)); got != requestID {
		defer response.Body.Close()
		t.Fatalf("X-Request-Id = %q, want %q", got, requestID)
	}
	return response
}

func ptrTimeHTTP(value time.Time) *time.Time {
	return &value
}

func waitForSSELine(
	t *testing.T,
	linesCh <-chan string,
	errCh <-chan error,
	want string,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				select {
				case err := <-errCh:
					if err != nil {
						t.Fatalf("scanner error = %v", err)
					}
				default:
				}
				t.Fatalf("stream closed before %q was observed", want)
			}
			if line == want {
				return
			}
		case err := <-errCh:
			if err != nil {
				t.Fatalf("scanner error = %v", err)
			}
			t.Fatalf("stream ended before %q was observed", want)
		case <-deadline:
			t.Fatalf("timeout waiting for SSE line %q", want)
		}
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func containsSubstring(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func firstEventID(lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, "id: ") {
			return strings.TrimPrefix(line, "id: ")
		}
	}
	return ""
}

func newRunEvent(runID string, seq uint64, kind events.EventKind, ts time.Time, payload string) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     ts.UTC(),
		Kind:          kind,
		Payload:       json.RawMessage(payload),
	}
}

func decodeJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func assertJSONEqual(t *testing.T, left []byte, right []byte) {
	t.Helper()

	var leftValue any
	var rightValue any
	decodeJSON(t, left, &leftValue)
	decodeJSON(t, right, &rightValue)
	if !reflect.DeepEqual(leftValue, rightValue) {
		t.Fatalf("json mismatch\nleft:  %s\nright: %s", left, right)
	}
}

func assertJSONEqualIgnoringRequestID(t *testing.T, left []byte, right []byte) {
	t.Helper()

	var leftValue map[string]any
	var rightValue map[string]any
	decodeJSON(t, left, &leftValue)
	decodeJSON(t, right, &rightValue)
	delete(leftValue, "request_id")
	delete(rightValue, "request_id")
	if !reflect.DeepEqual(leftValue, rightValue) {
		t.Fatalf("json mismatch ignoring request_id\nleft:  %s\nright: %s", left, right)
	}
}
