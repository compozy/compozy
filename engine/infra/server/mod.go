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

func NewServer(config *Config, tConfig *worker.TemporalConfig, sConfig *store.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Config:         config,
		TemporalConfig: tConfig,
		StoreConfig:    sConfig,
		ctx:            ctx,
		cancel:         cancel,
		shutdownChan:   make(chan struct{}, 1),
	}
}

func (s *Server) buildRouter(state *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())
	// Add monitoring middleware BEFORE other middleware if monitoring is initialized
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		r.Use(s.monitoring.GinMiddleware())
	}
	r.Use(LoggerMiddleware())
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
	if err := RegisterRoutes(r, state); err != nil {
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
	log := logger.NewLogger(nil)
	configService := csvc.NewService(log, s.Config.EnvFilePath)
	projectConfig, workflows, err := configService.LoadProject(s.Config.CWD, s.Config.ConfigFile)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to load project: %w", err)
	}
	// Initialize monitoring service based on project config
	monitoringService, err := monitoring.NewMonitoringService(projectConfig.MonitoringConfig)
	if err != nil {
		// Log but don't fail - monitoring is not critical
		logger.Error("Failed to initialize monitoring service", "error", err)
		// Continue with nil monitoring service
		s.monitoring = nil
	} else {
		s.monitoring = monitoringService
		if monitoringService.IsInitialized() {
			logger.Info("Monitoring service initialized successfully",
				"enabled", projectConfig.MonitoringConfig.Enabled,
				"path", projectConfig.MonitoringConfig.Path)
			cleanupFuncs = append(cleanupFuncs, func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := monitoringService.Shutdown(ctx); err != nil {
					logger.Error("Failed to shutdown monitoring service", "error", err)
				}
			})
		} else {
			logger.Info("Monitoring is disabled in the configuration")
		}
	}
	store, err := store.SetupStore(s.ctx, s.StoreConfig)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to setup store: %w", err)
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		store.DB.Close()
	})
	clientConfig := s.TemporalConfig
	deps := appstate.NewBaseDeps(projectConfig, workflows, store, clientConfig)
	worker, err := setupWorker(s.ctx, deps, s.monitoring)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		worker.Stop(ctx)
	})
	state, err := appstate.NewState(deps, worker)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to create app state: %w", err)
	}

	return state, cleanupFuncs, nil
}

func setupWorker(
	ctx context.Context,
	deps appstate.BaseDeps,
	monitoringService *monitoring.Service,
) (*worker.Worker, error) {
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
		logger.Error("Failed to create worker", "error", err)
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}
	if err := worker.Setup(ctx); err != nil {
		logger.Error("Failed to setup worker", "error", err)
		return nil, fmt.Errorf("failed to setup worker: %w", err)
	}
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
	logger.Info("Starting HTTP server",
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
		logger.Error("Server failed to start",
			"error", err,
		)
		os.Exit(1)
	}
}

func (s *Server) handleGracefulShutdown(srv *http.Server, cleanupFuncs []func()) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		logger.Debug("Received shutdown signal, initiating graceful shutdown")
	case <-s.shutdownChan:
		logger.Debug("Received programmatic shutdown signal, initiating graceful shutdown")
	}
	// Clean up dependencies first
	s.cleanup(cleanupFuncs)
	s.cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	logger.Info("Server shutdown completed successfully")
	return nil
}

func (s *Server) Shutdown() {
	s.shutdownChan <- struct{}{}
}
