package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Config         *Config
	TemporalConfig *worker.TemporalConfig
	StoreConfig    *store.Config
	router         *gin.Engine
	monitoring     *monitoring.Service
	ctx            context.Context
	cancel         context.CancelFunc
	httpServer     *http.Server
	shutdownChan   chan struct{}
}

func NewServer(ctx context.Context, config *Config, tConfig *worker.TemporalConfig, sConfig *store.Config) *Server {
	serverCtx, cancel := context.WithCancel(ctx)
	return &Server{
		Config:         config,
		TemporalConfig: tConfig,
		StoreConfig:    sConfig,
		ctx:            serverCtx,
		cancel:         cancel,
		shutdownChan:   make(chan struct{}, 1),
	}
}

func (s *Server) buildRouter(state *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())
	// Add monitoring middleware BEFORE other middleware if monitoring is initialized
	log := logger.FromContext(s.ctx)
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		r.Use(s.monitoring.GinMiddleware(s.ctx))
	}
	r.Use(LoggerMiddleware(log))
	if s.Config.CORSEnabled {
		r.Use(CORSMiddleware())
	}
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
	// Register metrics endpoint (not versioned under /api/v0/)
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		monitoringPath := state.ProjectConfig.MonitoringConfig.Path
		r.GET(monitoringPath, gin.WrapH(s.monitoring.ExporterHandler()))
	}
	if err := RegisterRoutes(s.ctx, r, state); err != nil {
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

func (s *Server) setupDependencies() (*appstate.State, []func(), error) {
	var cleanupFuncs []func()
	// Load project and workspace files
	log := logger.FromContext(s.ctx)
	setupStart := time.Now()
	configService := csvc.NewService(log, s.Config.EnvFilePath)
	projectConfig, workflows, err := configService.LoadProject(s.ctx, s.Config.CWD, s.Config.ConfigFile)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to load project: %w", err)
	}
	log.Debug("Loaded project configuration", "duration", time.Since(setupStart))
	// Initialize monitoring service with shorter timeout to prevent slow startup
	monitoringStart := time.Now()
	monitoringCtx, monitoringCancel := context.WithTimeout(s.ctx, 500*time.Millisecond)
	monitoringService, err := monitoring.NewMonitoringService(monitoringCtx, projectConfig.MonitoringConfig)
	monitoringDuration := time.Since(monitoringStart)
	if err != nil {
		// Cancel the context only on error
		monitoringCancel()
		if err == context.DeadlineExceeded {
			log.Warn("Monitoring initialization timed out, continuing without monitoring",
				"duration", monitoringDuration)
		} else {
			log.Error("Failed to initialize monitoring service", "error", err,
				"duration", monitoringDuration)
		}
		// Continue with nil monitoring service
		s.monitoring = nil
	} else {
		s.monitoring = monitoringService
		if monitoringService.IsInitialized() {
			log.Info("Monitoring service initialized successfully",
				"enabled", projectConfig.MonitoringConfig.Enabled,
				"path", projectConfig.MonitoringConfig.Path,
				"duration", monitoringDuration)
			// Add both monitoring shutdown and context cancellation to cleanup
			cleanupFuncs = append(cleanupFuncs, func() {
				// Cancel the monitoring context during cleanup
				monitoringCancel()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := monitoringService.Shutdown(ctx); err != nil {
					log.Error("Failed to shutdown monitoring service", "error", err)
				}
			})
		} else {
			// Cancel context if monitoring is disabled
			monitoringCancel()
			log.Info("Monitoring is disabled in the configuration",
				"duration", monitoringDuration)
		}
	}
	dbStart := time.Now()
	// Setup database store
	storeStart := time.Now()
	store, err := store.SetupStore(s.ctx, s.StoreConfig)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to setup store: %w", err)
	}
	log.Info("Database store initialized", "duration", time.Since(storeStart))
	log.Debug("Database connection established", "duration", time.Since(dbStart))
	cleanupFuncs = append(cleanupFuncs, func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		store.DB.Close(ctx)
	})
	clientConfig := s.TemporalConfig
	deps := appstate.NewBaseDeps(projectConfig, workflows, store, clientConfig)
	workerStart := time.Now()
	worker, err := setupWorker(s.ctx, deps, s.monitoring)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	log.Debug("Worker setup completed", "duration", time.Since(workerStart))
	cleanupFuncs = append(cleanupFuncs, func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		worker.Stop(ctx)
	})
	state, err := appstate.NewState(deps, worker)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to create app state: %w", err)
	}
	log.Info("Server dependencies setup completed", "total_duration", time.Since(setupStart))
	return state, cleanupFuncs, nil
}

func setupWorker(
	ctx context.Context,
	deps appstate.BaseDeps,
	monitoringService *monitoring.Service,
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
	addr := s.Config.FullAddress()
	log := logger.FromContext(s.ctx)
	log.Info("Starting HTTP server",
		"address", fmt.Sprintf("http://%s", addr),
	)
	return &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (s *Server) startServer(srv *http.Server) {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log := logger.FromContext(s.ctx)
		log.Error("Server failed to start",
			"error", err,
		)
		os.Exit(1)
	}
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
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
