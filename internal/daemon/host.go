package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// RunOptions control the long-lived daemon host process.
type RunOptions struct {
	Version           string
	HTTPPort          int
	Mode              RunMode
	WebDevProxyTarget string
}

type hostRuntime struct {
	runManager      *RunManager
	db              *globaldb.GlobalDB
	udsServer       *udsapi.Server
	httpServer      *httpapi.Server
	shutdownTimeout time.Duration
}

type hostPersistence struct {
	db              *globaldb.GlobalDB
	settings        RunLifecycleSettings
	reconcileResult ReconcileResult
}

var (
	shutdownRunManager = func(ctx context.Context, manager *RunManager) error {
		return manager.Shutdown(ctx, true)
	}
	shutdownHTTPServer = func(ctx context.Context, server *httpapi.Server) error {
		return server.Shutdown(ctx)
	}
	shutdownUDSServer = func(ctx context.Context, server *udsapi.Server) error {
		return server.Shutdown(ctx)
	}
	closeHostGlobalDB = func(ctx context.Context, db *globaldb.GlobalDB) error {
		return db.CloseContext(ctx)
	}
	closeDaemonHost = func(ctx context.Context, host *Host) error {
		return host.Close(ctx)
	}
)

// Run starts the singleton daemon host, including persistence, transports, and services.
func Run(ctx context.Context, opts RunOptions) (retErr error) {
	signalCtx, stopSignals, err := daemonRunSignalContext(ctx, opts.Mode)
	if err != nil {
		return err
	}
	defer stopSignals()

	runCtx, stop := context.WithCancel(signalCtx)
	defer stop()

	var runtime hostRuntime
	var host *Host
	var daemonLogger *logger.Runtime
	defer func() {
		if daemonLogger != nil {
			retErr = errors.Join(retErr, daemonLogger.Close())
		}
	}()

	mode := resolveRunMode(opts.Mode)
	result, err := Start(runCtx, StartOptions{
		Version:  opts.Version,
		HTTPPort: opts.HTTPPort,
		Healthy:  ProbeReady,
		Prepare: func(startCtx context.Context, currentHost *Host) error {
			host = currentHost
			preparedRuntime, preparedLogger, err := prepareRuntimeForRun(
				startCtx,
				runCtx,
				currentHost,
				stop,
				mode,
				opts,
			)
			if err != nil {
				return err
			}
			runtime = preparedRuntime
			daemonLogger = preparedLogger
			return nil
		},
	})
	if err != nil {
		return err
	}
	if result.Outcome == StartOutcomeAlreadyRunning {
		return nil
	}

	logDaemonStarted(mode, result.Info)
	return waitForRunShutdown(runCtx, runtime, host, mode)
}

func prepareRuntimeForRun(
	startCtx context.Context,
	runCtx context.Context,
	currentHost *Host,
	stop context.CancelFunc,
	mode RunMode,
	opts RunOptions,
) (hostRuntime, *logger.Runtime, error) {
	daemonLogger, err := logger.InstallDaemonLogger(logger.DaemonConfig{
		FilePath: currentHost.Paths().LogFile,
		Mode:     logger.Mode(mode),
	})
	if err != nil {
		return hostRuntime{}, nil, fmt.Errorf("daemon: configure logger: %w", err)
	}
	logDaemonStarting(mode, currentHost)

	runtime, err := prepareHostRuntime(startCtx, runCtx, currentHost, stop, opts)
	if err != nil {
		_ = daemonLogger.Close()
		return hostRuntime{}, nil, err
	}
	return runtime, daemonLogger, nil
}

func logDaemonStarting(mode RunMode, currentHost *Host) {
	slog.Info(
		"daemon starting",
		"mode",
		mode,
		"pid",
		currentHost.Info().PID,
		"socket_path",
		currentHost.Paths().SocketPath,
		"log_path",
		currentHost.Paths().LogFile,
	)
}

func logDaemonStarted(mode RunMode, info Info) {
	slog.Info(
		"daemon started",
		"mode",
		mode,
		"pid",
		info.PID,
		"http_port",
		info.HTTPPort,
		"socket_path",
		info.SocketPath,
	)
}

func waitForRunShutdown(runCtx context.Context, runtime hostRuntime, host *Host, mode RunMode) error {
	<-runCtx.Done()
	if cause := context.Cause(runCtx); cause != nil {
		slog.Info("daemon shutdown requested", "mode", mode, "reason", cause.Error())
	} else {
		slog.Info("daemon shutdown requested", "mode", mode)
	}
	if err := closeHostRuntime(runCtx, runtime, host); err != nil {
		slog.Error("daemon shutdown failed", "mode", mode, "error", err)
		return err
	}
	slog.Info("daemon stopped", "mode", mode)
	return nil
}

func prepareHostRuntime(
	startCtx context.Context,
	runCtx context.Context,
	currentHost *Host,
	stop context.CancelFunc,
	opts RunOptions,
) (_ hostRuntime, err error) {
	persistence, err := loadHostPersistence(startCtx, currentHost)
	if err != nil {
		return hostRuntime{}, err
	}

	runtime := hostRuntime{
		db:              persistence.db,
		shutdownTimeout: persistence.settings.ShutdownDrainTimeout,
	}
	defer func() {
		if err == nil {
			return
		}
		err = errors.Join(err, closeHostRuntime(startCtx, runtime, nil))
	}()

	runManager, err := NewRunManager(RunManagerConfig{
		GlobalDB:             persistence.db,
		LifecycleContext:     runCtx,
		ShutdownDrainTimeout: persistence.settings.ShutdownDrainTimeout,
	})
	if err != nil {
		return hostRuntime{}, err
	}
	runtime.runManager = runManager

	handlers := buildHostHandlers(currentHost, persistence, runManager, stop)
	servers, err := startHostTransports(
		startCtx,
		persistence.settings.ShutdownDrainTimeout,
		currentHost,
		handlers,
		opts,
	)
	if err != nil {
		return hostRuntime{}, err
	}
	runtime.udsServer = servers.udsServer
	runtime.httpServer = servers.httpServer
	return runtime, nil
}

func loadHostPersistence(ctx context.Context, currentHost *Host) (_ hostPersistence, err error) {
	if err := ensureHomeLayout(); err != nil {
		return hostPersistence{}, err
	}

	paths := currentHost.Paths()
	db, err := globaldb.Open(ctx, paths.GlobalDBPath)
	if err != nil {
		return hostPersistence{}, err
	}
	defer func() {
		if err == nil {
			return
		}
		err = errors.Join(err, db.Close())
	}()

	settings, _, err := LoadRunLifecycleSettings(ctx)
	if err != nil {
		return hostPersistence{}, err
	}
	reconcileResult, err := ReconcileStartup(ctx, ReconcileConfig{
		HomePaths: paths,
	})
	if err != nil {
		return hostPersistence{}, fmt.Errorf("refresh registered workspaces: %w", err)
	}
	workspaceRefresh, err := refreshRegisteredWorkspaces(ctx, db, workspaceRefreshOptions{
		SyncPresent: false,
	})
	if err != nil {
		return hostPersistence{}, err
	}
	if workspaceRefresh.Checked > 0 || len(workspaceRefresh.Warnings) > 0 {
		slog.Info(
			"daemon workspace catalog refreshed",
			"checked",
			workspaceRefresh.Checked,
			"removed",
			workspaceRefresh.Removed,
			"missing",
			workspaceRefresh.Missing,
			"warnings",
			len(workspaceRefresh.Warnings),
		)
	}

	return hostPersistence{
		db:              db,
		settings:        settings,
		reconcileResult: reconcileResult,
	}, nil
}

func buildHostHandlers(
	currentHost *Host,
	persistence hostPersistence,
	runManager *RunManager,
	stop context.CancelFunc,
) *apicore.Handlers {
	daemonService := NewService(ServiceConfig{
		Host:              currentHost,
		GlobalDB:          persistence.db,
		RunManager:        runManager,
		ReconcileResult:   persistence.reconcileResult,
		LifecycleSettings: persistence.settings,
		RequestStop: func(context.Context) error {
			stop()
			return nil
		},
	})
	queryService := NewQueryService(QueryServiceConfig{
		GlobalDB:   persistence.db,
		RunManager: runManager,
		Daemon:     daemonService,
	})

	return apicore.NewHandlers(&apicore.HandlerConfig{
		TransportName:   "daemon",
		Daemon:          daemonService,
		Workspaces:      newTransportWorkspaceService(persistence.db),
		Tasks:           newTransportTaskService(persistence.db, runManager, queryService),
		Reviews:         newTransportReviewService(persistence.db, runManager, queryService),
		Runs:            runManager,
		Sync:            newTransportSyncService(persistence.db, runManager),
		Exec:            newTransportExecService(runManager),
		WorkspaceEvents: runManager,
	})
}

type hostServers struct {
	udsServer  *udsapi.Server
	httpServer *httpapi.Server
}

func startHostTransports(
	ctx context.Context,
	shutdownTimeout time.Duration,
	currentHost *Host,
	handlers *apicore.Handlers,
	opts RunOptions,
) (_ hostServers, err error) {
	udsServer, err := udsapi.New(
		udsapi.WithHandlers(handlers),
		udsapi.WithSocketPath(currentHost.Paths().SocketPath),
	)
	if err != nil {
		return hostServers{}, err
	}
	defer func() {
		if err == nil {
			return
		}
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		defer cancel()
		err = errors.Join(err, shutdownUDSServer(shutdownCtx, udsServer))
	}()
	if err := udsServer.Start(ctx); err != nil {
		return hostServers{}, err
	}

	httpServer, err := httpapi.New(
		httpapi.WithHandlers(handlers),
		httpapi.WithPort(currentHost.Info().HTTPPort),
		httpapi.WithPortUpdater(currentHost),
		httpapi.WithDevProxyTarget(opts.WebDevProxyTarget),
	)
	if err != nil {
		return hostServers{}, err
	}
	defer func() {
		if err == nil {
			return
		}
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		defer cancel()
		err = errors.Join(err, shutdownHTTPServer(shutdownCtx, httpServer))
	}()
	if err := httpServer.Start(ctx); err != nil {
		return hostServers{}, err
	}

	return hostServers{
		udsServer:  udsServer,
		httpServer: httpServer,
	}, nil
}

func closeHostRuntime(ctx context.Context, runtime hostRuntime, host *Host) error {
	shutdownTimeout := runtime.shutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownDrainTimeout
	}
	var errs []error
	if runtime.runManager != nil {
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		errs = append(errs, shutdownRunManager(shutdownCtx, runtime.runManager))
		cancel()
	}
	if runtime.httpServer != nil {
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		errs = append(errs, shutdownHTTPServer(shutdownCtx, runtime.httpServer))
		cancel()
	}
	if runtime.udsServer != nil {
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		errs = append(errs, shutdownUDSServer(shutdownCtx, runtime.udsServer))
		cancel()
	}
	if runtime.db != nil {
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		errs = append(errs, closeHostGlobalDB(shutdownCtx, runtime.db))
		cancel()
	}
	if host != nil {
		shutdownCtx, cancel := boundedLifecycleContext(ctx, shutdownTimeout)
		errs = append(errs, closeDaemonHost(shutdownCtx, host))
		cancel()
	}
	return errors.Join(errs...)
}

// ProbeReady verifies that one daemon info record points at a healthy transport.
func ProbeReady(ctx context.Context, info Info) error {
	if err := info.Validate(); err != nil {
		return err
	}
	if info.State != ReadyStateReady {
		return fmt.Errorf("daemon: daemon is not ready (state=%s)", info.State)
	}
	if !ProcessAlive(info.PID) {
		return fmt.Errorf("daemon: daemon pid %d is not running", info.PID)
	}

	client, err := apiclient.New(apiclient.Target{
		SocketPath: strings.TrimSpace(info.SocketPath),
		HTTPPort:   info.HTTPPort,
	})
	if err != nil {
		return err
	}

	health, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("daemon: health probe failed: %w", err)
	}
	if !health.Ready {
		return daemonHealthProblem(health)
	}
	return nil
}

func daemonHealthProblem(health apicore.DaemonHealth) error {
	message := "daemon is not ready"
	if len(health.Details) > 0 {
		detail := strings.TrimSpace(health.Details[0].Message)
		if detail != "" {
			message = detail
		}
	}
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"daemon_not_ready",
		message,
		nil,
		nil,
	)
}
