package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Config         *Config
	TemporalConfig *worker.TemporalConfig
	StoreConfig    *store.Config
	router         *gin.Engine
	ctx            context.Context
	cancel         context.CancelFunc
	httpServer     *http.Server
	shutdownChan   chan struct{}
}

func NewServer(config Config, tConfig *worker.TemporalConfig, sConfig *store.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Config:         &config,
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
	r.Use(LoggerMiddleware())
	if s.Config.CORSEnabled {
		r.Use(CORSMiddleware())
	}
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
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
	projectConfig, workflows, err := loadProject(s.Config.CWD, s.Config.ConfigFile)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to load project: %w", err)
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
	worker, err := setupWorker(s.ctx, deps)
	if err != nil {
		return nil, cleanupFuncs, err
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		worker.Stop()
	})

	state, err := appstate.NewState(deps, worker)
	if err != nil {
		return nil, cleanupFuncs, fmt.Errorf("failed to create app state: %w", err)
	}

	return state, cleanupFuncs, nil
}

func loadProject(cwd string, file string) (*project.Config, []*workflow.Config, error) {
	pCWD, err := core.CWDFromPath(cwd)
	if err != nil {
		return nil, nil, err
	}
	logger.Info("Starting compozy server")
	logger.Debug("Loading config file", "config_file", file)

	projectConfig, err := project.Load(pCWD, file)
	if err != nil {
		logger.Error("Failed to load project config", "error", err)
		return nil, nil, err
	}

	if err := projectConfig.Validate(); err != nil {
		logger.Error("Invalid project config", "error", err)
		return nil, nil, err
	}
	globalScope, err := projectConfig.AsMap()
	if err != nil {
		logger.Error("Failed to convert project config to map", "error", err)
		return nil, nil, err
	}

	// Load workflows from sources
	ev := ref.NewEvaluator(
		ref.WithGlobalScope(globalScope),
		ref.WithCacheEnabled(),
	)
	workflows, err := workflow.WorkflowsFromProject(projectConfig, ev)
	if err != nil {
		logger.Error("Failed to load workflows", "error", err)
		return nil, nil, err
	}

	return projectConfig, workflows, nil
}

func setupWorker(ctx context.Context, deps appstate.BaseDeps) (*worker.Worker, error) {
	workerConfig := &worker.Config{
		WorkflowRepo: func() workflow.Repository {
			return deps.Store.NewWorkflowRepo()
		},
		TaskRepo: func() task.Repository {
			return deps.Store.NewTaskRepo()
		},
	}
	worker, err := worker.NewWorker(
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
