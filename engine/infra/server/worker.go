package server

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
)

func setupWorker(
	ctx context.Context,
	deps appstate.BaseDeps,
	monitoringService *monitoring.Service,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, error) {
	log := logger.FromContext(ctx)
	workerCreateStart := time.Now()
	log.Debug("Initializing worker with workflow configs", "workflow_count", len(deps.Workflows))
	workerConfig := &worker.Config{
		WorkflowRepo:      func() workflow.Repository { return deps.Store.NewWorkflowRepo() },
		TaskRepo:          func() task.Repository { return deps.Store.NewTaskRepo() },
		MonitoringService: monitoringService,
		ResourceRegistry:  configRegistry,
	}
	worker, err := worker.NewWorker(ctx, workerConfig, deps.ClientConfig, deps.ProjectConfig, deps.Workflows)
	if err != nil {
		log.Error("Failed to create worker", "error", err)
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}
	log.Debug("Worker created", "duration", time.Since(workerCreateStart))
	setupStartTime := time.Now()
	if err := worker.Setup(ctx); err != nil {
		log.Error("Failed to setup worker", "error", err)
		return nil, fmt.Errorf("failed to setup worker: %w", err)
	}
	log.Debug("Worker setup done", "duration", time.Since(setupStartTime))
	return worker, nil
}

func (s *Server) maybeStartWorker(
	deps appstate.BaseDeps,
	cfg *config.Config,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, func(), error) {
	log := logger.FromContext(s.ctx)
	if !isHostPortReachable(s.ctx, cfg.Temporal.HostPort, temporalReachabilityTimeout) {
		return nil, nil, fmt.Errorf("temporal not reachable at %s", cfg.Temporal.HostPort)
	}
	s.setTemporalReady(true)
	s.onReadinessMaybeChanged("temporal_reachable")
	start := time.Now()
	w, err := setupWorker(s.ctx, deps, s.monitoring, configRegistry)
	if err != nil {
		return nil, nil, err
	}
	log.Debug("Worker setup completed", "duration", time.Since(start))
	s.setWorkerReady(true)
	s.onReadinessMaybeChanged("worker_ready")
	cleanup := func() {
		ctx, cancel := context.WithTimeout(s.ctx, workerShutdownTimeout)
		defer cancel()
		w.Stop(ctx)
	}
	return w, cleanup, nil
}

func (s *Server) initializeScheduleManager(state *appstate.State, worker *worker.Worker, workflows []*workflow.Config) {
	log := logger.FromContext(s.ctx)
	var scheduleManager schedule.Manager
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		scheduleManager = schedule.NewManagerWithMetrics(
			s.ctx,
			worker.GetWorkerClient(),
			state.ProjectConfig.Name,
			s.monitoring.Meter(),
		)
		log.Debug("Schedule manager initialized with metrics")
	} else {
		scheduleManager = schedule.NewManager(worker.GetWorkerClient(), state.ProjectConfig.Name)
		log.Debug("Schedule manager initialized without metrics")
	}
	state.SetScheduleManager(scheduleManager)
	go s.runReconciliationWithRetry(scheduleManager, workflows)
}

func (s *Server) runReconciliationWithRetry(
	scheduleManager schedule.Manager,
	workflows []*workflow.Config,
) {
	log := logger.FromContext(s.ctx)
	startTime := time.Now()
	backoff := retry.NewExponential(scheduleRetryBaseDelay)
	backoff = retry.WithCappedDuration(scheduleRetryMaxDelay, backoff)
	err := retry.Do(
		s.ctx,
		retry.WithMaxDuration(scheduleRetryMaxDuration, backoff),
		func(ctx context.Context) error {
			reconcileStart := time.Now()
			err := scheduleManager.ReconcileSchedules(ctx, workflows)
			if err == nil {
				s.reconciliationState.setCompleted()
				s.onReadinessMaybeChanged("schedules_reconciled")
				log.Info("Schedule reconciliation completed successfully",
					"duration", time.Since(reconcileStart),
					"total_duration", time.Since(startTime))
				return nil
			}
			if ctx.Err() == context.Canceled {
				log.Info("Schedule reconciliation canceled during server shutdown")
				return err
			}
			log.Warn("Schedule reconciliation failed, will retry",
				"error", err,
				"elapsed", time.Since(startTime))
			s.reconciliationState.setError(err)
			return retry.RetryableError(err)
		},
	)
	if err != nil {
		if s.ctx.Err() == context.Canceled {
			log.Info("Schedule reconciliation canceled during server shutdown")
		} else {
			finalErr := fmt.Errorf("schedule reconciliation failed after maximum retries: %w", err)
			s.reconciliationState.setError(finalErr)
			log.Error("Schedule reconciliation exhausted retries",
				"error", err,
				"duration", time.Since(startTime),
				"max_duration", scheduleRetryMaxDuration)
		}
	}
}
