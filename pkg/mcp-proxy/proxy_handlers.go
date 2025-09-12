package mcpproxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

// ProxyHandlers handles MCP protocol proxy requests
type ProxyHandlers struct {
	storage       Storage
	clientManager ClientManager
	baseURL       string                  // Base URL for SSE servers
	servers       map[string]*ProxyServer // Map of MCP name to proxy server
	serversMutex  sync.RWMutex            // Protects servers map
}

// ProxyServer wraps an MCP server and SSE server for proxying requests
type ProxyServer struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
	client    MCPClientInterface
	def       *MCPDefinition // Cache definition to avoid repeated storage queries
}

// NewProxyHandlers creates a new proxy handlers instance
func NewProxyHandlers(
	storage Storage,
	clientManager ClientManager,
	baseURL string,
) *ProxyHandlers {
	return &ProxyHandlers{
		storage:       storage,
		clientManager: clientManager,
		baseURL:       baseURL,
		servers:       make(map[string]*ProxyServer),
	}
}

// RegisterMCPProxy registers an MCP client as a proxy server
func (p *ProxyHandlers) RegisterMCPProxy(ctx context.Context, name string, def *MCPDefinition) error {
	log := logger.FromContext(ctx)
	log.Info("Registering MCP proxy", "name", name)

	// Get or create MCP client
	client, err := p.clientManager.GetClient(name)
	if err != nil {
		log.Error("Failed to get MCP client", "name", name, "error", err)
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
	if _, exists := p.servers[name]; exists {
		p.serversMutex.Unlock()
		return fmt.Errorf("MCP proxy %q is already registered", name)
	}
	p.servers[name] = proxyServer
	p.serversMutex.Unlock()

	// Initialize the client connection and add its capabilities to the server
	// This runs in background after the server is registered
	go func() {
		// Independent context with timeout to avoid indefinite wait
		bgCtx := logger.ContextWithLogger(context.Background(), log)
		timeout := 30 * time.Second
		if def != nil && def.Timeout > 0 {
			timeout = def.Timeout
		}
		waitCtx, cancel := context.WithTimeout(bgCtx, timeout)
		defer cancel()
		if err := p.initializeClientConnection(waitCtx, client, mcpServer, name, def); err != nil {
			log.Error("Failed to initialize MCP client connection", "name", name, "error", err)
			// Update client status to reflect initialization failure
			if status := client.GetStatus(); status != nil {
				status.UpdateStatus(StatusError, fmt.Sprintf("initialization failed: %v", err))
			}
		}
	}()

	log.Info("Successfully registered MCP proxy", "name", name)
	return nil
}

// UnregisterMCPProxy removes an MCP proxy server
func (p *ProxyHandlers) UnregisterMCPProxy(ctx context.Context, name string) error {
	log := logger.FromContext(ctx)
	log.Info("Unregistering MCP proxy", "name", name)

	p.serversMutex.Lock()
	proxyServer, exists := p.servers[name]
	if exists {
		delete(p.servers, name)
	}
	p.serversMutex.Unlock()

	if !exists {
		log.Debug("Proxy server not found for unregistration", "name", name)
		return nil
	}

	// Shutdown server resources first
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if proxyServer.sseServer != nil {
		if err := proxyServer.sseServer.Shutdown(shutdownCtx); err != nil {
			log.Error("Failed to shutdown SSE server", "name", name, "error", err)
		}
	}
	// Disconnect the client connection
	if proxyServer.client != nil {
		disconnectCtx := logger.ContextWithLogger(shutdownCtx, log)
		if err := proxyServer.client.Disconnect(disconnectCtx); err != nil {
			log.Error("Failed to disconnect MCP client", "name", name, "error", err)
		}
	}

	log.Info("Successfully unregistered MCP proxy", "name", name)
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
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /{name}/sse [get]
// @Router /{name}/sse/{path} [get]
func (p *ProxyHandlers) SSEProxyHandler(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MCP name is required"})
		return
	}

	log.Debug("Handling SSE proxy request", "name", name, "path", c.Request.URL.Path)

	// Get the proxy server for this MCP
	p.serversMutex.RLock()
	proxyServer, exists := p.servers[name]
	p.serversMutex.RUnlock()

	if !exists {
		log.Debug("MCP proxy server not found", "name", name)
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Check if proxy server is properly initialized
	if proxyServer.def == nil {
		log.Error("MCP proxy server not properly initialized", "name", name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP server not properly initialized"})
		return
	}

	// Use cached definition to avoid repeated storage queries

	middlewares := []gin.HandlerFunc{
		recoverMiddleware(name),
	}

	// Check if SSE server is available (test scenario)
	if proxyServer.sseServer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SSE server not initialized"})
		return
	}

	// Wrap SSE server with middlewares and call
	wrappedHandler := wrapWithGinMiddlewares(proxyServer.sseServer, middlewares...)
	wrappedHandler(c)
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
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /{name}/stream [get]
// @Router /{name}/stream [post]
// @Router /{name}/stream [put]
// @Router /{name}/stream [patch]
// @Router /{name}/stream [delete]
// @Router /{name}/stream/{path} [get]
// @Router /{name}/stream/{path} [post]
// @Router /{name}/stream/{path} [put]
// @Router /{name}/stream/{path} [patch]
// @Router /{name}/stream/{path} [delete]
func (p *ProxyHandlers) StreamableHTTPProxyHandler(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MCP name is required"})
		return
	}

	log.Debug("Handling streamable HTTP proxy request", "name", name, "path", c.Request.URL.Path)

	// For streamable HTTP, we use the same SSE handler approach
	// The client transport handles the difference
	p.SSEProxyHandler(c)
}

// initializeClientConnection waits for the MCP client to be connected and then loads its capabilities to the server
func (p *ProxyHandlers) initializeClientConnection(
	ctx context.Context,
	client MCPClientInterface,
	mcpServer *server.MCPServer,
	name string,
	def *MCPDefinition,
) error {
	log := logger.FromContext(ctx)
	log.Debug("Waiting for MCP client to be connected", "name", name)

	// Wait for the client to be connected by the ClientManager.
	// This requires a way to observe the client's status. The client has WaitUntilConnected method.
	if err := client.WaitUntilConnected(ctx); err != nil {
		return fmt.Errorf("client connection timed out or failed: %w", err)
	}

	log.Debug("MCP client is connected, loading resources", "name", name)

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
	log := logger.FromContext(ctx)
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
				log.Warn("Failed to add capability",
					"capability", capability.name,
					"error", err)
			}
			return nil // Don't propagate errors for optional capabilities
		})
	}

	// Wait for all optional operations to complete
	if err := optionalGroup.Wait(); err != nil {
		log.Debug("Unexpected error from optional operations", "error", err)
	}
}

// GetProxyServer returns the proxy server for a given MCP name (for testing)
func (p *ProxyHandlers) GetProxyServer(name string) *ProxyServer {
	p.serversMutex.RLock()
	defer p.serversMutex.RUnlock()
	return p.servers[name]
}

// Note: IP allowlist middleware has been removed. Network-level controls should be used instead.
