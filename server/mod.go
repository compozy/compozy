package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/server/appstate"
	"github.com/compozy/compozy/server/router"
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

func (s *Server) buildRouter(st *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(LoggerMiddleware())
	if s.Config.CORSEnabled {
		r.Use(CORSMiddleware())
	}
	r.Use(appstate.StateMiddleware(st))
	r.Use(router.ErrorHandler())
	if err := RegisterRoutes(r, st); err != nil {
		return err
	}
	s.router = r
	return nil
}

func (s *Server) Run() error {
	// Load project and workspace files
	pjc, wfs, err := loadProject(s.Config.CWD, s.Config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Setup NATS server
	ns, err := setupNatsServer()
	if err != nil {
		return fmt.Errorf("failed to setup NATS server: %w", err)
	}
	defer func() {
		if err := ns.Shutdown(); err != nil {
			logger.Error("Error shutting down NATS server", "error", err)
		}
	}()

	// Get and start services
	orch, store, err := getServices(s.ctx, ns, pjc, wfs)
	if err != nil {
		return fmt.Errorf("failed to load services: %w", err)
	}
	if err := orch.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}
	defer func() {
		if err := orch.Stop(s.ctx); err != nil {
			logger.Error("Error shutting down orchestrator", "error", err)
		}
	}()

	// Create server state
	st, err := appstate.NewState(ns, orch, store, pjc, wfs)
	if err != nil {
		return fmt.Errorf("failed to create app state: %w", err)
	}

	// Build server routes
	if err := s.buildRouter(st); err != nil {
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
