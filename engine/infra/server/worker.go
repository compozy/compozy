package server

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/toolenv/builder"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
)

const (
	maxScheduleRetryAttemptsCap   = 1_000_000
	defaultScheduleRetryBaseDelay = time.Second
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
	workflowRepo := workerConfig.WorkflowRepo()
	if workflowRepo == nil {
		return nil, fmt.Errorf("failed to resolve workflow repository")
	}
	taskRepo := workerConfig.TaskRepo()
	if taskRepo == nil {
		return nil, fmt.Errorf("failed to resolve task repository")
	}
	toolEnv, err := buildToolEnvironment(ctx, deps.ProjectConfig, deps.Workflows, workflowRepo, taskRepo)
	if err != nil {
		log.Error("Failed to build tool environment", "error", err)
		return nil, fmt.Errorf("failed to build tool environment: %w", err)
	}
	worker, err := worker.NewWorker(ctx, workerConfig, deps.ClientConfig, deps.ProjectConfig, deps.Workflows, toolEnv)
	if err != nil {
		log.Error("Failed to create worker", "error", err)
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}
	log.Debug("Worker created", "duration", time.Since(workerCreateStart))
	setupStartTime := time.Now()
	if err := worker.Setup(ctx); err != nil {
		log.Error("Failed to setup worker", "error", err)
		cfg := config.FromContext(ctx)
		stopCtx, cancel := context.WithTimeout(ctx, cfg.Server.Timeouts.WorkerShutdown)
		defer cancel()
		worker.Stop(stopCtx)
		return nil, fmt.Errorf("failed to setup worker: %w", err)
	}
	log.Debug("Worker setup done", "duration", time.Since(setupStartTime))
	return worker, nil
}

func buildToolEnvironment(
	ctx context.Context,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) (toolenv.Environment, error) {
	if projectConfig == nil {
		return nil, fmt.Errorf("project config is required for tool environment")
	}
	if projectConfig.Name == "" {
		return nil, fmt.Errorf("project name is required for tool environment")
	}
	if taskRepo == nil {
		return nil, fmt.Errorf("task repository is required for tool environment")
	}
	store := resources.NewMemoryResourceStore()
	if err := projectConfig.IndexToResourceStore(ctx, store); err != nil {
		return nil, fmt.Errorf("index project resources: %w", err)
	}
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		if err := wf.IndexToResourceStore(ctx, projectConfig.Name, store); err != nil {
			return nil, fmt.Errorf("index workflow %s resources: %w", wf.ID, err)
		}
	}
	env, err := builder.Build(projectConfig, workflows, workflowRepo, taskRepo, store)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (s *Server) maybeStartWorker(
	deps appstate.BaseDeps,
	cfg *config.Config,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, func(), error) {
	log := logger.FromContext(s.ctx)
	if !isHostPortReachable(s.ctx, cfg.Temporal.HostPort, cfg.Server.Timeouts.TemporalReachability) {
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
		ctx, cancel := context.WithTimeout(s.ctx, cfg.Server.Timeouts.WorkerShutdown)
		defer cancel()
		w.Stop(ctx)
	}
	return w, cleanup, nil
}

func (s *Server) initializeScheduleManager(state *appstate.State, worker *worker.Worker, workflows []*workflow.Config) {
	log := logger.FromContext(s.ctx)
	var opts []schedule.Option
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		opts = append(opts, schedule.WithMetrics(s.ctx, s.monitoring.Meter()))
		log.Debug("Schedule manager initialized with metrics")
	} else {
		log.Debug("Schedule manager initialized without metrics")
	}
	scheduleManager := schedule.NewManager(worker.GetWorkerClient(), state.ProjectConfig.Name, opts...)
	state.SetScheduleManager(scheduleManager)
	go s.runReconciliationWithRetry(scheduleManager, workflows)
}

func (s *Server) runReconciliationWithRetry(
	scheduleManager schedule.Manager,
	workflows []*workflow.Config,
) {
	log := logger.FromContext(s.ctx)
	startTime := time.Now()
	cfg := config.FromContext(s.ctx)
	if cfg == nil {
		log.Error("Schedule reconciliation aborted: config missing from context")
		return
	}
	policy := buildScheduleRetryPolicy(cfg)
	err := retry.Do(
		s.ctx,
		policy,
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
	s.handleScheduleReconciliationFailure(err, startTime, cfg)
}

// buildScheduleRetryPolicy constructs the retry policy for schedule reconciliation.
func buildScheduleRetryPolicy(cfg *config.Config) retry.Backoff {
	baseDelay := scheduleRetryBaseDelay(cfg)
	backoff := retry.NewExponential(baseDelay)
	backoff = retry.WithCappedDuration(cfg.Server.Timeouts.ScheduleRetryMaxDelay, backoff)
	policy := retry.WithMaxDuration(cfg.Server.Timeouts.ScheduleRetryMaxDuration, backoff)
	if cfg.Server.Timeouts.ScheduleRetryMaxAttempts >= 1 {
		attempts := min(cfg.Server.Timeouts.ScheduleRetryMaxAttempts, maxScheduleRetryAttemptsCap)
		policy = retry.WithMaxRetries(nonNegativeUint64(attempts), policy)
	}
	return policy
}

// scheduleRetryBaseDelay resolves the base delay for retrying schedule reconciliation.
func scheduleRetryBaseDelay(cfg *config.Config) time.Duration {
	base := cfg.Server.Timeouts.ScheduleRetryBaseDelay
	if secs := cfg.Server.Timeouts.ScheduleRetryBackoffSeconds; secs > 0 {
		base = time.Duration(secs) * time.Second
	}
	if base <= 0 {
		return defaultScheduleRetryBaseDelay
	}
	return base
}

// handleScheduleReconciliationFailure records terminal reconciliation failures.
func (s *Server) handleScheduleReconciliationFailure(
	err error,
	start time.Time,
	cfg *config.Config,
) {
	if err == nil {
		return
	}
	log := logger.FromContext(s.ctx)
	if s.ctx.Err() == context.Canceled {
		log.Info("Schedule reconciliation canceled during server shutdown")
		return
	}
	finalErr := fmt.Errorf("schedule reconciliation failed after maximum retries: %w", err)
	s.reconciliationState.setError(finalErr)
	log.Error("Schedule reconciliation exhausted retries",
		"error", err,
		"duration", time.Since(start),
		"max_duration", cfg.Server.Timeouts.ScheduleRetryMaxDuration)
}

func nonNegativeUint64(value int) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}
