package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// ServiceConfig wires the daemon-wide status, health, metrics, and stop
// surface exposed to transports.
type ServiceConfig struct {
	Host              *Host
	GlobalDB          *globaldb.GlobalDB
	RunManager        *RunManager
	RequestStop       func(context.Context) error
	ReconcileResult   ReconcileResult
	LifecycleSettings RunLifecycleSettings
	Now               func() time.Time
}

// Service implements the shared transport-facing daemon control surface.
type Service struct {
	host            *Host
	globalDB        *globaldb.GlobalDB
	runManager      *RunManager
	requestStop     func(context.Context) error
	reconcileResult ReconcileResult
	settings        RunLifecycleSettings
	now             func() time.Time

	shutdownConflicts atomic.Int64
}

var _ apicore.DaemonService = (*Service)(nil)

// NewService constructs the daemon-wide control service.
func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	settings := cfg.LifecycleSettings
	if settings.ShutdownDrainTimeout <= 0 {
		settings.ShutdownDrainTimeout = defaultShutdownDrainTimeout
	}
	return &Service{
		host:            cfg.Host,
		globalDB:        cfg.GlobalDB,
		runManager:      cfg.RunManager,
		requestStop:     cfg.RequestStop,
		reconcileResult: cfg.ReconcileResult,
		settings:        settings,
		now:             now,
	}
}

// Status returns the current daemon status snapshot.
func (s *Service) Status(ctx context.Context) (apicore.DaemonStatus, error) {
	info := s.currentInfo()
	workspaceCount, err := s.countWorkspaces(ctx)
	if err != nil {
		return apicore.DaemonStatus{}, err
	}

	status := apicore.DaemonStatus{
		PID:            info.PID,
		Version:        info.Version,
		StartedAt:      info.StartedAt,
		SocketPath:     info.SocketPath,
		HTTPPort:       info.HTTPPort,
		ActiveRunCount: s.activeRunCount(),
		WorkspaceCount: workspaceCount,
	}
	return status, nil
}

// Health reports readiness and any degraded state known to the daemon.
func (s *Service) Health(ctx context.Context) (apicore.DaemonHealth, error) {
	if _, err := s.countWorkspaces(ctx); err != nil {
		return apicore.DaemonHealth{}, err
	}

	info := s.currentInfo()
	health := apicore.DaemonHealth{
		Ready: strings.EqualFold(string(info.State), string(ReadyStateReady)),
	}
	if !health.Ready {
		health.Details = []apicore.HealthDetail{{
			Code:    "daemon_not_ready",
			Message: fmt.Sprintf("daemon is %s", info.State),
		}}
		return health, nil
	}
	if s.reconcileResult.CrashEventFailures > 0 {
		health.Degraded = true
		health.Details = append(health.Details, apicore.HealthDetail{
			Code:     "startup_reconcile_warnings",
			Message:  "one or more recovered runs could not persist a synthetic crash event",
			Severity: "warning",
		})
	}
	return health, nil
}

// Metrics returns the minimal daemon metrics required by the current transport
// contract and lifecycle task set.
func (s *Service) Metrics(ctx context.Context) (apicore.MetricsPayload, error) {
	workspaceCount, err := s.countWorkspaces(ctx)
	if err != nil {
		return apicore.MetricsPayload{}, err
	}

	body := fmt.Sprintf(
		"daemon_active_runs %d\n"+
			"daemon_registered_workspaces %d\n"+
			"daemon_shutdown_conflicts_total %d\n"+
			"runs_reconciled_crashed_total %d\n",
		s.activeRunCount(),
		workspaceCount,
		s.shutdownConflicts.Load(),
		s.reconcileResult.ReconciledRuns,
	)
	return apicore.MetricsPayload{
		Body:        body,
		ContentType: "text/plain; version=0.0.4; charset=utf-8",
	}, nil
}

// Stop enforces the daemon stop contract, delegating active-run ownership to
// the daemon run manager and then invoking the host stop callback.
func (s *Service) Stop(ctx context.Context, force bool) error {
	if s.runManager != nil {
		if err := s.runManager.Shutdown(ctx, force); err != nil {
			s.shutdownConflicts.Add(1)
			return err
		}
	}
	if s.requestStop != nil {
		return s.requestStop(detachContext(ctx))
	}
	return nil
}

func (s *Service) currentInfo() Info {
	if s == nil || s.host == nil {
		return Info{State: ReadyStateStopped}
	}
	return s.host.Info()
}

func (s *Service) activeRunCount() int {
	if s == nil || s.runManager == nil {
		return 0
	}
	return s.runManager.ActiveRunCount()
}

func (s *Service) countWorkspaces(ctx context.Context) (int, error) {
	if s == nil || s.globalDB == nil {
		return 0, nil
	}
	return s.globalDB.CountWorkspaces(detachContext(ctx))
}
