package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	"github.com/compozy/compozy/engine/infra/server/reconciler"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenvstate"
	"github.com/compozy/compozy/engine/streaming"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
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
	defer monitoringCancel()
	monitoringService, err := monitoring.NewMonitoringService(monitoringCtx, projectConfig.MonitoringConfig)
	monitoringDuration := time.Since(monitoringStart)
	if err != nil {
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
		ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), cfg.Server.Timeouts.DBShutdown)
		defer cancel()
		if err := drv.Close(ctx); err != nil {
			logger.FromContext(s.ctx).Warn("Failed to close store", "error", err)
		}
	}
}

func (s *Server) setupMCPProxyIfEnabled() (func(), error) {
	if !shouldEmbedMCPProxy(s.ctx) {
		return nil, nil
	}
	return s.setupMCPProxy(s.ctx)
}

func (s *Server) setupDependencies() (*appstate.State, []func(), error) {
	cleanupFuncs := make([]func(), 0)
	cfg := config.FromContext(s.ctx)
	setupStart := time.Now()
	cacheInstance, cacheCleanup, err := cache.SetupCache(s.ctx)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	s.cacheInstance = cacheInstance
	cleanupFuncs = appendCleanup(cleanupFuncs, cacheCleanup)
	resourceStore := chooseResourceStore(s.cacheInstance, cfg)
	projectConfig, workflows, configRegistry, err := s.setupProjectConfig(resourceStore)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	storeInstance, cleanupFuncs, err := s.initRuntimeServices(projectConfig, cleanupFuncs)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	temporalCleanup, err := maybeStartStandaloneTemporal(s.ctx)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	cleanupFuncs = appendCleanup(cleanupFuncs, temporalCleanup)
	deps := appstate.NewBaseDeps(projectConfig, workflows, storeInstance, newTemporalConfig(cfg))
	workerInstance, workerCleanup, err := s.maybeStartWorker(deps, resourceStore, configRegistry)
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
	if err := s.attachStreamingProviders(state); err != nil {
		return nil, cleanupFuncs, err
	}
	if err := s.startReconcilerIfNeeded(state, &cleanupFuncs); err != nil {
		return nil, cleanupFuncs, err
	}
	s.emitStartupSummary(time.Since(setupStart))
	return state, cleanupFuncs, nil
}

func (s *Server) attachStreamingProviders(state *appstate.State) error {
	redisClient := s.RedisClient()
	if redisClient == nil {
		return nil
	}
	provider, err := pubsub.NewRedisProvider(redisClient)
	if err != nil {
		return fmt.Errorf("failed to initialize pubsub provider: %w", err)
	}
	state.SetPubSubProvider(provider)
	publisher, err := streaming.NewRedisPublisher(redisClient, nil)
	if err != nil {
		logger.FromContext(s.ctx).Warn("Failed to initialize stream publisher", "error", err)
		return nil
	}
	state.SetStreamPublisher(publisher)
	return nil
}

// initRuntimeServices wires monitoring, database, and MCP proxy services.
func (s *Server) initRuntimeServices(
	projectConfig *project.Config,
	cleanups []func(),
) (*repo.Provider, []func(), error) {
	cleanups = appendCleanup(cleanups, s.setupMonitoring(projectConfig))
	storeInstance, storeCleanup, err := s.setupStore()
	if err != nil {
		return nil, cleanups, err
	}
	cleanups = appendCleanup(cleanups, storeCleanup)
	mcpCleanup, err := s.setupMCPProxyIfEnabled()
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
	if err := s.buildAndStoreToolEnvironment(state, resourceStore, projectConfig, workflows); err != nil {
		return nil, err
	}
	logger.FromContext(s.ctx).Debug("Tool environment initialized and stored")
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

func (s *Server) buildAndStoreToolEnvironment(
	state *appstate.State,
	resourceStore resources.ResourceStore,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) error {
	if state == nil || state.Store == nil {
		return fmt.Errorf("app state or store not available for tool environment")
	}
	workflowRepo := state.Store.NewWorkflowRepo()
	taskRepo := state.Store.NewTaskRepo()
	toolEnv, err := buildToolEnvironment(s.ctx, projectConfig, workflows, workflowRepo, taskRepo, resourceStore)
	if err != nil {
		return fmt.Errorf("failed to build tool environment for app state: %w", err)
	}
	toolenvstate.Store(state, toolEnv)
	return nil
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

func chooseResourceStore(cacheInstance *cache.Cache, cfg *config.Config) resources.ResourceStore {
	if cfg != nil && cfg.Server.SourceOfTruth == sourceRepo {
		return resources.NewMemoryResourceStore()
	}
	if cacheInstance != nil && cacheInstance.Redis != nil {
		client := cacheInstance.Redis.Client()
		if redisClient, ok := client.(*redis.Client); ok {
			return resources.NewRedisResourceStore(redisClient)
		}
	}
	return resources.NewMemoryResourceStore()
}

func maybeStartStandaloneTemporal(ctx context.Context) (func(), error) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required to start Temporal")
	}
	if cfg.EffectiveTemporalMode() != modeStandalone {
		return nil, nil
	}
	embeddedCfg := standaloneEmbeddedConfig(cfg)
	log := logger.FromContext(ctx)
	log.Info(
		"Starting in standalone mode",
		"database", embeddedCfg.DatabaseFile,
		"frontend_port", embeddedCfg.FrontendPort,
		"ui_enabled", embeddedCfg.EnableUI,
	)
	log.Warn("Temporal standalone mode is not recommended for production")
	server, err := embedded.NewServer(ctx, embeddedCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare embedded Temporal server: %w", err)
	}
	if err := server.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start embedded Temporal server: %w", err)
	}
	cfg.Temporal.HostPort = server.FrontendAddress()
	log.Info(
		"Temporal standalone mode started",
		"frontend_addr", cfg.Temporal.HostPort,
		"ui_enabled", embeddedCfg.EnableUI,
		"ui_port", embeddedCfg.UIPort,
	)
	shutdownTimeout := cfg.Server.Timeouts.WorkerShutdown
	if shutdownTimeout <= 0 {
		shutdownTimeout = embeddedCfg.StartTimeout
	}
	return standaloneTemporalCleanup(ctx, server, shutdownTimeout), nil
}

func standaloneEmbeddedConfig(cfg *config.Config) *embedded.Config {
	standalone := cfg.Temporal.Standalone
	return &embedded.Config{
		DatabaseFile: standalone.DatabaseFile,
		FrontendPort: standalone.FrontendPort,
		BindIP:       standalone.BindIP,
		Namespace:    standalone.Namespace,
		ClusterName:  standalone.ClusterName,
		EnableUI:     standalone.EnableUI,
		RequireUI:    standalone.RequireUI,
		UIPort:       standalone.UIPort,
		LogLevel:     standalone.LogLevel,
		StartTimeout: standalone.StartTimeout,
	}
}

func standaloneTemporalCleanup(
	ctx context.Context,
	server *embedded.Server,
	shutdownTimeout time.Duration,
) func() {
	return func() {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := server.Stop(stopCtx); err != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded Temporal server", "error", err)
		}
	}
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
	redisClient := s.RedisClient()
	switch {
	case redisClient != nil:
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
