package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// RunOptions control the long-lived daemon host process.
type RunOptions struct {
	Version  string
	HTTPPort int
}

type hostRuntime struct {
	db         *globaldb.GlobalDB
	udsServer  *udsapi.Server
	httpServer *httpapi.Server
}

type hostPersistence struct {
	db              *globaldb.GlobalDB
	settings        RunLifecycleSettings
	reconcileResult ReconcileResult
}

// Run starts the singleton daemon host, including persistence, transports, and services.
func Run(ctx context.Context, opts RunOptions) error {
	if ctx == nil {
		return errors.New("daemon: run context is required")
	}

	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	var runtime hostRuntime
	var host *Host

	result, err := Start(runCtx, StartOptions{
		Version:  opts.Version,
		HTTPPort: opts.HTTPPort,
		Healthy:  ProbeReady,
		Prepare: func(startCtx context.Context, currentHost *Host) error {
			host = currentHost
			preparedRuntime, err := prepareHostRuntime(startCtx, runCtx, currentHost, stop)
			if err != nil {
				return err
			}
			runtime = preparedRuntime
			return nil
		},
	})
	if err != nil {
		return err
	}
	if result.Outcome == StartOutcomeAlreadyRunning {
		return nil
	}

	<-runCtx.Done()
	return closeHostRuntime(runtime, host)
}

func prepareHostRuntime(
	startCtx context.Context,
	runCtx context.Context,
	currentHost *Host,
	stop context.CancelFunc,
) (_ hostRuntime, err error) {
	persistence, err := loadHostPersistence(startCtx, currentHost)
	if err != nil {
		return hostRuntime{}, err
	}

	runtime := hostRuntime{
		db: persistence.db,
	}
	defer func() {
		if err == nil {
			return
		}
		err = errors.Join(err, closeHostRuntime(runtime, nil))
	}()

	runManager, err := NewRunManager(RunManagerConfig{
		GlobalDB:             persistence.db,
		LifecycleContext:     runCtx,
		ShutdownDrainTimeout: persistence.settings.ShutdownDrainTimeout,
	})
	if err != nil {
		return hostRuntime{}, err
	}

	handlers := buildHostHandlers(currentHost, persistence, runManager, stop)
	servers, err := startHostTransports(startCtx, currentHost, handlers)
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
		return hostPersistence{}, err
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

	return apicore.NewHandlers(&apicore.HandlerConfig{
		TransportName: "daemon",
		Daemon:        daemonService,
		Workspaces:    newTransportWorkspaceService(persistence.db),
		Tasks:         newTransportTaskService(persistence.db, runManager),
		Reviews:       newTransportReviewService(persistence.db, runManager),
		Runs:          runManager,
		Sync:          newTransportSyncService(persistence.db),
		Exec:          newTransportExecService(runManager),
	})
}

type hostServers struct {
	udsServer  *udsapi.Server
	httpServer *httpapi.Server
}

func startHostTransports(
	ctx context.Context,
	currentHost *Host,
	handlers *apicore.Handlers,
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
		err = errors.Join(err, udsServer.Shutdown(context.Background()))
	}()
	if err := udsServer.Start(ctx); err != nil {
		return hostServers{}, err
	}

	httpServer, err := httpapi.New(
		httpapi.WithHandlers(handlers),
		httpapi.WithPort(currentHost.Info().HTTPPort),
		httpapi.WithPortUpdater(currentHost),
	)
	if err != nil {
		return hostServers{}, err
	}
	defer func() {
		if err == nil {
			return
		}
		err = errors.Join(err, httpServer.Shutdown(context.Background()))
	}()
	if err := httpServer.Start(ctx); err != nil {
		return hostServers{}, err
	}

	return hostServers{
		udsServer:  udsServer,
		httpServer: httpServer,
	}, nil
}

func closeHostRuntime(runtime hostRuntime, host *Host) error {
	var errs []error
	if runtime.httpServer != nil {
		errs = append(errs, runtime.httpServer.Shutdown(context.Background()))
	}
	if runtime.udsServer != nil {
		errs = append(errs, runtime.udsServer.Shutdown(context.Background()))
	}
	if runtime.db != nil {
		errs = append(errs, runtime.db.Close())
	}
	if host != nil {
		errs = append(errs, host.Close(context.Background()))
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
