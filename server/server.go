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
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	Config           *Config
	State            *AppState
	router           *gin.Engine
	componentManager *ComponentManager
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewServer creates a new server instance
func NewServer(config *Config, state *AppState) *Server {
	if config == nil {
		config = &Config{
			CWD:         state.CWD.PathStr(),
			Host:        "0.0.0.0",
			Port:        3000,
			CORSEnabled: true,
		}
	}

	// Create root context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		Config: config,
		State:  state,
		ctx:    ctx,
		cancel: cancel,
	}
}

// buildRouter builds the Gin router with all registered routes
func (s *Server) buildRouter() error {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware())

	if s.Config.CORSEnabled {
		router.Use(CORSMiddleware())
	}

	// Add app state to context
	router.Use(AppStateMiddleware(s.State))

	if err := RegisterRoutes(router, s.State); err != nil {
		return err
	}

	s.router = router
	return nil
}

// Run starts the HTTP server and all components
func (s *Server) Run() error {
	// Initialize component manager
	componentManager, err := NewComponentManager(s.State)
	if err != nil {
		return fmt.Errorf("failed to initialize component manager: %w", err)
	}
	s.componentManager = componentManager

	// Start all components
	if err := s.componentManager.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start components: %w", err)
	}

	// Build router
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

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start",
				"error", err,
			)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Debug("Received shutdown signal, initiating graceful shutdown")

	// Cancel the context to signal all components to shut down
	s.cancel()

	// Stop all components
	if err := s.componentManager.Stop(); err != nil {
		logger.Error("Error stopping components", "error", err)
	}

	// Shut down the HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logger.Info("Server shutdown completed successfully")
	return nil
}
