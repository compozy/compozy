package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
}

// Config holds server configuration
type Config struct {
	Port            string
	Host            string
	BaseURL         string // Base URL for SSE server
	ShutdownTimeout time.Duration

	// Admin API security
	AdminAllowIPs []string // List of allowed IP addresses/CIDR blocks for admin API

	// Trusted proxies for X-Forwarded-For header validation
	TrustedProxies []string // List of trusted proxy IP addresses/CIDR blocks
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
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all server routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.Router.GET("/healthz", s.healthzHandler)

	// Admin API for MCP management with IP-based security middleware
	admin := s.Router.Group("/admin")
	admin.Use(s.adminSecurityMiddleware())
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
		// SSE transport proxy - all methods for SSE endpoint
		s.Router.Any("/:name/sse", s.proxyHandlers.SSEProxyHandler)
		s.Router.Any("/:name/sse/*path", s.proxyHandlers.SSEProxyHandler)

		// Streamable HTTP transport proxy
		s.Router.Any("/:name/stream", s.proxyHandlers.StreamableHTTPProxyHandler)
		s.Router.Any("/:name/stream/*path", s.proxyHandlers.StreamableHTTPProxyHandler)
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

	// Security check: prevent default admin token in production
	if err := s.validateSecurityConfig(ctx); err != nil {
		return fmt.Errorf("security configuration error: %w", err)
	}

	// Start client manager to restore existing MCP connections
	if err := s.clientManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client manager: %w", err)
	}

	// Start server in a goroutine with error channel
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server failed: %w", err)
		} else {
			errChan <- nil
		}
	}()

	// Give server time to start and check for immediate failures
	select {
	case err := <-errChan:
		if err != nil {
			if stopErr := s.clientManager.Stop(ctx); stopErr != nil {
				log.Error("Failed to stop client manager during server startup failure", "error", stopErr)
			}
			return err
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Info("MCP proxy server started successfully")

	// Wait for shutdown signal or HTTP server failure
	return s.waitForShutdown(ctx, errChan)
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
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit) // Clean up signal handler to prevent resource leak

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

// adminSecurityMiddleware implements security checks for admin API
func (s *Server) adminSecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check IP allow-list if configured
		if len(s.config.AdminAllowIPs) > 0 {
			clientIP := s.getClientIP(c)
			if !s.isIPAllowed(clientIP, s.config.AdminAllowIPs) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied: IP not allowed",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// validateSecurityConfig checks security configuration at startup
func (s *Server) validateSecurityConfig(_ context.Context) error {
	// Currently only validates IP configuration if needed
	// Admin API is open by default unless IP restrictions are configured
	return nil
}

// getClientIP extracts the real client IP from the request
func (s *Server) getClientIP(c *gin.Context) string {
	// Extract the direct connection IP first
	directIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		directIP = c.Request.RemoteAddr
	}

	// Only trust X-Forwarded-For and X-Real-IP headers if the request comes from a trusted proxy
	if s.isTrustedProxy(directIP) {
		// Check X-Forwarded-For header first (for reverse proxies)
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP header
		if xri := c.GetHeader("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Fall back to direct connection IP
	return directIP
}

// isIPAllowed checks if an IP is allowed based on the allow list
func (s *Server) isIPAllowed(clientIP string, allowList []string) bool {
	if len(allowList) == 0 {
		return true
	}

	// Parse client IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, allowed := range allowList {
		// Check if it's a CIDR block
		if strings.Contains(allowed, "/") {
			_, cidr, err := net.ParseCIDR(allowed)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return true
			}
		} else {
			// Direct IP comparison
			allowedIP := net.ParseIP(allowed)
			if allowedIP != nil && ip.Equal(allowedIP) {
				return true
			}
		}
	}

	return false
}

// isTrustedProxy checks if an IP is in the trusted proxy list
func (s *Server) isTrustedProxy(clientIP string) bool {
	if len(s.config.TrustedProxies) == 0 {
		return false
	}

	// Parse client IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, trusted := range s.config.TrustedProxies {
		// Check if it's a CIDR block
		if strings.Contains(trusted, "/") {
			_, cidr, err := net.ParseCIDR(trusted)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return true
			}
		} else {
			// Direct IP comparison
			trustedIP := net.ParseIP(trusted)
			if trustedIP != nil && ip.Equal(trustedIP) {
				return true
			}
		}
	}

	return false
}
