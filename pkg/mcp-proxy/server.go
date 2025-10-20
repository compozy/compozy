package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/version"
	"github.com/gin-gonic/gin"
)

// Server represents the MCP proxy server
type Server struct {
	Router        *gin.Engine
	httpServer    *http.Server
	config        *Config
	storage       Storage
	clientManager ClientManager
	adminHandlers *AdminHandlers
	proxyHandlers *ProxyHandlers
	ln            net.Listener
	boundCh       chan struct{}
	boundOnce     sync.Once
}

// Config holds server configuration
type Config struct {
	Port            string
	Host            string
	BaseURL         string // Base URL for SSE server
	ShutdownTimeout time.Duration
	// UseOSSignalHandler controls whether the server installs its own
	// OS signal handler and blocks awaiting SIGINT/SIGTERM.
	// When running embedded inside another server, set this to false
	// so shutdown is driven by the parent context only.
	UseOSSignalHandler bool
}

// Validate validates the server configuration
func (c *Config) Validate() error {
	if c.Port == "" {
		return errors.New("port is required")
	}
	if c.Host == "" {
		return errors.New("host is required")
	}
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}
	return nil
}

// NewServer creates a new MCP proxy server instance
func NewServer(config *Config, storage Storage, clientManager ClientManager) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Add logging middleware
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			p := param.Path
			if param.Request != nil && param.Request.URL != nil {
				p = param.Request.URL.EscapedPath()
			}
			return fmt.Sprintf("[%s] %s %s %d %s\n",
				param.TimeStamp.Format("2006-01-02 15:04:05"),
				param.Method,
				p,
				param.StatusCode,
				param.Latency,
			)
		},
	}))

	router.Use(gin.Recovery())
	proxyHandlers := NewProxyHandlers(storage, clientManager, config.BaseURL)
	service := NewMCPService(storage, clientManager, proxyHandlers)
	adminHandlers := NewAdminHandlers(service)
	server := &Server{
		Router:        router,
		config:        config,
		storage:       storage,
		clientManager: clientManager,
		adminHandlers: adminHandlers,
		proxyHandlers: proxyHandlers,
		httpServer: &http.Server{
			Addr:         config.Host + ":" + config.Port,
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		boundCh: make(chan struct{}),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all server routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.Router.GET("/healthz", s.healthzHandler)

	// Admin API for MCP management (no IP-based filtering; rely on external network controls)
	admin := s.Router.Group("/admin")
	{
		// MCP Definition CRUD operations
		admin.POST("/mcps", s.adminHandlers.AddMCPHandler)
		admin.PUT("/mcps/:name", s.adminHandlers.UpdateMCPHandler)
		admin.DELETE("/mcps/:name", s.adminHandlers.RemoveMCPHandler)
		admin.GET("/mcps", s.adminHandlers.ListMCPsHandler)
		admin.GET("/mcps/:name", s.adminHandlers.GetMCPHandler)

		// Tools discovery endpoint
		admin.GET("/tools", s.adminHandlers.ListToolsHandler)

		// Tool execution endpoint
		admin.POST("/tools/call", s.adminHandlers.CallToolHandler)

		// Metrics endpoint
		admin.GET("/metrics", s.metricsHandler)
	}

	// MCP Proxy endpoints - direct routes for each transport type
	{
		// SSE transport proxy - GET only (SSE semantics)
		s.Router.GET("/mcp-proxy/:name/sse", s.proxyHandlers.SSEProxyHandler)
		s.Router.GET("/mcp-proxy/:name/sse/*path", s.proxyHandlers.SSEProxyHandler)

		// Streamable HTTP transport proxy
		s.Router.Any("/mcp-proxy/:name/stream", s.proxyHandlers.StreamableHTTPProxyHandler)
		s.Router.Any("/mcp-proxy/:name/stream/*path", s.proxyHandlers.StreamableHTTPProxyHandler)
	}

	// API versioning for legacy compatibility
	v1 := s.Router.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})
	}
}

// healthzHandler handles health check requests
func (s *Server) healthzHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   version.Get().Version,
	})
}

// metricsHandler handles metrics requests
func (s *Server) metricsHandler(c *gin.Context) {
	metrics := s.clientManager.GetMetrics()

	c.JSON(http.StatusOK, gin.H{
		"timestamp": time.Now().UTC(),
		"metrics":   metrics,
	})
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Starting MCP proxy server", "port", s.config.Port, "host", s.config.Host)
	log.Info("MCP proxy has no built-in IP filtering; secure /admin endpoints via network controls")
	s.httpServer.BaseContext = func(_ net.Listener) context.Context { return ctx }

	ln, err := s.bindListener(ctx)
	if err != nil {
		return err
	}
	s.boundOnce.Do(func() { close(s.boundCh) })

	if err := s.clientManager.Start(ctx); err != nil {
		_ = ln.Close()
		return fmt.Errorf("failed to start client manager: %w", err)
	}

	errChan := s.launchHTTPServer(ln)
	if err := s.awaitStartup(ctx, errChan); err != nil {
		s.stopClientManagerSilently(ctx)
		return err
	}

	log.Info("MCP proxy server started successfully")
	return s.waitForShutdown(ctx, errChan)
}

// bindListener creates the TCP listener and configures derived settings.
func (s *Server) bindListener(ctx context.Context) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.config.Host+":"+s.config.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to bind listener: %w", err)
	}
	s.ln = ln
	s.configureBaseURL(ln)
	return ln, nil
}

// configureBaseURL computes the effective BaseURL when binding completes.
func (s *Server) configureBaseURL(ln net.Listener) {
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		hostForURL := s.config.Host
		if hostForURL == "0.0.0.0" || hostForURL == "::" {
			hostForURL = "127.0.0.1"
		}
		if s.config.BaseURL == "" || s.config.Port == "0" {
			s.config.BaseURL = fmt.Sprintf("http://%s:%d", hostForURL, tcp.Port)
			s.proxyHandlers.SetBaseURL(s.config.BaseURL)
		}
	}
}

// launchHTTPServer runs the HTTP server in a goroutine and returns its error channel.
func (s *Server) launchHTTPServer(ln net.Listener) chan error {
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server failed: %w", err)
		} else {
			errChan <- nil
		}
	}()
	return errChan
}

// awaitStartup waits for the HTTP server to start or fail immediately.
func (s *Server) awaitStartup(ctx context.Context, errChan <-chan error) error {
	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stopClientManagerSilently stops the client manager while logging failures.
func (s *Server) stopClientManagerSilently(ctx context.Context) {
	if err := s.clientManager.Stop(ctx); err != nil {
		logger.FromContext(ctx).Error("Failed to stop client manager during server startup failure", "error", err)
	}
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Shutting down MCP proxy server")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	// Stop client manager first
	if err := s.clientManager.Stop(shutdownCtx); err != nil {
		log.Error("Client manager shutdown failed", "error", err)
	}

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown failed", "error", err)
		return err
	}

	log.Info("MCP proxy server stopped gracefully")
	return nil
}

// waitForShutdown waits for shutdown signals and handles graceful shutdown
func (s *Server) waitForShutdown(ctx context.Context, errChan <-chan error) error {
	log := logger.FromContext(ctx)
	// When embedded, do not install an OS signal handler; rely on ctx.
	if !s.config.UseOSSignalHandler {
		select {
		case <-ctx.Done():
			log.Debug("Context canceled, shutting down server")
			return s.Stop(ctx)
		case err := <-errChan:
			if err != nil {
				log.Error("HTTP server failed", "error", err)
				if stopErr := s.Stop(ctx); stopErr != nil {
					log.Error("Failed to stop server after HTTP failure", "error", stopErr)
				}
				return err
			}
			return s.Stop(ctx)
		}
	}

	// Standalone mode: install OS signal handler
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case <-ctx.Done():
		log.Debug("Context canceled, shutting down server")
		return s.Stop(ctx)
	case sig := <-quit:
		log.Info("Received shutdown signal", "signal", sig.String())
		return s.Stop(ctx)
	case err := <-errChan:
		if err != nil {
			log.Error("HTTP server failed", "error", err)
			if stopErr := s.Stop(ctx); stopErr != nil {
				log.Error("Failed to stop server after HTTP failure", "error", stopErr)
			}
			return err
		}
		return s.Stop(ctx)
	}
}

// BaseURL returns the effective base URL after the server has bound.
// When the server is configured with port "0" or BaseURL was empty,
// this reflects the computed URL using the bound ephemeral port.
func (s *Server) BaseURL() string {
	return s.config.BaseURL
}

// Bound returns a channel that is closed once the server listener is bound
// and BaseURL is populated (useful when port "0" is requested).
func (s *Server) Bound() <-chan struct{} {
	return s.boundCh
}
