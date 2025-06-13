package mcpproxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

// ProxyHandlers handles MCP protocol proxy requests
type ProxyHandlers struct {
	storage          Storage
	clientManager    ClientManager
	baseURL          string                  // Base URL for SSE servers
	servers          map[string]*ProxyServer // Map of MCP name to proxy server
	serversMutex     sync.RWMutex            // Protects servers map
	globalAuthTokens []string                // Global auth tokens inherited by all clients
}

// ProxyServer wraps an MCP server and SSE server for proxying requests
type ProxyServer struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
	client    *MCPClient
	def       *MCPDefinition // Cache definition to avoid repeated storage queries
}

// NewProxyHandlers creates a new proxy handlers instance
func NewProxyHandlers(
	storage Storage,
	clientManager ClientManager,
	baseURL string,
	globalAuthTokens []string,
) *ProxyHandlers {
	return &ProxyHandlers{
		storage:          storage,
		clientManager:    clientManager,
		baseURL:          baseURL,
		servers:          make(map[string]*ProxyServer),
		globalAuthTokens: globalAuthTokens,
	}
}

// RegisterMCPProxy registers an MCP client as a proxy server
func (p *ProxyHandlers) RegisterMCPProxy(_ context.Context, name string, def *MCPDefinition) error {
	logger.Info("Registering MCP proxy", "name", name)

	// Get or create MCP client
	client, err := p.clientManager.GetClient(name)
	if err != nil {
		logger.Error("Failed to get MCP client", "name", name, "error", err)
		return err
	}

	// Create MCP server for this client
	serverOpts := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	}

	// Add logging configuration based on MCPDefinition
	if def.LogEnabled {
		serverOpts = append(serverOpts, server.WithLogging())
	}

	mcpServer := server.NewMCPServer(
		name,
		"1.0.0", // version
		serverOpts...,
	)

	// Create SSE server for this MCP server
	sseServer := server.NewSSEServer(mcpServer,
		server.WithStaticBasePath(name),
		server.WithBaseURL(p.baseURL),
	)

	proxyServer := &ProxyServer{
		mcpServer: mcpServer,
		sseServer: sseServer,
		client:    client,
		def:       def,
	}

	// Store the proxy server BEFORE initialization to avoid race condition
	// This allows requests to find the server, even if initialization is still in progress
	p.serversMutex.Lock()
	p.servers[name] = proxyServer
	p.serversMutex.Unlock()

	// Initialize the client connection and add its capabilities to the server
	// This runs in background after the server is registered
	go func() {
		// Create independent context to avoid cancellation affecting background initialization
		bgCtx := context.Background()
		if err := p.initializeClientConnection(bgCtx, client, mcpServer, name, def); err != nil {
			logger.Error("Failed to initialize MCP client connection", "name", name, "error", err)
			// Update client status to reflect initialization failure
			if status := client.GetStatus(); status != nil {
				status.UpdateStatus(StatusError, fmt.Sprintf("initialization failed: %v", err))
			}
		}
	}()

	logger.Info("Successfully registered MCP proxy", "name", name)
	return nil
}

// UnregisterMCPProxy removes an MCP proxy server
func (p *ProxyHandlers) UnregisterMCPProxy(name string) error {
	logger.Info("Unregistering MCP proxy", "name", name)

	p.serversMutex.Lock()
	proxyServer, exists := p.servers[name]
	if exists {
		delete(p.servers, name)
	}
	p.serversMutex.Unlock()

	if !exists {
		logger.Warn("Proxy server not found for unregistration", "name", name)
		return nil
	}

	// Disconnect the client connection
	if proxyServer.client != nil {
		if err := proxyServer.client.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect MCP client", "name", name, "error", err)
		}
	}

	logger.Info("Successfully unregistered MCP proxy", "name", name)
	return nil
}

// SSEProxyHandler handles SSE proxy requests for MCP servers
// @Summary Proxy SSE requests to MCP server
// @Description Proxy Server-Sent Events requests to a specific MCP server
// @Tags MCP Proxy
// @Param name path string true "MCP name"
// @Param path path string false "Additional path"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 404 {object} map[string]interface{} "MCP not found"
// @Router /{name}/sse [get]
// @Router /{name}/sse/{path} [get]
func (p *ProxyHandlers) SSEProxyHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MCP name is required"})
		return
	}

	logger.Debug("Handling SSE proxy request", "name", name, "path", c.Request.URL.Path)

	// Get the proxy server for this MCP
	p.serversMutex.RLock()
	proxyServer, exists := p.servers[name]
	p.serversMutex.RUnlock()

	if !exists {
		logger.Warn("MCP proxy server not found", "name", name)
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Check if proxy server is properly initialized
	if proxyServer.def == nil {
		logger.Error("MCP proxy server not properly initialized", "name", name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP server not properly initialized"})
		return
	}

	// Use cached definition to avoid repeated storage queries
	def := proxyServer.def

	// Apply middleware chain in security-first order
	middlewares := []MiddlewareFunc{
		recoverMiddleware(name), // Recovery should be outermost
	}

	// Add auth middleware first (before logging to prevent logging unauthorized requests)
	allTokens := combineAuthTokens(p.globalAuthTokens, def.AuthTokens)
	if len(allTokens) > 0 {
		middlewares = append(middlewares, newAuthMiddleware(allTokens))
	}

	// Add logging middleware after auth
	if def.LogEnabled {
		middlewares = append(middlewares, loggerMiddleware(name))
	}

	// Chain middlewares and serve through SSE server
	handler := chainMiddleware(proxyServer.sseServer, middlewares...)
	handler.ServeHTTP(c.Writer, c.Request)
}

// StreamableHTTPProxyHandler handles streamable HTTP proxy requests
// @Summary Proxy streamable HTTP requests to MCP server
// @Description Proxy streamable HTTP requests to a specific MCP server
// @Tags MCP Proxy
// @Param name path string true "MCP name"
// @Param path path string false "Additional path"
// @Success 200 {string} string "HTTP stream"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 404 {object} map[string]interface{} "MCP not found"
// @Router /{name}/stream [any]
// @Router /{name}/stream/{path} [any]
func (p *ProxyHandlers) StreamableHTTPProxyHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MCP name is required"})
		return
	}

	logger.Debug("Handling streamable HTTP proxy request", "name", name, "path", c.Request.URL.Path)

	// For streamable HTTP, we use the same SSE handler approach
	// The client transport handles the difference
	p.SSEProxyHandler(c)
}

// initializeClientConnection initializes the MCP client and adds its capabilities to the server
func (p *ProxyHandlers) initializeClientConnection(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
	def *MCPDefinition,
) error {
	logger.Info("Initializing MCP client connection", "name", name)

	// Connect to the MCP server (this handles initialization internally)
	err := client.Connect(ctx)
	if err != nil {
		return err
	}

	logger.Info("Successfully initialized MCP client", "name", name)

	// Create resource loader
	resourceLoader := NewResourceLoader(client, mcpServer, name)

	// Load critical capabilities first (tools)
	if err := resourceLoader.LoadTools(ctx, def.ToolFilter); err != nil {
		return err
	}

	// Load optional capabilities concurrently
	p.loadOptionalCapabilities(ctx, resourceLoader)

	return nil
}

// loadOptionalCapabilities loads non-critical capabilities in parallel
func (p *ProxyHandlers) loadOptionalCapabilities(
	ctx context.Context,
	resourceLoader *ResourceLoader,
) {
	optionalGroup, optionalCtx := errgroup.WithContext(ctx)

	// Define optional capability loaders
	capabilities := []struct {
		name   string
		loader func(context.Context) error
	}{
		{"prompts", resourceLoader.LoadPrompts},
		{"resources", resourceLoader.LoadResources},
		{"resource_templates", resourceLoader.LoadResourceTemplates},
	}

	// Load each capability concurrently
	for _, cap := range capabilities {
		capability := cap // capture loop variable
		optionalGroup.Go(func() error {
			if err := capability.loader(optionalCtx); err != nil {
				logger.Warn("Failed to add capability",
					"capability", capability.name,
					"error", err)
			}
			return nil // Don't propagate errors for optional capabilities
		})
	}

	// Wait for all optional operations to complete
	if err := optionalGroup.Wait(); err != nil {
		logger.Warn("Unexpected error from optional operations", "error", err)
	}
}

// GetProxyServer returns the proxy server for a given MCP name (for testing)
func (p *ProxyHandlers) GetProxyServer(name string) *ProxyServer {
	p.serversMutex.RLock()
	defer p.serversMutex.RUnlock()
	return p.servers[name]
}
