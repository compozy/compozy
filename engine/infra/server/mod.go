package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/postgres"
	rediscache "github.com/compozy/compozy/engine/infra/redis"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	temporal "github.com/compozy/compozy/engine/infra/server/temporal"
	"github.com/compozy/compozy/engine/infra/sqlite"
	sugarauth "github.com/compozy/compozy/engine/infra/sugardb"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/compozy/compozy/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sethvargo/go-retry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Timeout constants for server operations
const (
	// Status constants for server readiness
	statusNotReady = "not_ready"
	statusReady    = "ready"
	// Timeout constants for server operations
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
	modeStandalone   = "standalone"
	// MCP readiness probe defaults (config overrides total timeout)
	mcpHealthPollInterval   = 200 * time.Millisecond
	mcpHealthRequestTimeout = 500 * time.Millisecond
	hostAny                 = "0.0.0.0"
	hostLoopback            = "127.0.0.1"
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
	// readiness aggregation and embedded server (standalone)
	readinessMu      sync.RWMutex
	temporalReady    bool
	workerReady      bool
	embeddedTemporal interface{ Stop() error }

	// embedded MCP proxy (standalone)
	mcpMu      sync.RWMutex
	mcpReady   bool
	mcpBaseURL string
	mcpProxy   interface {
		Start(context.Context) error
		Stop(context.Context) error
	}

	// readiness metrics
	readyGauge            metric.Int64ObservableGauge
	readyTransitionsTotal metric.Int64Counter
	readyCallback         metric.Registration
	lastReady             bool

	shutdownOnce sync.Once

	// embedded SugarDB for auth cache (standalone)
	sugarAuthCleanup func()

	// observability labels captured at startup
	modeLabel            string
	storeDriverLabel     string
	cacheDriverLabel     string
	authRepoDriverLabel  string
	authCacheDriverLabel string
}

func NewServer(ctx context.Context, cwd, configFile, envFilePath string) (*Server, error) {
	serverCtx, cancel := context.WithCancel(ctx)
	cfg := config.FromContext(serverCtx)
	if cfg == nil {
		cancel()
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}

	// Do not mutate cfg.CLI.CWD; the server uses its own cwd field.

	return &Server{
		serverConfig:        &cfg.Server,
		cwd:                 cwd,
		configFile:          configFile,
		envFilePath:         envFilePath,
		ctx:                 serverCtx,
		cancel:              cancel,
		shutdownChan:        make(chan struct{}, 1),
		reconciliationState: &reconciliationStatus{},
		lastReady:           false,
	}, nil
}

// SetupRedisClient creates a Redis client for rate limiting if Redis is configured
func (s *Server) SetupRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
	log := logger.FromContext(s.ctx)
	// In standalone mode, use in-memory limiter unless Redis is explicitly configured
	if cfg != nil && cfg.Mode == modeStandalone && !isRedisConfigured(cfg) {
		log.Info(
			"rate limiter initialized",
			"driver",
			"memory",
			"mode",
			cfg.Mode,
			"note",
			"best-effort, single-process semantics",
		)
		return nil, func() {}, nil
	}

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
	// NOTE(leaky-abstraction): The rate limiter depends on ulule/limiter's Redis store
	// which accepts a concrete *redis.Client. Our cache layer exposes a generic
	// redis.UniversalClient. We intentionally assert to *redis.Client here to avoid
	// over-abstracting the limiter path. If in the future we wrap limiter with our
	// own store abstraction, this assertion can be removed.
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

// buildAuthRepo composes the auth repository with an optional cache decorator
// and returns the decorated repo and selected cache driver label.
func (s *Server) buildAuthRepo(cfg *config.Config, base authuc.Repository) (authuc.Repository, string) {
	repo := base
	driver := "none"
	const cacheDriverRedis = "redis"
	ttl := cfg.Cache.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if s.redisClient != nil {
		repo = rediscache.NewCachedRepository(repo, s.redisClient, ttl)
		driver = cacheDriverRedis
		return repo, driver
	}
	if cfg.Mode == modeStandalone {
		if sugar, err := sugarauth.NewEmbedded(s.ctx); err == nil && sugar != nil {
			repo = sugarauth.NewAuthCachedRepository(repo, sugar.DB(), ttl)
			driver = "sugardb"
			// register cleanup
			s.sugarAuthCleanup = func() {
				if stopErr := sugar.Stop(); stopErr != nil {
					logger.FromContext(s.ctx).Warn("SugarDB auth cache stop failed", "error", stopErr)
				}
			}
		} else {
			logger.FromContext(s.ctx).Warn("SugarDB auth cache not initialized; continuing without cache")
		}
	}
	return repo, driver
}

func (s *Server) buildRouter(state *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())

	// Get config first
	cfg := config.FromContext(s.ctx)

	// Setup auth middleware first (before rate limiting)
	baseAuth := state.Store.NewAuthRepo()
	authRepoDriver := "postgres"
	if cfg.Mode == modeStandalone {
		authRepoDriver = "sqlite"
	}
	authRepo, authCacheDriver := s.buildAuthRepo(cfg, baseAuth)
	// persist labels and emit unified log
	s.authRepoDriverLabel = authRepoDriver
	s.authCacheDriverLabel = authCacheDriver
	logger.FromContext(s.ctx).Info(
		"auth repository configured",
		"auth_repo_driver", authRepoDriver,
		"auth_cache_driver", authCacheDriver,
	)
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
			driver := "memory"
			if s.redisClient != nil {
				driver = "redis"
			}
			log.Info("rate limiter initialized",
				"driver", driver,
				"mode", cfg.Mode,
				"global_limit", cfg.RateLimit.GlobalRate.Limit,
				"global_period", cfg.RateLimit.GlobalRate.Period)
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
		s.cleanup(cleanupFuncs)
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
		// Initialize readiness metrics
		s.initReadinessMetrics()
		return func() {
			monitoringCancel()
			if s.readyCallback != nil {
				if err := s.readyCallback.Unregister(); err != nil {
					log.Error("Failed to unregister readiness callback", "error", err)
				}
			}
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
	// If running in standalone, use SQLite-only store and skip Postgres entirely
	if cfg.Mode == modeStandalone {
		projDir := s.cwd
		stateDir := filepath.Join(projDir, ".compozy", "state")
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create state dir: %w", err)
		}
		dbPath := filepath.Join(stateDir, "compozy.sqlite")
		sq, err := sqlite.NewStore(s.ctx, dbPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize sqlite store: %w", err)
		}
		if err := sqlite.ApplyMigrations(s.ctx, sq.DB()); err != nil {
			_ = sq.Close(s.ctx)
			return nil, nil, fmt.Errorf("failed to apply sqlite migrations: %w", err)
		}
		provider := repo.NewProvider(cfg.Mode, nil, sq)
		s.modeLabel = cfg.Mode
		s.storeDriverLabel = "sqlite"
		log.Info("Database store initialized",
			"store_driver", s.storeDriverLabel,
			"mode", s.modeLabel,
			"duration", time.Since(storeStart),
		)
		cleanup := func() {
			ctx, cancel := context.WithTimeout(s.ctx, dbShutdownTimeout)
			defer cancel()
			_ = sq.Close(ctx)
		}
		return provider, cleanup, nil
	}

	// Distributed mode: initialize Postgres store as primary
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
		if err := postgres.ApplyMigrationsWithLock(s.ctx, postgres.DSNFor(pgCfg)); err != nil {
			return nil, nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
	}
	drv, err := postgres.NewStore(s.ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize postgres store: %w", err)
	}
	provider := repo.NewProvider(cfg.Mode, drv.Pool(), nil)
	// set labels for later metric decoration and startup summary
	s.modeLabel = cfg.Mode
	s.storeDriverLabel = "postgres"
	// cache driver label is decided later once redis/sugardb are wired
	log.Info("Database store initialized",
		"store_driver", s.storeDriverLabel,
		"mode", s.modeLabel,
		"duration", time.Since(storeStart),
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

	// Start embedded MCP Proxy first in standalone so workers can register MCPs
	if cfg.Mode == modeStandalone {
		mcpCleanup, mcpErr := s.setupMCPProxy(s.ctx)
		if mcpErr != nil {
			return nil, cleanupFuncs, mcpErr
		}
		if mcpCleanup != nil {
			cleanupFuncs = append(cleanupFuncs, mcpCleanup)
		}
	}

	// Start embedded Temporal (standalone mode)
	if tCleanup, terr := s.setupEmbeddedTemporal(projectConfig, cfg); terr == nil && tCleanup != nil {
		cleanupFuncs = append(cleanupFuncs, tCleanup)
	} else if terr != nil {
		log.Warn("Embedded Temporal not started", "error", terr)
	}

	// Setup Redis client for rate limiting
	redisClient, redisCleanup, err := s.SetupRedisClient(cfg)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	s.redisClient = redisClient
	if redisCleanup != nil {
		cleanupFuncs = append(cleanupFuncs, redisCleanup)
	}
	// Decide cache driver label and persist
	s.finalizeStartupLabels()

	// Create Temporal config from unified config for app state
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
	if w != nil {
		s.initializeScheduleManager(state, w, workflows)
	}
	// Emit a single, consistent startup summary with labels
	s.emitStartupSummary(time.Since(setupStart))
	return state, cleanupFuncs, nil
}

// finalizeStartupLabels computes derived driver labels once dependencies are wired.
func (s *Server) finalizeStartupLabels() {
	switch {
	case s.redisClient != nil:
		s.cacheDriverLabel = "redis"
	case s.sugarAuthCleanup != nil:
		s.cacheDriverLabel = "sugardb"
	default:
		s.cacheDriverLabel = "none"
	}
}

// emitStartupSummary logs the server startup summary with observability labels.
func (s *Server) emitStartupSummary(total time.Duration) {
	log := logger.FromContext(s.ctx)
	log.Info("Server dependencies setup completed",
		"total_duration", total,
		"mode", s.modeLabel,
		"store_driver", s.storeDriverLabel,
		"cache_driver", s.cacheDriverLabel,
		"auth_repo_driver", s.authRepoDriverLabel,
		"auth_cache_driver", s.authCacheDriverLabel,
	)
}

// setupEmbeddedTemporal starts embedded_temporal in standalone mode and wires cleanup.
func (s *Server) setupEmbeddedTemporal(projectConfig *project.Config, cfg *config.Config) (func(), error) {
	if cfg.Mode != modeStandalone || !cfg.Temporal.DevServerEnabled {
		return nil, nil
	}
	log := logger.FromContext(s.ctx)
	projDir := projectConfig.GetCWD().PathStr()
	stateDir := filepath.Join(projDir, ".compozy", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create state dir: %w", err)
	}
	tStart := time.Now()
	tsrv, terr := temporal.StartEmbedded(s.ctx, cfg, stateDir)
	if terr != nil {
		return nil, terr
	}
	s.setTemporalReady(true)
	s.onReadinessMaybeChanged("temporal_ready")
	log.Info("Temporal namespace ready", "duration", time.Since(tStart))
	s.embeddedTemporal = tsrv
	return func() {
		if s.embeddedTemporal != nil {
			if err := tsrv.Stop(); err != nil {
				log.Warn("Failed to stop embedded temporal", "error", err)
			}
		}
	}, nil
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

// maybeStartWorker decides whether to start the Temporal worker based on mode and availability.
// In standalone mode, if the embedded server is not ready and HostPort is unreachable, it skips worker startup.
func (s *Server) maybeStartWorker(
	deps appstate.BaseDeps,
	cfg *config.Config,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, func(), error) {
	log := logger.FromContext(s.ctx)
	// Do not gate worker on external Redis in standalone mode.
	if cfg.Mode == modeStandalone && !s.isTemporalReady() {
		if !isHostPortReachable(s.ctx, cfg.Temporal.HostPort, 1500*time.Millisecond) {
			log.Warn("Temporal not reachable; starting server without worker in standalone",
				"host_port", cfg.Temporal.HostPort)
			s.setWorkerReady(false)
			s.onReadinessMaybeChanged("worker_skipped")
			return nil, nil, nil
		}
	}
	// Start worker normally
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

func (s *Server) cleanup(cleanupFuncs []func()) {
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		cleanupFuncs[i]()
	}
	// best-effort additional cleanups
	if s.sugarAuthCleanup != nil {
		s.sugarAuthCleanup()
	}
}

// initReadinessMetrics registers readiness gauge and transition counter.
func (s *Server) initReadinessMetrics() {
	if s.monitoring == nil || !s.monitoring.IsInitialized() {
		return
	}
	log := logger.FromContext(s.ctx)
	meter := s.monitoring.Meter()
	g, err := meter.Int64ObservableGauge(
		"compozy_server_ready",
		metric.WithDescription("Server readiness: 1 ready, 0 not_ready"),
	)
	if err == nil {
		s.readyGauge = g
		reg, regErr := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			val := int64(0)
			if s.isFullyReady() {
				val = 1
			}
			o.ObserveInt64(
				s.readyGauge,
				val,
				metric.WithAttributes(
					attribute.String("mode", s.modeLabel),
					attribute.String("store_driver", s.storeDriverLabel),
					attribute.String("cache_driver", s.cacheDriverLabel),
					attribute.String("auth_repo_driver", s.authRepoDriverLabel),
					attribute.String("auth_cache_driver", s.authCacheDriverLabel),
				),
			)
			return nil
		}, s.readyGauge)
		if regErr != nil {
			log.Error("Failed to register readiness gauge callback", "error", regErr)
		} else {
			s.readyCallback = reg
		}
	} else {
		log.Error("Failed to create readiness gauge", "error", err)
	}
	c, err := meter.Int64Counter(
		"compozy_server_ready_transitions_total",
		metric.WithDescription("Count of readiness state transitions"),
	)
	if err == nil {
		s.readyTransitionsTotal = c
	} else {
		log.Error("Failed to create readiness transition counter", "error", err)
	}
}

// onReadinessMaybeChanged increments transition counter when overall readiness flips.
func (s *Server) onReadinessMaybeChanged(reason string) {
	now := s.isFullyReady()
	s.readinessMu.Lock()
	changed := (now != s.lastReady)
	s.lastReady = now
	s.readinessMu.Unlock()
	if changed && s.monitoring != nil && s.monitoring.IsInitialized() && s.readyTransitionsTotal != nil {
		to := statusNotReady
		if now {
			to = statusReady
		}
		s.readyTransitionsTotal.Add(
			s.ctx,
			1,
			metric.WithAttributes(
				attribute.String("component", "server"),
				attribute.String("to", to),
				attribute.String("reason", reason),
				attribute.String("mode", s.modeLabel),
				attribute.String("store_driver", s.storeDriverLabel),
				attribute.String("cache_driver", s.cacheDriverLabel),
				attribute.String("auth_repo_driver", s.authRepoDriverLabel),
				attribute.String("auth_cache_driver", s.authCacheDriverLabel),
			),
		)
	}
}

// isFullyReady computes aggregate readiness for the server.
func (s *Server) isFullyReady() bool {
	return s.isTemporalReady() && s.isWorkerReady() && s.isMCPReady() && s.IsReconciliationReady()
}

// logStartupBanner prints a friendly list of service endpoints and ports.
func (s *Server) logStartupBanner() {
	log := logger.FromContext(s.ctx)
	host := friendlyHost(s.serverConfig.Host)
	httpURL := fmt.Sprintf("http://%s:%d", host, s.serverConfig.Port)
	apiURL := fmt.Sprintf("%s%s", httpURL, routes.Base())
	swaggerURL := fmt.Sprintf("%s/swagger/index.html", httpURL)
	docsURL := fmt.Sprintf("%s/docs/index.html", httpURL)
	hooksURL := fmt.Sprintf("%s%s", httpURL, routes.Hooks())
	mcp := s.mcpBaseURL
	temporalHP := ""
	if cfg := config.FromContext(s.ctx); cfg != nil {
		temporalHP = cfg.Temporal.HostPort
	}
	uiURL := s.temporalUIURL(host)
	ver := version.Get().Version
	lines := []string{
		fmt.Sprintf("Compozy %s", ver),
		fmt.Sprintf("  API           > %s", apiURL),
		fmt.Sprintf("  Health        > %s/health", httpURL),
		fmt.Sprintf("  Readyz        > %s/readyz", httpURL),
		fmt.Sprintf("  Swagger       > %s", swaggerURL),
		fmt.Sprintf("  Docs          > %s", docsURL),
		fmt.Sprintf("  Webhooks      > %s", hooksURL),
	}
	if mcp != "" {
		lines = append(lines,
			fmt.Sprintf("  MCP Proxy     > %s", mcp),
			fmt.Sprintf("  MCP Admin     > %s/admin/mcps", mcp),
		)
	}
	if temporalHP != "" {
		lines = append(lines, fmt.Sprintf("  Temporal gRPC > %s", temporalHP))
	}
	if uiURL != "" {
		lines = append(lines, fmt.Sprintf("  Temporal UI   > %s", uiURL))
	}
	banner := "\n" + strings.Join(lines, "\n")
	log.Info(banner)
}

func friendlyHost(h string) string {
	if h == hostAny || h == "::" || h == "" {
		return hostLoopback
	}
	return h
}

func (s *Server) temporalUIURL(defaultHost string) string {
	if s.embeddedTemporal == nil {
		return ""
	}
	type uiGetter interface{ UIPort() int }
	type hpGetter interface{ HostPort() string }
	ug, ok := s.embeddedTemporal.(uiGetter)
	if !ok {
		return ""
	}
	uip := ug.UIPort()
	if uip <= 0 {
		return ""
	}
	thost := defaultHost
	if hg, ok := s.embeddedTemporal.(hpGetter); ok {
		hp := hg.HostPort()
		if h, _, err := net.SplitHostPort(hp); err == nil && h != "" {
			thost = h
		}
	}
	thost = friendlyHost(thost)
	return fmt.Sprintf("http://%s:%d", thost, uip)
}

func (s *Server) startAndRunServer(cleanupFuncs []func()) error {
	srv := s.createHTTPServer()
	s.httpServer = srv
	// Start server in goroutine with error channel to catch immediate failures
	errChan := make(chan error, 1)
	go s.startServer(srv, errChan)
	select {
	case err := <-errChan:
		if err != nil {
			s.cleanup(cleanupFuncs)
			return err
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		s.logStartupBanner()
	}
	// Wait for shutdown signal, server failure, or programmatic shutdown
	return s.handleGracefulShutdown(srv, cleanupFuncs, errChan)
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

func (s *Server) startServer(srv *http.Server, errChan chan<- error) {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log := logger.FromContext(s.ctx)
		log.Error("HTTP server failed", "error", err)
		errChan <- fmt.Errorf("HTTP server failed: %w", err)
		return
	}
	errChan <- nil
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

func (s *Server) handleGracefulShutdown(srv *http.Server, cleanupFuncs []func(), errChan <-chan error) error {
	log := logger.FromContext(s.ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		log.Debug("Received shutdown signal, initiating graceful shutdown")
	case <-s.shutdownChan:
		log.Debug("Received programmatic shutdown signal, initiating graceful shutdown")
	case err := <-errChan:
		if err != nil {
			log.Error("Server reported failure, shutting down", "error", err)
			s.cleanup(cleanupFuncs)
			s.cancel()
			return err
		}
		log.Debug("HTTP server closed, proceeding with shutdown")
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

// isHostPortReachable performs a best-effort TCP dial to host:port within the given timeout.
// It avoids importing Temporal client just to check if a server is up.
func isHostPortReachable(ctx context.Context, hostPort string, timeout time.Duration) bool {
	d := net.Dialer{}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := d.DialContext(cctx, "tcp", hostPort)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		select {
		case s.shutdownChan <- struct{}{}:
		default:
		}
	})
}

// GetReconciliationStatus returns the current reconciliation status
func (s *Server) GetReconciliationStatus() (bool, time.Time, int, error) {
	return s.reconciliationState.getStatus()
}

// IsReconciliationReady returns whether the initial reconciliation has completed
func (s *Server) IsReconciliationReady() bool {
	return s.reconciliationState.isReady()
}

// temporal readiness helpers
//

func (s *Server) setTemporalReady(v bool) {
	s.readinessMu.Lock()
	s.temporalReady = v
	s.readinessMu.Unlock()
}

func (s *Server) setWorkerReady(v bool) {
	s.readinessMu.Lock()
	s.workerReady = v
	s.readinessMu.Unlock()
}
func (s *Server) isTemporalReady() bool {
	s.readinessMu.RLock()
	defer s.readinessMu.RUnlock()
	return s.temporalReady
}
func (s *Server) isWorkerReady() bool {
	s.readinessMu.RLock()
	defer s.readinessMu.RUnlock()
	return s.workerReady
}

// mcp readiness helpers
//

func (s *Server) setMCPReady(v bool) {
	s.mcpMu.Lock()
	s.mcpReady = v
	s.mcpMu.Unlock()
}
func (s *Server) isMCPReady() bool {
	s.mcpMu.RLock()
	defer s.mcpMu.RUnlock()
	return s.mcpReady
}

// setupMCPProxy boots the embedded MCP proxy when running in standalone mode.
// It starts the proxy in a goroutine, waits for health readiness, and returns a cleanup.
// Configuration is sourced from config.FromContext(ctx). It sets cfg.LLM.ProxyURL
// when empty so downstream components (worker, LLM) can reach the proxy.
func (s *Server) setupMCPProxy(ctx context.Context) (func(), error) {
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}
	if cfg.Mode != modeStandalone {
		return func() {}, nil
	}
	host, portStr := normalizeMCPHostAndPort(cfg)
	// Build storage + server via helper
	initialBase := initialMCPBaseURL(host, portStr, cfg.MCPProxy.BaseURL)
	server, driver, err := s.newMCPProxyServer(ctx, cfg.Mode, host, portStr, initialBase, cfg.MCPProxy.ShutdownTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP proxy: %w", err)
	}
	// Start proxy in background
	go func() {
		if err := server.Start(ctx); err != nil {
			logger.FromContext(ctx).Error("Embedded MCP proxy exited with error", "error", err)
		}
	}()
	// Build HTTP client and timeouts
	total := mcpProbeTimeout(cfg)
	client := &http.Client{Timeout: mcpHealthRequestTimeout}
	// Wait until the proxy bound its listener so BaseURL reflects the actual port
	bctx, bcancel := context.WithTimeout(ctx, total)
	select {
	case <-server.Bound():
	case <-bctx.Done():
		bcancel()
		// Use WithoutCancel so the proxy can still stop even if boot context was canceled
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		if stopErr := server.Stop(ctx2); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after bind timeout", "error", stopErr)
		}
		cancel()
		return nil, fmt.Errorf("embedded MCP proxy did not bind within timeout")
	}
	bcancel()
	// Use the server's effective BaseURL (handles :0) for readiness polling
	baseURL := server.BaseURL()
	s.mcpBaseURL = baseURL
	ready := s.awaitMCPProxyReady(ctx, client, baseURL, total)
	if !ready {
		// Ensure proxy is stopped to avoid leaks
		// Use WithoutCancel so shutdown isn't aborted by the parent cancel
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		if stopErr := server.Stop(ctx2); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after readiness failure", "error", stopErr)
		}
		cancel()
		return nil, fmt.Errorf("embedded MCP proxy failed readiness within timeout: %s", baseURL)
	}
	// Mark readiness, set proxy URL if needed, and log
	s.afterMCPReady(ctx, cfg, baseURL, driver)
	// Cleanup wiring
	cleanup := func() {
		// Use WithoutCancel to guarantee best-effort graceful shutdown on server exit
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		defer cancel()
		if err := server.Stop(ctx2); err != nil {
			log.Warn("Failed to stop embedded MCP proxy", "error", err)
		}
		s.setMCPReady(false)
		s.onReadinessMaybeChanged("mcp_stopped")
	}
	s.mcpProxy = server
	return cleanup, nil
}

// awaitMCPProxyReady polls the MCP proxy /healthz until ready or timeout.
func (s *Server) awaitMCPProxyReady(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	total time.Duration,
) bool {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		rctx, cancel := context.WithTimeout(ctx, mcpHealthRequestTimeout)
		req, reqErr := http.NewRequestWithContext(rctx, http.MethodGet, baseURL+"/healthz", http.NoBody)
		if reqErr != nil {
			cancel()
			time.Sleep(mcpHealthPollInterval)
			continue
		}
		resp, err := client.Do(req)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			cancel()
			return true
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		cancel()
		time.Sleep(mcpHealthPollInterval)
	}
	return false
}

// afterMCPReady marks MCP readiness, sets ProxyURL if empty, and logs startup.
func (s *Server) afterMCPReady(ctx context.Context, cfg *config.Config, baseURL, driver string) {
	s.setMCPReady(true)
	s.onReadinessMaybeChanged("mcp_ready")
	if cfg.LLM.ProxyURL == "" {
		cfg.LLM.ProxyURL = baseURL
		logger.FromContext(ctx).Info("Set LLM proxy URL from embedded MCP proxy", "proxy_url", baseURL)
	}
	logger.FromContext(ctx).Info(
		"Embedded MCP proxy started",
		"mode", cfg.Mode,
		"mcp_storage_driver", driver,
		"base_url", baseURL,
	)
}

// normalizeMCPHostAndPort resolves host and port string, using 127.0.0.1 and :0 defaults.
func normalizeMCPHostAndPort(cfg *config.Config) (string, string) {
	host := cfg.MCPProxy.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if cfg.MCPProxy.Port <= 0 {
		return host, "0"
	}
	return host, fmt.Sprintf("%d", cfg.MCPProxy.Port)
}

// initialMCPBaseURL computes a base URL prior to binding; for :0 it is a placeholder.
func initialMCPBaseURL(host, portStr, cfgBase string) string {
	if cfgBase != "" {
		return cfgBase
	}
	bhost := host
	if host == "0.0.0.0" || host == "::" {
		bhost = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", bhost, portStr)
}

// mcpProbeTimeout returns the configured health check timeout with a sane default.
func mcpProbeTimeout(cfg *config.Config) time.Duration {
	if cfg.Worker.MCPProxyHealthCheckTimeout <= 0 {
		return 10 * time.Second
	}
	return cfg.Worker.MCPProxyHealthCheckTimeout
}

// newMCPProxyServer constructs the embedded MCP proxy server and returns it with a driver label.
func (s *Server) newMCPProxyServer(
	ctx context.Context,
	mode string,
	host string,
	port string,
	baseURL string,
	shutdown time.Duration,
) (*mcpproxy.Server, string, error) {
	stCfg := mcpproxy.DefaultStorageConfigForMode(mode)
	storage, err := mcpproxy.NewStorage(ctx, stCfg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize MCP storage: %w", err)
	}
	cm := mcpproxy.NewMCPClientManager(storage, nil)
	// Disable OS signal handling for embedded proxy; parent server owns signals
	mcfg := &mcpproxy.Config{
		Host:               host,
		Port:               port,
		BaseURL:            baseURL,
		ShutdownTimeout:    shutdown,
		UseOSSignalHandler: false,
	}
	server := mcpproxy.NewServer(mcfg, storage, cm)
	return server, string(stCfg.Type), nil
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
				s.onReadinessMaybeChanged("schedules_reconciled")
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
