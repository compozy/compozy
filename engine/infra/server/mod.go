package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Config *Config
	router *gin.Engine
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(config Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Config: &config,
		ctx:    ctx,
		cancel: cancel,
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
	// Load project and workspace files
	projectConfig, workflows, err := loadProject(s.Config.CWD, s.Config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Setup NATS server
	cwd := projectConfig.GetCWD()
	ns, nc, err := setupNats(s.ctx, cwd)
	if err != nil {
		return fmt.Errorf("failed to setup NATS server: %w", err)
	}
	defer func() {
		if err := ns.Shutdown(); err != nil {
			logger.Error("Error shutting down NATS server", "error", err)
		}
	}()

	// Load store
	dataDir := filepath.Join(core.GetStoreDir(cwd), "data")
	dbFilePath := filepath.Join(dataDir, "compozy.db")
	st, err := store.NewStore(dbFilePath)
	if err != nil {
		return fmt.Errorf("failed to create state store: %w", err)
	}
	if err := st.Setup(); err != nil {
		return fmt.Errorf("failed to setup store: %w", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("Error shutting down store", "error", err)
		}
	}()

	// Setup orchestrator
	deps := appstate.NewBaseDeps(ns, nc, st, projectConfig, workflows)
	orch, err := setupOrchestrator(s.ctx, deps)
	if err != nil {
		return err
	}
	state, err := appstate.NewState(deps, orch)
	if err != nil {
		return fmt.Errorf("failed to create app state: %w", err)
	}

	// Build server routes
	if err := s.buildRouter(state); err != nil {
		return fmt.Errorf("failed to build router: %w", err)
	}

	addr := s.Config.FullAddress()
	logger.Info("Starting HTTP server",
		"address", fmt.Sprintf("http://%s", addr),
	)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start",
				"error", err,
			)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Debug("Received shutdown signal, initiating graceful shutdown")

	s.cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logger.Info("Server shutdown completed successfully")
	return nil
}

func setupOrchestrator(ctx context.Context, deps appstate.BaseDeps) (*orchestrator.Orchestrator, error) {
	ns := deps.NatsServer
	nc := deps.NatsClient
	store := deps.Store
	projectConfig := deps.ProjectConfig
	workflows := deps.Workflows

	orchConfig := orchestrator.Config{
		WorkflowRepoFactory: func() workflow.Repository {
			return store.NewWorkflowRepository(projectConfig, workflows)
		},
		TaskRepoFactory: func() task.Repository {
			workflowRepo := store.NewWorkflowRepository(projectConfig, workflows)
			return store.NewTaskRepository(workflowRepo)
		},
		AgentRepoFactory: func() agent.Repository {
			workflowRepo := store.NewWorkflowRepository(projectConfig, workflows)
			taskRepo := store.NewTaskRepository(workflowRepo)
			return store.NewAgentRepository(workflowRepo, taskRepo)
		},
		ToolRepoFactory: func() tool.Repository {
			workflowRepo := store.NewWorkflowRepository(projectConfig, workflows)
			taskRepo := store.NewTaskRepository(workflowRepo)
			return store.NewToolRepository(workflowRepo, taskRepo)
		},
	}
	orch := orchestrator.NewOrchestrator(
		ns,
		nc,
		store,
		orchConfig,
		projectConfig,
		workflows,
	)
	if err := orch.Setup(ctx); err != nil {
		logger.Error("Failed to setup orchestrator", "error", err)
		return nil, fmt.Errorf("failed to setup orchestrator: %w", err)
	}
	return orch, nil
}

func setupNats(ctx context.Context, cwd *core.CWD) (*nats.Server, *nats.Client, error) {
	opts := nats.DefaultServerOptions(cwd)
	opts.EnableJetStream = true
	natsServer, err := nats.NewNatsServer(opts)
	if err != nil {
		logger.Error("Failed to setup NATS server", "error", err)
		return nil, nil, err
	}
	nc, err := nats.NewClient(natsServer.Conn)
	if err != nil {
		logger.Error("Failed to create NATS client", "error", err)
		return nil, nil, err
	}
	if err := nc.SetupStreams(ctx); err != nil {
		logger.Error("Failed to setup NATS streams", "error", err)
		return nil, nil, err
	}
	return natsServer, nc, nil
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

	// Load workflows from sources
	workflows, err := workflow.WorkflowsFromProject(projectConfig)
	if err != nil {
		logger.Error("Failed to load workflows", "error", err)
		return nil, nil, err
	}

	return projectConfig, workflows, nil
}
