package mcpproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/pkg/logger"
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
	AdminTokens   []string // List of valid admin tokens
	AdminAllowIPs []string // List of allowed IP addresses/CIDR blocks for admin API

	// Trusted proxies for X-Forwarded-For header validation
	TrustedProxies []string // List of trusted proxy IP addresses/CIDR blocks

	// Global auth tokens that apply to all MCP clients
	// These tokens are automatically inherited by all MCP clients in addition to their specific auth tokens.
	// Global tokens are checked first, then client-specific tokens.
	// This enables setting common authentication tokens once that work for all clients.
	GlobalAuthTokens []string
}

// NewServer creates a new MCP proxy server instance
func NewServer(config *Config, storage Storage, clientManager ClientManager) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Add logging middleware
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			return fmt.Sprintf("[%s] %s %s %d %s\n",
				param.TimeStamp.Format("2006-01-02 15:04:05"),
				param.Method,
				param.Path,
				param.StatusCode,
				param.Latency,
			)
		},
	}))

	router.Use(gin.Recovery())
	proxyHandlers := NewProxyHandlers(storage, clientManager, config.BaseURL, config.GlobalAuthTokens)
	adminHandlers := NewAdminHandlers(storage, clientManager, proxyHandlers)
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

	// Admin API for MCP management with security middleware
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
		"version":   "1.0.0",
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
	logger.Info("Starting MCP proxy server", "port", s.config.Port, "host", s.config.Host)

	// Security check: prevent default admin token in production
	if err := s.validateSecurityConfig(); err != nil {
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
				logger.Error("Failed to stop client manager during server startup failure", "error", stopErr)
			}
			return err
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	case <-ctx.Done():
		return ctx.Err()
	}

	logger.Info("MCP proxy server started successfully")

	// Wait for shutdown signal
	return s.waitForShutdown(ctx)
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Shutting down MCP proxy server")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	// Stop client manager first
	if err := s.clientManager.Stop(shutdownCtx); err != nil {
		logger.Error("Client manager shutdown failed", "error", err)
	}

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
		return err
	}

	logger.Info("MCP proxy server stopped gracefully")
	return nil
}

// waitForShutdown waits for shutdown signals and handles graceful shutdown
func (s *Server) waitForShutdown(ctx context.Context) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit) // Clean up signal handler to prevent resource leak

	select {
	case <-ctx.Done():
		logger.Info("Context canceled, shutting down server")
		return s.Stop(ctx)
	case sig := <-quit:
		logger.Info("Received shutdown signal", "signal", sig.String())
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

		// Check admin token if configured
		if len(s.config.AdminTokens) > 0 {
			token := s.extractToken(c)
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Unauthorized: admin token required",
				})
				c.Abort()
				return
			}

			if !s.isValidAdminToken(token) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Unauthorized: invalid admin token",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// validateSecurityConfig checks security configuration at startup
func (s *Server) validateSecurityConfig() error {
	// Check if we should disable default token in production
	disableDefault := os.Getenv("MCP_PROXY_DISABLE_DEFAULT_TOKEN") == "true"
	if !disableDefault {
		// Not enforcing, just warn
		for _, token := range s.config.AdminTokens {
			if token == "CHANGE_ME_ADMIN_TOKEN" {
				logger.Warn("SECURITY WARNING: Using default admin token. " +
					"Set MCP_PROXY_DISABLE_DEFAULT_TOKEN=true to fail on startup with default token")
			}
		}
		return nil
	}

	// Strict mode: fail if default token is found
	for _, token := range s.config.AdminTokens {
		if token == "CHANGE_ME_ADMIN_TOKEN" {
			return fmt.Errorf("default admin token 'CHANGE_ME_ADMIN_TOKEN' is not allowed. " +
				"Please set a secure admin token via MCP_PROXY_ADMIN_TOKEN environment variable")
		}
	}

	// Ensure at least one admin token is configured
	if len(s.config.AdminTokens) == 0 {
		return fmt.Errorf("no admin tokens configured. " +
			"Please set MCP_PROXY_ADMIN_TOKEN environment variable")
	}

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

// extractToken extracts the authentication token from the request
func (s *Server) extractToken(c *gin.Context) string {
	// Try Authorization header first
	auth := c.GetHeader("Authorization")
	if auth != "" {
		// Support both "Bearer token" and "token" formats
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return strings.TrimSpace(auth)
	}

	// Try query parameter as fallback
	return c.Query("token")
}

// isValidAdminToken checks if the provided token is valid
func (s *Server) isValidAdminToken(token string) bool {
	for _, validToken := range s.config.AdminTokens {
		if token == validToken {
			return true
		}
	}
	return false
}
