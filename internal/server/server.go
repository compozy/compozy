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

	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/gin-gonic/gin"
)

// Context keys
type contextKey string

const (
	appStateKey contextKey = "app_state"
)

// AppState contains the state shared across the server
type AppState struct {
	CWD       string
	Workflows []*workflow.WorkflowConfig
}

// NewAppState creates a new AppState
func NewAppState(cwd string, workflows []*workflow.WorkflowConfig) (*AppState, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, NewServerError(ErrInternalCode, "Failed to get current working directory")
		}
	}

	if !filepath.IsAbs(cwd) {
		absPath, err := filepath.Abs(cwd)
		if err != nil {
			return nil, NewServerError(ErrInternalCode, "Failed to resolve absolute path")
		}
		cwd = absPath
	}

	if workflows == nil {
		workflows = []*workflow.WorkflowConfig{}
	}

	return &AppState{
		CWD:       cwd,
		Workflows: workflows,
	}, nil
}

// WithAppState adds the app state to the context
func WithAppState(ctx context.Context, state *AppState) context.Context {
	return context.WithValue(ctx, appStateKey, state)
}

// GetAppState retrieves the app state from the context
func GetAppState(ctx context.Context) (*AppState, error) {
	state, ok := ctx.Value(appStateKey).(*AppState)
	if !ok {
		return nil, NewServerError(ErrInternalCode, "App state not found in context")
	}
	return state, nil
}

// Server represents the HTTP server
type Server struct {
	Config *ServerConfig
	State  *AppState
	router *gin.Engine
}

// NewServer creates a new server instance
func NewServer(config *ServerConfig, state *AppState) *Server {
	if config == nil {
		config = &ServerConfig{
			CWD:         state.CWD,
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

	// Add health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

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
