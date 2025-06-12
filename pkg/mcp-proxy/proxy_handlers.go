package mcpproxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

// ProxyHandlers handles MCP protocol proxy requests
type ProxyHandlers struct {
	storage          Storage
	clientManager    ClientManager
	baseURL          string                  // Base URL for SSE servers
	servers          map[string]*ProxyServer // Map of MCP name to proxy server
	serversMu        sync.RWMutex            // Protects servers map
	globalAuthTokens []string                // Global auth tokens inherited by all clients
}

// ProxyServer wraps an MCP server and SSE server for proxying requests
type ProxyServer struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
	client    *MCPClient
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
func (p *ProxyHandlers) RegisterMCPProxy(ctx context.Context, name string, def *MCPDefinition) error {
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
	}

	// Store the proxy server BEFORE initialization to avoid race condition
	// This allows requests to find the server, even if initialization is still in progress
	p.serversMu.Lock()
	p.servers[name] = proxyServer
	p.serversMu.Unlock()

	// Initialize the client connection and add its capabilities to the server
	// This runs in background after the server is registered
	go func() {
		if err := p.initializeClientConnection(ctx, client, mcpServer, name); err != nil {
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

	p.serversMu.Lock()
	proxyServer, exists := p.servers[name]
	if exists {
		delete(p.servers, name)
	}
	p.serversMu.Unlock()

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
	p.serversMu.RLock()
	proxyServer, exists := p.servers[name]
	p.serversMu.RUnlock()

	if !exists {
		logger.Warn("MCP proxy server not found", "name", name)
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Get MCP definition for options first
	def, err := p.storage.LoadMCP(context.Background(), name)
	if err != nil {
		logger.Error("Failed to get MCP definition", "name", name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MCP configuration"})
		return
	}

	// Apply middleware chain in security-first order
	middlewares := []MiddlewareFunc{
		p.recoverMiddleware(name), // Recovery should be outermost
	}

	// Add auth middleware first (before logging to prevent logging unauthorized requests)
	allTokens := p.combineAuthTokens(def.AuthTokens)
	if len(allTokens) > 0 {
		middlewares = append(middlewares, p.newAuthMiddleware(allTokens))
	}

	// Add logging middleware after auth
	if def.LogEnabled {
		middlewares = append(middlewares, p.loggerMiddleware(name))
	}

	// Chain middlewares and serve through SSE server
	handler := p.chainMiddleware(proxyServer.sseServer, middlewares...)
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
) error {
	logger.Info("Initializing MCP client connection", "name", name)

	// Connect to the MCP server (this handles initialization internally)
	err := client.Connect(ctx)
	if err != nil {
		return err
	}

	logger.Info("Successfully initialized MCP client", "name", name)

	// Load critical capabilities first (tools)
	if err := p.loadCriticalCapabilities(ctx, client, mcpServer, name); err != nil {
		return err
	}

	// Load optional capabilities concurrently
	p.loadOptionalCapabilities(ctx, client, mcpServer, name)

	return nil
}

// loadCriticalCapabilities loads tools which are required for the proxy to function
func (p *ProxyHandlers) loadCriticalCapabilities(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) error {
	// Tools are critical - must succeed
	toolsGroup, toolsCtx := errgroup.WithContext(ctx)
	toolsGroup.Go(func() error {
		return p.addToolsToServer(toolsCtx, client, mcpServer, name)
	})

	return toolsGroup.Wait()
}

// loadOptionalCapabilities loads non-critical capabilities in parallel
func (p *ProxyHandlers) loadOptionalCapabilities(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) {
	optionalGroup, optionalCtx := errgroup.WithContext(ctx)

	// Define optional capability loaders
	capabilities := []struct {
		name   string
		loader func(context.Context, *MCPClient, *server.MCPServer, string) error
	}{
		{"prompts", p.addPromptsToServer},
		{"resources", p.addResourcesToServer},
		{"resource_templates", p.addResourceTemplatesToServer},
	}

	// Load each capability concurrently
	for _, cap := range capabilities {
		capability := cap // capture loop variable
		optionalGroup.Go(func() error {
			if err := capability.loader(optionalCtx, client, mcpServer, name); err != nil {
				logger.Warn("Failed to add capability",
					"name", name,
					"capability", capability.name,
					"error", err)
			}
			return nil // Don't propagate errors for optional capabilities
		})
	}

	// Wait for all optional operations to complete
	if err := optionalGroup.Wait(); err != nil {
		logger.Warn("Unexpected error from optional operations", "name", name, "error", err)
	}
}

// addToolsToServer adds MCP client tools to the proxy server
func (p *ProxyHandlers) addToolsToServer(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) error {
	tools, err := client.ListTools(ctx)
	if err != nil {
		return err
	}

	logger.Info("Successfully listed tools", "name", name, "count", len(tools))

	// Get MCP definition for tool filtering
	def := client.GetDefinition()
	filterFunc := p.createToolFilter(def.ToolFilter, name)

	addedCount := 0
	for i := range tools {
		tool := &tools[i]
		if filterFunc(tool.Name) {
			logger.Debug("Adding tool to proxy server", "name", name, "tool", tool.Name)
			mcpServer.AddTool(*tool, client.CallTool)
			addedCount++
		}
	}

	logger.Info("Successfully added filtered tools", "name", name, "total", len(tools), "added", addedCount)
	return nil
}

// addPromptsToServer adds MCP client prompts to the proxy server with pagination support
// Uses concurrent processing within each batch for improved performance
func (p *ProxyHandlers) addPromptsToServer(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) error {
	var cursor string
	totalCount := 0

	// Create a semaphore to limit concurrent prompt additions per batch
	const maxConcurrentAdds = 5
	sem := make(chan struct{}, maxConcurrentAdds)

	for {
		prompts, nextCursor, err := client.ListPromptsWithCursor(ctx, cursor)
		if err != nil {
			return err
		}

		if len(prompts) == 0 {
			break
		}

		totalCount += len(prompts)
		logger.Debug("Listed prompts batch", "name", name, "count", len(prompts), "cursor", cursor)

		// Process prompts in this batch concurrently
		g, gCtx := errgroup.WithContext(ctx)

		for _, prompt := range prompts {
			prompt := prompt // capture loop variable

			g.Go(func() error {
				// Acquire semaphore to limit concurrency
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}

				logger.Debug("Adding prompt to proxy server", "name", name, "prompt", prompt.Name)
				mcpServer.AddPrompt(prompt, client.GetPrompt)
				return nil
			})
		}

		// Wait for all prompts in this batch to be processed
		if err := g.Wait(); err != nil {
			return err
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info("Successfully added all prompts", "name", name, "total", totalCount)
	return nil
}

// addResourcesToServer adds MCP client resources to the proxy server with pagination support
// Uses concurrent processing within each batch for improved performance
func (p *ProxyHandlers) addResourcesToServer(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) error {
	var cursor string
	totalCount := 0

	// Create a semaphore to limit concurrent resource additions per batch
	const maxConcurrentAdds = 5
	sem := make(chan struct{}, maxConcurrentAdds)

	for {
		resources, nextCursor, err := client.ListResourcesWithCursor(ctx, cursor)
		if err != nil {
			return err
		}

		if len(resources) == 0 {
			break
		}

		totalCount += len(resources)
		logger.Debug("Listed resources batch", "name", name, "count", len(resources), "cursor", cursor)

		// Process resources in this batch concurrently
		g, gCtx := errgroup.WithContext(ctx)

		for _, resource := range resources {
			resource := resource // capture loop variable

			g.Go(func() error {
				// Acquire semaphore to limit concurrency
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}

				logger.Debug("Adding resource to proxy server", "name", name, "resource", resource.URI)
				mcpServer.AddResource(
					resource,
					func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
						result, err := client.ReadResource(ctx, request)
						if err != nil {
							return nil, err
						}
						return result.Contents, nil
					},
				)
				return nil
			})
		}

		// Wait for all resources in this batch to be processed
		if err := g.Wait(); err != nil {
			return err
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info("Successfully added all resources", "name", name, "total", totalCount)
	return nil
}

// addResourceTemplatesToServer adds MCP client resource templates to the proxy server with pagination support
// Uses concurrent processing within each batch for improved performance
func (p *ProxyHandlers) addResourceTemplatesToServer(
	ctx context.Context,
	client *MCPClient,
	mcpServer *server.MCPServer,
	name string,
) error {
	var cursor string
	totalCount := 0

	// Create a semaphore to limit concurrent resource template additions per batch
	const maxConcurrentAdds = 5
	sem := make(chan struct{}, maxConcurrentAdds)

	for {
		templates, nextCursor, err := client.ListResourceTemplatesWithCursor(ctx, cursor)
		if err != nil {
			return err
		}

		if len(templates) == 0 {
			break
		}

		totalCount += len(templates)
		logger.Debug("Listed resource templates batch", "name", name, "count", len(templates), "cursor", cursor)

		// Process resource templates in this batch concurrently
		g, gCtx := errgroup.WithContext(ctx)

		for _, template := range templates {
			template := template // capture loop variable

			g.Go(func() error {
				// Acquire semaphore to limit concurrency
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}

				logger.Debug("Adding resource template to proxy server", "name", name, "template", template.Name)
				mcpServer.AddResourceTemplate(
					template,
					func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
						result, err := client.ReadResource(ctx, request)
						if err != nil {
							return nil, err
						}
						return result.Contents, nil
					},
				)
				return nil
			})
		}

		// Wait for all resource templates in this batch to be processed
		if err := g.Wait(); err != nil {
			return err
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info("Successfully added all resource templates", "name", name, "total", totalCount)
	return nil
}

// Middleware functions adapted from the reference implementation

type MiddlewareFunc func(http.Handler) http.Handler

func (p *ProxyHandlers) chainMiddleware(h http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	for _, mw := range middlewares {
		h = mw(h)
	}
	return h
}

// combineAuthTokens combines global auth tokens with client-specific tokens
func (p *ProxyHandlers) combineAuthTokens(clientTokens []string) []string {
	if len(p.globalAuthTokens) == 0 {
		return clientTokens
	}

	if len(clientTokens) == 0 {
		return p.globalAuthTokens
	}

	// Combine both sets, avoiding duplicates
	combined := make([]string, 0, len(p.globalAuthTokens)+len(clientTokens))
	tokenSet := make(map[string]struct{})

	// Add global tokens first
	for _, token := range p.globalAuthTokens {
		if token != "" {
			combined = append(combined, token)
			tokenSet[token] = struct{}{}
		}
	}

	// Add client tokens, skipping duplicates
	for _, token := range clientTokens {
		if token != "" {
			if _, exists := tokenSet[token]; !exists {
				combined = append(combined, token)
				tokenSet[token] = struct{}{}
			}
		}
	}

	return combined
}

func (p *ProxyHandlers) newAuthMiddleware(tokens []string) MiddlewareFunc {
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		tokenSet[token] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(tokenSet) != 0 {
				authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
				if authHeader == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Case-insensitive Bearer prefix check
				const bearerPrefix = "bearer "
				if len(authHeader) < len(bearerPrefix) ||
					!strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				token := strings.TrimSpace(authHeader[len(bearerPrefix):])
				if token == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				if _, ok := tokenSet[token]; !ok {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (p *ProxyHandlers) loggerMiddleware(prefix string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("Proxy request", "prefix", prefix, "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

func (p *ProxyHandlers) recoverMiddleware(prefix string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Recovered from panic", "prefix", prefix, "error", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// createToolFilter creates a tool filtering function based on configuration
func (p *ProxyHandlers) createToolFilter(filter *ToolFilter, clientName string) func(string) bool {
	if filter == nil || len(filter.List) == 0 {
		return func(_ string) bool { return true }
	}

	filterSet := make(map[string]struct{})
	for _, toolName := range filter.List {
		filterSet[toolName] = struct{}{}
	}

	switch filter.Mode {
	case ToolFilterAllow:
		return func(toolName string) bool {
			_, inList := filterSet[toolName]
			if !inList {
				logger.Debug("Tool filtered out by allow list", "client", clientName, "tool", toolName)
			}
			return inList
		}
	case ToolFilterBlock:
		return func(toolName string) bool {
			_, inList := filterSet[toolName]
			if inList {
				logger.Debug("Tool filtered out by block list", "client", clientName, "tool", toolName)
			}
			return !inList
		}
	default:
		logger.Warn("Unknown tool filter mode, allowing all tools", "client", clientName, "mode", filter.Mode)
		return func(_ string) bool { return true }
	}
}

// GetProxyServer returns the proxy server for a given MCP name (for testing)
func (p *ProxyHandlers) GetProxyServer(name string) *ProxyServer {
	p.serversMu.RLock()
	defer p.serversMu.RUnlock()
	return p.servers[name]
}
