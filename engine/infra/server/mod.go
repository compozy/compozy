package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sethvargo/go-retry"
)

// Timeout constants for server operations
const (
	monitoringInitTimeout     = 500 * time.Millisecond
	monitoringShutdownTimeout = 5 * time.Second
	dbShutdownTimeout         = 30 * time.Second
	workerShutdownTimeout     = 30 * time.Second
	serverShutdownTimeout     = 5 * time.Second
	scheduleRetryMaxDuration  = 5 * time.Minute
	scheduleRetryBaseDelay    = 1 * time.Second
	scheduleRetryMaxDelay     = 30 * time.Second
	// Use duration-capped exponential retry (~5 minutes total, 30s max delay)
	// HTTP server timeouts
	httpReadTimeout  = 15 * time.Second
	httpWriteTimeout = 15 * time.Second
	httpIdleTimeout  = 60 * time.Second
)

type reconciliationStatus struct {
	mu           sync.RWMutex
	completed    bool
	lastAttempt  time.Time
	lastError    error
	attemptCount int
	nextRetryAt  time.Time
}

func (rs *reconciliationStatus) isReady() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.completed
}

func (rs *reconciliationStatus) getStatus() (bool, time.Time, int, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.completed, rs.lastAttempt, rs.attemptCount, rs.lastError
}

func (rs *reconciliationStatus) setCompleted() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.completed = true
	rs.lastAttempt = time.Now()
	rs.lastError = nil
}

func (rs *reconciliationStatus) setError(err error, nextRetry time.Time) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.lastAttempt = time.Now()
	rs.lastError = err
	rs.attemptCount++
	rs.nextRetryAt = nextRetry
}

type Server struct {
	serverConfig        *config.ServerConfig
	cwd                 string
	configFile          string
	envFilePath         string
	router              *gin.Engine
	monitoring          *monitoring.Service
	redisClient         *redis.Client
	ctx                 context.Context
	cancel              context.CancelFunc
	httpServer          *http.Server
	shutdownChan        chan struct{}
	reconciliationState *reconciliationStatus
}

func NewServer(ctx context.Context, cwd, configFile, envFilePath string) (*Server, error) {
	serverCtx, cancel := context.WithCancel(ctx)
	cfg := config.FromContext(serverCtx)
	if cfg == nil {
		cancel()
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}

	return &Server{
		serverConfig:        &cfg.Server,
		cwd:                 cwd,
		configFile:          configFile,
		envFilePath:         envFilePath,
		ctx:                 serverCtx,
		cancel:              cancel,
		shutdownChan:        make(chan struct{}, 1),
		reconciliationState: &reconciliationStatus{},
	}, nil
}

// setupRedisClient creates a Redis client for rate limiting if Redis is configured
func (s *Server) setupRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
	log := logger.FromContext(s.ctx)

	// If no Redis host is configured, return nil (will use in-memory store)
	if !isRedisConfigured(cfg) {
		log.Debug("Redis not configured, rate limiting will use in-memory store")
		return nil, func() {}, nil
	}

	// Create cache config from app config
	cacheConfig := cache.FromAppConfig(cfg)

	// Create Redis client using the cache package
	redisInstance, err := cache.NewRedis(s.ctx, cacheConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Redis client for rate limiting: %w", err)
	}

	log.Info("Redis client created for rate limiting",
		"host", cfg.Redis.Host,
		"port", cfg.Redis.Port,
		"db", cfg.Redis.DB)

	redisClient := redisInstance.Client()
	// We need *redis.Client for the rate limiting middleware, so we do a type assertion
	client, ok := redisClient.(*redis.Client)
	if !ok {
		return nil, nil, fmt.Errorf("redis client is not a *redis.Client type")
	}

	cleanup := func() {
		if err := redisInstance.Close(); err != nil {
			log.Error("Failed to close Redis client", "error", err)
		}
	}

	return client, cleanup, nil
}

// convertRateLimitConfig creates a rate limit config from the application config
func convertRateLimitConfig(cfg *config.Config) *ratelimit.Config {
	return &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{
			Limit:  cfg.RateLimit.GlobalRate.Limit,
			Period: cfg.RateLimit.GlobalRate.Period,
		},
		APIKeyRate: ratelimit.RateConfig{
			Limit:  cfg.RateLimit.APIKeyRate.Limit,
			Period: cfg.RateLimit.APIKeyRate.Period,
		},
		// RedisAddr, RedisPassword, RedisDB are no longer needed since we pass the client directly
		Prefix:   cfg.RateLimit.Prefix,
		MaxRetry: cfg.RateLimit.MaxRetry,
		ExcludedPaths: []string{
			"/health",
			"/metrics",
			"/swagger",
			routes.HealthVersioned(),
		},
	}
}

func (s *Server) buildRouter(state *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())

	// Get config first
	cfg := config.FromContext(s.ctx)

	// Setup auth middleware first (before rate limiting)
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)
	r.Use(authManager.Middleware())

	// Setup rate limiting
	if cfg.RateLimit.GlobalRate.Limit > 0 {
		log := logger.FromContext(s.ctx)
		rateLimitConfig := convertRateLimitConfig(cfg)

		var manager *ratelimit.Manager
		var err error

		// Use NewManagerWithMetrics if monitoring is initialized
		if s.monitoring != nil && s.monitoring.IsInitialized() {
			manager, err = ratelimit.NewManagerWithMetrics(s.ctx, rateLimitConfig, s.redisClient, s.monitoring.Meter())
		} else {
			manager, err = ratelimit.NewManager(rateLimitConfig, s.redisClient)
		}

		if err != nil {
			log.Error("Failed to initialize rate limiting", "error", err)
			// Continue without rate limiting
		} else {
			// Apply global rate limit middleware
			r.Use(manager.Middleware())
			storageType := "in-memory"
			if s.redisClient != nil {
				storageType = "redis"
			}
			log.Info("Rate limiting enabled",
				"global_limit", cfg.RateLimit.GlobalRate.Limit,
				"global_period", cfg.RateLimit.GlobalRate.Period,
				"storage", storageType)
		}
	}

	// Add monitoring middleware BEFORE other middleware if monitoring is initialized
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		r.Use(s.monitoring.GinMiddleware(s.ctx))
	}
	r.Use(LoggerMiddleware(s.ctx))
	if cfg.Server.CORSEnabled {
		r.Use(CORSMiddleware(cfg.Server.CORS))
	}
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
	// Register metrics endpoint (not versioned under /api/v0/)
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		monitoringPath := state.ProjectConfig.MonitoringConfig.Path
		r.GET(monitoringPath, gin.WrapH(s.monitoring.ExporterHandler()))
	}
	if err := RegisterRoutes(s.ctx, r, state, s); err != nil {
		return err
	}
	s.router = r
	return nil
}

func (s *Server) Run() error {
	// Setup all dependencies
	state, cleanupFuncs, err := s.setupDependencies()
	if err != nil {
		return err
	}
	// Build server routes
	if err := s.buildRouter(state); err != nil {
		s.cleanup(cleanupFuncs)
		return fmt.Errorf("failed to build router: %w", err)
	}

	// Start and run the HTTP server
	return s.startAndRunServer(cleanupFuncs)
}

// setupProjectConfig loads project configuration and returns project config, workflows, and config registry
func (s *Server) setupProjectConfig() (*project.Config, []*workflow.Config, *autoload.ConfigRegistry, error) {
	log := logger.FromContext(s.ctx)
	setupStart := time.Now()
	configService := csvc.NewService(s.envFilePath)
	projectConfig, workflows, configRegistry, err := configService.LoadProject(s.ctx, s.cwd, s.configFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}
	log.Debug("Loaded project configuration", "duration", time.Since(setupStart))
	return projectConfig, workflows, configRegistry, nil
}

// setupMonitoring initializes monitoring service and returns cleanup function
func (s *Server) setupMonitoring(projectConfig *project.Config) func() {
	log := logger.FromContext(s.ctx)
	monitoringStart := time.Now()
	monitoringCtx, monitoringCancel := context.WithTimeout(s.ctx, monitoringInitTimeout)
	monitoringService, err := monitoring.NewMonitoringService(monitoringCtx, projectConfig.MonitoringConfig)
	monitoringDuration := time.Since(monitoringStart)
	if err != nil {
		monitoringCancel()
		if err == context.DeadlineExceeded {
			log.Warn("Monitoring initialization timed out, continuing without monitoring",
				"duration", monitoringDuration)
		} else {
			log.Error("Failed to initialize monitoring service", "error", err,
				"duration", monitoringDuration)
		}
		s.monitoring = nil
		return func() {} // no-op cleanup
	}
	s.monitoring = monitoringService
	if monitoringService.IsInitialized() {
		log.Info("Monitoring service initialized successfully",
			"enabled", projectConfig.MonitoringConfig.Enabled,
			"path", projectConfig.MonitoringConfig.Path,
			"duration", monitoringDuration)
		return func() {
			monitoringCancel()
			ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), monitoringShutdownTimeout)
			defer cancel()
			if err := monitoringService.Shutdown(ctx); err != nil {
				log.Error("Failed to shutdown monitoring service", "error", err)
			}
		}
	}
	monitoringCancel()
	log.Info("Monitoring is disabled in the configuration", "duration", monitoringDuration)
	return func() {} // no-op cleanup
}

// setupStore initializes database store and returns store instance and cleanup function
func (s *Server) setupStore() (*repo.Provider, func(), error) {
	log := logger.FromContext(s.ctx)
	storeStart := time.Now()
	cfg := config.FromContext(s.ctx)
	if cfg == nil {
		return nil, nil, fmt.Errorf(
			"configuration missing from context; attach a manager with config.ContextWithManager",
		)
	}
	// Build driver config
	pgCfg := &postgres.Config{
		ConnString: cfg.Database.ConnString,
		Host:       cfg.Database.Host,
		Port:       cfg.Database.Port,
		User:       cfg.Database.User,
		Password:   cfg.Database.Password,
		DBName:     cfg.Database.DBName,
		SSLMode:    cfg.Database.SSLMode,
	}
	// Apply migrations if enabled (guarded by advisory lock)
	if cfg.Database.AutoMigrate {
		if err := postgres.ApplyMigrationsWithLock(s.ctx, postgres.DSNFor(pgCfg)); err != nil {
			return nil, nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
	}
	// Initialize store
	drv, err := postgres.NewStore(s.ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize postgres store: %w", err)
	}
	provider := repo.NewProvider(drv.Pool())
	cacheDriver := "in-memory"
	if isRedisConfigured(cfg) {
		cacheDriver = "redis"
	}
	log.Info(
		"Database store initialized",
		"store_driver",
		"postgres",
		"mode",
		cfg.Mode,
		"cache_driver",
		cacheDriver,
		"duration",
		time.Since(storeStart),
	)
	cleanup := func() {
		ctx, cancel := context.WithTimeout(s.ctx, dbShutdownTimeout)
		defer cancel()
		_ = drv.Close(ctx)
	}
	return provider, cleanup, nil
}

func (s *Server) setupDependencies() (*appstate.State, []func(), error) {
	var cleanupFuncs []func()
	log := logger.FromContext(s.ctx)
	cfg := config.FromContext(s.ctx)
	setupStart := time.Now()

	projectConfig, workflows, configRegistry, err := s.setupProjectConfig()
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

	// Setup Redis client for rate limiting
	redisClient, redisCleanup, err := s.setupRedisClient(cfg)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	s.redisClient = redisClient
	if redisCleanup != nil {
		cleanupFuncs = append(cleanupFuncs, redisCleanup)
	}

	// Create Temporal config from unified config
	clientConfig := &worker.TemporalConfig{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
		TaskQueue: cfg.Temporal.TaskQueue,
	}
	deps := appstate.NewBaseDeps(projectConfig, workflows, storeInstance, clientConfig)
	workerStart := time.Now()
	worker, err := setupWorker(s.ctx, deps, s.monitoring, configRegistry)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	log.Debug("Worker setup completed", "duration", time.Since(workerStart))
	cleanupFuncs = append(cleanupFuncs, func() {
		ctx, cancel := context.WithTimeout(s.ctx, workerShutdownTimeout)
		defer cancel()
		worker.Stop(ctx)
	})
	state, err := appstate.NewState(deps, worker)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to create app state: %w", err)
	}
	s.initializeScheduleManager(state, worker, workflows)
	log.Info("Server dependencies setup completed", "total_duration", time.Since(setupStart))
	return state, cleanupFuncs, nil
}

func setupWorker(
	ctx context.Context,
	deps appstate.BaseDeps,
	monitoringService *monitoring.Service,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, error) {
	log := logger.FromContext(ctx)
	workerCreateStart := time.Now()
	workerConfig := &worker.Config{
		WorkflowRepo: func() workflow.Repository {
			return deps.Store.NewWorkflowRepo()
		},
		TaskRepo: func() task.Repository {
			return deps.Store.NewTaskRepo()
		},
		MonitoringService: monitoringService,
		ResourceRegistry:  configRegistry,
	}
	worker, err := worker.NewWorker(
		ctx,
		workerConfig,
		deps.ClientConfig,
		deps.ProjectConfig,
		deps.Workflows,
	)
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

func (s *Server) cleanup(cleanupFuncs []func()) {
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		cleanupFuncs[i]()
	}
}

func (s *Server) startAndRunServer(cleanupFuncs []func()) error {
	srv := s.createHTTPServer()
	s.httpServer = srv
	// Start server in goroutine
	go s.startServer(srv)
	// Wait for shutdown signal and handle graceful shutdown
	return s.handleGracefulShutdown(srv, cleanupFuncs)
}

func (s *Server) createHTTPServer() *http.Server {
	addr := fmt.Sprintf("%s:%d", s.serverConfig.Host, s.serverConfig.Port)
	log := logger.FromContext(s.ctx)
	log.Info("Starting HTTP server",
		"address", fmt.Sprintf("http://%s", addr),
	)
	return &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

func (s *Server) startServer(srv *http.Server) {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log := logger.FromContext(s.ctx)
		log.Error("Server failed to start, initiating shutdown", "error", err)
		// Trigger graceful shutdown instead of exiting abruptly
		s.Shutdown()
	}
}

// isRedisConfigured centralizes the logic to determine if Redis should be used.
// Rules:
// - If a full URL is provided, it's configured.
// - If host is empty, it's not configured.
// - If host is localhost with default port and URL empty, treat as not configured.
// - Otherwise, configured.
func isRedisConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.Redis.URL != "" {
		return true
	}
	if cfg.Redis.Host == "" {
		return false
	}
	if cfg.Redis.Host == "localhost" && cfg.Redis.Port == "6379" && cfg.Redis.URL == "" {
		return false
	}
	return true
}

func (s *Server) handleGracefulShutdown(srv *http.Server, cleanupFuncs []func()) error {
	log := logger.FromContext(s.ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		log.Debug("Received shutdown signal, initiating graceful shutdown")
	case <-s.shutdownChan:
		log.Debug("Received programmatic shutdown signal, initiating graceful shutdown")
	}
	// Clean up dependencies first
	s.cleanup(cleanupFuncs)
	s.cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(s.ctx), serverShutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	log.Info("Server shutdown completed successfully")
	return nil
}

func (s *Server) Shutdown() {
	s.shutdownChan <- struct{}{}
}

// GetReconciliationStatus returns the current reconciliation status
func (s *Server) GetReconciliationStatus() (bool, time.Time, int, error) {
	return s.reconciliationState.getStatus()
}

// IsReconciliationReady returns whether the initial reconciliation has completed
func (s *Server) IsReconciliationReady() bool {
	return s.reconciliationState.isReady()
}

func (s *Server) initializeScheduleManager(state *appstate.State, worker *worker.Worker, workflows []*workflow.Config) {
	log := logger.FromContext(s.ctx)

	// Create schedule manager with metrics if monitoring is available
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
	// Run schedule reconciliation in background with retry logic
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
				// Success
				s.reconciliationState.setCompleted()
				log.Info("Schedule reconciliation completed successfully",
					"duration", time.Since(reconcileStart),
					"total_duration", time.Since(startTime))
				return nil
			}

			// Check if error is due to context cancellation
			if ctx.Err() == context.Canceled {
				log.Info("Schedule reconciliation canceled during server shutdown")
				return err // Don't retry on cancellation
			}

			// Log the retry attempt
			log.Warn("Schedule reconciliation failed, will retry",
				"error", err,
				"elapsed", time.Since(startTime))

			// Track the error for status reporting
			s.reconciliationState.setError(err, time.Now().Add(scheduleRetryBaseDelay))

			// Return retryable error to continue retrying
			return retry.RetryableError(err)
		},
	)

	// Handle final result
	if err != nil {
		if s.ctx.Err() == context.Canceled {
			log.Info("Schedule reconciliation canceled during server shutdown")
		} else {
			finalErr := fmt.Errorf("schedule reconciliation failed after maximum retries: %w", err)
			s.reconciliationState.setError(finalErr, time.Time{})
			log.Error("Schedule reconciliation exhausted retries",
				"error", err,
				"duration", time.Since(startTime),
				"max_duration", scheduleRetryMaxDuration)
		}
	}
}
