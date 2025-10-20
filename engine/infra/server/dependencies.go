package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	"github.com/compozy/compozy/engine/infra/server/reconciler"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	redis "github.com/redis/go-redis/v9"
)

func (s *Server) setupProjectConfig(
	store resources.ResourceStore,
) (*project.Config, []*workflow.Config, *autoload.ConfigRegistry, error) {
	log := logger.FromContext(s.ctx)
	setupStart := time.Now()
	configService := csvc.NewService(s.envFilePath, store)
	projectConfig, workflows, configRegistry, err := configService.LoadProject(s.ctx, s.cwd, s.configFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}
	log.Debug("Loaded project configuration", "duration", time.Since(setupStart))
	return projectConfig, workflows, configRegistry, nil
}

func (s *Server) setupMonitoring(projectConfig *project.Config) func() {
	log := logger.FromContext(s.ctx)
	monitoringStart := time.Now()
	timeouts := config.FromContext(s.ctx).Server.Timeouts
	monitoringCtx, monitoringCancel := context.WithTimeout(s.ctx, timeouts.MonitoringInit)
	monitoringService, err := monitoring.NewMonitoringService(monitoringCtx, projectConfig.MonitoringConfig)
	monitoringDuration := time.Since(monitoringStart)
	if err != nil {
		monitoringCancel()
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn("Monitoring initialization timed out, continuing without monitoring",
				"duration", monitoringDuration)
		} else {
			log.Error("Failed to initialize monitoring service", "error", err,
				"duration", monitoringDuration)
		}
		s.monitoring = nil
		return func() {}
	}
	s.monitoring = monitoringService
	if monitoringService.IsInitialized() {
		log.Info("Monitoring service initialized successfully",
			"enabled", projectConfig.MonitoringConfig.Enabled,
			"path", projectConfig.MonitoringConfig.Path,
			"duration", monitoringDuration)
		s.initReadinessMetrics()
		return func() {
			monitoringCancel()
			if s.readyCallback != nil {
				if err := s.readyCallback.Unregister(); err != nil {
					log.Error("Failed to unregister readiness callback", "error", err)
				}
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), timeouts.MonitoringShutdown)
			defer cancel()
			if err := monitoringService.Shutdown(ctx); err != nil {
				log.Error("Failed to shutdown monitoring service", "error", err)
			}
		}
	}
	monitoringCancel()
	log.Info("Monitoring is disabled in the configuration", "duration", monitoringDuration)
	return func() {}
}

func (s *Server) setupStore() (*repo.Provider, func(), error) {
	cfg := config.FromContext(s.ctx)
	if cfg == nil {
		return nil, nil, fmt.Errorf(
			"configuration missing from context; attach a manager with config.ContextWithManager",
		)
	}
	log := logger.FromContext(s.ctx)
	start := time.Now()
	pgCfg := buildPostgresConfigFromApp(cfg)
	if err := s.applyStoreMigrations(pgCfg, cfg); err != nil {
		return nil, nil, err
	}
	drv, err := postgres.NewStore(s.ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize postgres store: %w", err)
	}
	provider := repo.NewProvider(drv.Pool())
	s.storeDriverLabel = driverPostgres
	s.authRepoDriverLabel = driverPostgres
	if s.authCacheDriverLabel == "" {
		s.authCacheDriverLabel = driverNone
	}
	log.Info("Database store initialized",
		"store_driver", s.storeDriverLabel,
		"duration", time.Since(start),
	)
	return provider, s.storeCleanupFunc(cfg, drv), nil
}

// buildPostgresConfigFromApp constructs the postgres.Config from application configuration.
func buildPostgresConfigFromApp(cfg *config.Config) *postgres.Config {
	return &postgres.Config{
		ConnString:         cfg.Database.ConnString,
		Host:               cfg.Database.Host,
		Port:               cfg.Database.Port,
		User:               cfg.Database.User,
		Password:           cfg.Database.Password,
		DBName:             cfg.Database.DBName,
		SSLMode:            cfg.Database.SSLMode,
		MaxOpenConns:       cfg.Database.MaxOpenConns,
		MaxIdleConns:       cfg.Database.MaxIdleConns,
		ConnMaxLifetime:    cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.Database.ConnMaxIdleTime,
		PingTimeout:        cfg.Database.PingTimeout,
		HealthCheckTimeout: cfg.Database.HealthCheckTimeout,
		HealthCheckPeriod:  cfg.Database.HealthCheckPeriod,
		ConnectTimeout:     cfg.Database.ConnectTimeout,
	}
}

// applyStoreMigrations runs database migrations when enabled.
func (s *Server) applyStoreMigrations(pgCfg *postgres.Config, cfg *config.Config) error {
	if !cfg.Database.AutoMigrate {
		return nil
	}
	mctx, mcancel := context.WithTimeout(s.ctx, cfg.Database.MigrationTimeout)
	defer mcancel()
	if err := postgres.ApplyMigrationsWithLock(mctx, postgres.DSNFor(pgCfg)); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

// storeCleanupFunc returns a cleanup function that gracefully closes the store.
func (s *Server) storeCleanupFunc(cfg *config.Config, drv *postgres.Store) func() {
	return func() {
		ctx, cancel := context.WithTimeout(s.ctx, cfg.Server.Timeouts.DBShutdown)
		defer cancel()
		if err := drv.Close(ctx); err != nil {
			logger.FromContext(s.ctx).Warn("Failed to close store", "error", err)
		}
	}
}

func (s *Server) setupMCPProxyIfEnabled(cfg *config.Config) (func(), error) {
	if !shouldEmbedMCPProxy(cfg) {
		return nil, nil
	}
	return s.setupMCPProxy(s.ctx)
}

func (s *Server) setupDependencies() (*appstate.State, []func(), error) {
	cleanupFuncs := make([]func(), 0)
	cfg := config.FromContext(s.ctx)
	setupStart := time.Now()
	redisClient, redisCleanup, err := s.SetupRedisClient(cfg)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	s.redisClient = redisClient
	cleanupFuncs = appendCleanup(cleanupFuncs, redisCleanup)
	resourceStore := chooseResourceStore(s.redisClient, cfg)
	projectConfig, workflows, configRegistry, err := s.setupProjectConfig(resourceStore)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	storeInstance, cleanupFuncs, err := s.initRuntimeServices(cfg, projectConfig, cleanupFuncs)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	deps := appstate.NewBaseDeps(projectConfig, workflows, storeInstance, newTemporalConfig(cfg))
	workerInstance, workerCleanup, err := s.maybeStartWorker(deps, cfg, configRegistry)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	cleanupFuncs = appendCleanup(cleanupFuncs, workerCleanup)
	state, err := s.buildAppState(
		deps,
		workerInstance,
		resourceStore,
		projectConfig,
		workflows,
		configRegistry,
	)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	if err := s.startReconcilerIfNeeded(state, &cleanupFuncs); err != nil {
		return nil, cleanupFuncs, err
	}
	s.emitStartupSummary(time.Since(setupStart))
	return state, cleanupFuncs, nil
}

// initRuntimeServices wires monitoring, database, and MCP proxy services.
func (s *Server) initRuntimeServices(
	cfg *config.Config,
	projectConfig *project.Config,
	cleanups []func(),
) (*repo.Provider, []func(), error) {
	cleanups = appendCleanup(cleanups, s.setupMonitoring(projectConfig))
	storeInstance, storeCleanup, err := s.setupStore()
	if err != nil {
		return nil, cleanups, err
	}
	cleanups = appendCleanup(cleanups, storeCleanup)
	mcpCleanup, err := s.setupMCPProxyIfEnabled(cfg)
	if err != nil {
		return nil, cleanups, err
	}
	cleanups = appendCleanup(cleanups, mcpCleanup)
	s.finalizeStartupLabels()
	return storeInstance, cleanups, nil
}

// buildAppState constructs the application state and seeds supporting services.
func (s *Server) buildAppState(
	deps appstate.BaseDeps,
	w *worker.Worker,
	resourceStore resources.ResourceStore,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	configRegistry *autoload.ConfigRegistry,
) (*appstate.State, error) {
	state, err := appstate.NewState(deps, w)
	if err != nil {
		return nil, fmt.Errorf("failed to create app state: %w", err)
	}
	state.SetMonitoringService(s.monitoring)
	state.SetResourceStore(resourceStore)
	if err := s.seedAndIngestKnowledge(state, resourceStore, projectConfig, workflows); err != nil {
		return nil, err
	}
	if configRegistry != nil {
		state.SetConfigRegistry(configRegistry)
	}
	if w != nil {
		s.initializeScheduleManager(state, w, workflows)
	}
	return state, nil
}

func (s *Server) seedAndIngestKnowledge(
	state *appstate.State,
	store resources.ResourceStore,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) error {
	if err := seedKnowledgeDefinitions(s.ctx, store, projectConfig, workflows); err != nil {
		return fmt.Errorf("seed knowledge definitions: %w", err)
	}
	return ingestKnowledgeBasesOnStart(s.ctx, state, projectConfig, workflows)
}

func chooseResourceStore(redisClient *redis.Client, cfg *config.Config) resources.ResourceStore {
	if cfg != nil && cfg.Server.SourceOfTruth == sourceRepo {
		return resources.NewMemoryResourceStore()
	}
	if redisClient != nil {
		return resources.NewRedisResourceStore(redisClient)
	}
	return resources.NewMemoryResourceStore()
}

// appendCleanup appends a cleanup function when it is non-nil.
func appendCleanup(cleanups []func(), cleanup func()) []func() {
	if cleanup == nil {
		return cleanups
	}
	return append(cleanups, cleanup)
}

// newTemporalConfig creates the Temporal client configuration from app settings.
func newTemporalConfig(cfg *config.Config) *worker.TemporalConfig {
	return &worker.TemporalConfig{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
		TaskQueue: cfg.Temporal.TaskQueue,
	}
}

// startReconcilerIfNeeded wires the reconciler in builder mode and tracks cleanup.
func (s *Server) startReconcilerIfNeeded(state *appstate.State, cleanups *[]func()) error {
	r, err := reconciler.StartIfBuilderMode(s.ctx, state)
	if err != nil {
		return fmt.Errorf("failed to start reconciler: %w", err)
	}
	if r != nil {
		*cleanups = append(*cleanups, func() { r.Stop() })
	}
	return nil
}

func (s *Server) finalizeStartupLabels() {
	switch {
	case s.redisClient != nil:
		s.cacheDriverLabel = "redis"
	default:
		s.cacheDriverLabel = driverNone
	}
}

func (s *Server) emitStartupSummary(total time.Duration) {
	log := logger.FromContext(s.ctx)
	log.Info("Server dependencies setup completed",
		"total_duration", total,
		"store_driver", s.storeDriverLabel,
		"cache_driver", s.cacheDriverLabel,
		"auth_repo_driver", s.authRepoDriverLabel,
		"auth_cache_driver", s.authCacheDriverLabel,
	)
}

func (s *Server) cleanup(cleanupFuncs []func()) {
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		cleanupFuncs[i]()
	}
	s.cleanupMu.Lock()
	extra := s.extraCleanups
	s.extraCleanups = nil
	s.cleanupMu.Unlock()
	for i := len(extra) - 1; i >= 0; i-- {
		extra[i]()
	}
}
