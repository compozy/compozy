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
	Config *Config
	State  *AppState
	router *gin.Engine
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

	return &Server{
		Config: config,
		State:  state,
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

// Run starts the HTTP server
func (s *Server) Run() error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logger.Info("Server shutdown completed successfully")
	return nil
}
