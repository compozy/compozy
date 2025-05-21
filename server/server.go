package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/pkg/app"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Config *Config
	State  *app.State
	router *gin.Engine
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(config *Config, state *app.State) *Server {
	if config == nil {
		config = &Config{
			CWD:         state.CWD.PathStr(),
			Host:        "0.0.0.0",
			Port:        3000,
			CORSEnabled: true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Config: config,
		State:  state,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) buildRouter() error {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware())

	if s.Config.CORSEnabled {
		router.Use(CORSMiddleware())
	}

	router.Use(app.StateMiddleware(s.State))
	if err := RegisterRoutes(router, s.State); err != nil {
		return err
	}

	s.router = router
	return nil
}

func (s *Server) Run() error {
	orch, err := orchestrator.NewOrchestartor(
		s.State.NatsServer,
		s.State.ProjectConfig,
		s.State.Workflows,
	)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	if err := orch.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}
	s.State.Orchestrator = orch
	defer func() {
		if err := orch.Stop(context.Background()); err != nil {
			logger.Error("Error shutting down orchestrator", "error", err)
		}
	}()

	if err := s.buildRouter(); err != nil {
		return err
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
