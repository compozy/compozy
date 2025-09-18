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
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
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
	log := logger.FromContext(s.ctx)
	storeStart := time.Now()
	cfg := config.FromContext(s.ctx)
	if cfg == nil {
		return nil, nil, fmt.Errorf(
			"configuration missing from context; attach a manager with config.ContextWithManager",
		)
	}
	pgCfg := &postgres.Config{
		ConnString: cfg.Database.ConnString,
		Host:       cfg.Database.Host,
		Port:       cfg.Database.Port,
		User:       cfg.Database.User,
		Password:   cfg.Database.Password,
		DBName:     cfg.Database.DBName,
		SSLMode:    cfg.Database.SSLMode,
	}
	if cfg.Database.AutoMigrate {
		mctx, mcancel := context.WithTimeout(s.ctx, cfg.Database.MigrationTimeout)
		defer mcancel()
		if err := postgres.ApplyMigrationsWithLock(mctx, postgres.DSNFor(pgCfg)); err != nil {
			return nil, nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
	}
	drv, err := postgres.NewStore(s.ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize postgres store: %w", err)
	}
	provider := repo.NewProvider(drv.Pool())
	s.storeDriverLabel = "postgres"
	log.Info("Database store initialized",
		"store_driver", s.storeDriverLabel,
		"duration", time.Since(storeStart),
	)
	cleanup := func() {
		ctx, cancel := context.WithTimeout(s.ctx, cfg.Server.Timeouts.DBShutdown)
		defer cancel()
		if err := drv.Close(ctx); err != nil {
			logger.FromContext(s.ctx).Warn("Failed to close store", "error", err)
		}
	}
	return provider, cleanup, nil
}

func (s *Server) setupDependencies() (*appstate.State, []func(), error) {
	var cleanupFuncs []func()
	cfg := config.FromContext(s.ctx)
	setupStart := time.Now()
	redisClient, redisCleanup, err := s.SetupRedisClient(cfg)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	s.redisClient = redisClient
	if redisCleanup != nil {
		cleanupFuncs = append(cleanupFuncs, redisCleanup)
	}
	var resourceStore resources.ResourceStore
	if s.redisClient != nil {
		resourceStore = resources.NewRedisResourceStore(s.redisClient)
	} else {
		resourceStore = resources.NewMemoryResourceStore()
	}
	projectConfig, workflows, configRegistry, err := s.setupProjectConfig(resourceStore)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	monitoringCleanup := s.setupMonitoring(projectConfig)
	cleanupFuncs = append(cleanupFuncs, monitoringCleanup)
	storeInstance, storeCleanup, err := s.setupStore()
	if err != nil {
		return nil, cleanupFuncs, err
	}
	cleanupFuncs = append(cleanupFuncs, storeCleanup)
	if shouldEmbedMCPProxy(cfg) {
		mcpCleanup, mcpErr := s.setupMCPProxy(s.ctx)
		if mcpErr != nil {
			return nil, cleanupFuncs, mcpErr
		}
		if mcpCleanup != nil {
			cleanupFuncs = append(cleanupFuncs, mcpCleanup)
		}
	}
	s.finalizeStartupLabels()
	clientConfig := &worker.TemporalConfig{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
		TaskQueue: cfg.Temporal.TaskQueue,
	}
	deps := appstate.NewBaseDeps(projectConfig, workflows, storeInstance, clientConfig)
	w, wcleanup, werr := s.maybeStartWorker(deps, cfg, configRegistry)
	if werr != nil {
		return nil, cleanupFuncs, werr
	}
	if wcleanup != nil {
		cleanupFuncs = append(cleanupFuncs, wcleanup)
	}
	state, err := appstate.NewState(deps, w)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to create app state: %w", err)
	}
	state.SetResourceStore(resourceStore)
	if w != nil {
		s.initializeScheduleManager(state, w, workflows)
	}
	s.emitStartupSummary(time.Since(setupStart))
	return state, cleanupFuncs, nil
}

func (s *Server) finalizeStartupLabels() {
	switch {
	case s.redisClient != nil:
		s.cacheDriverLabel = "redis"
	default:
		s.cacheDriverLabel = "none"
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
