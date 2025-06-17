package mcpproxy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient wraps the MCP-Go client with our management layer
type MCPClient struct {
	// Core fields
	definition *MCPDefinition
	status     *MCPStatus
	storage    Storage
	config     *ClientManagerConfig

	// MCP-Go client
	mcpClient *mcpclient.Client

	// MCP-specific configuration
	needPing        bool
	needManualStart bool

	// Lifecycle management
	initialized bool
	connected   bool
	managerCtx  context.Context    // Manager context for long-lived operations
	pingCancel  context.CancelFunc // Cancel function for ping routine
	pingDone    chan struct{}      // Signal for ping routine completion
	mu          sync.RWMutex
}

// NewMCPClient creates a new MCP client based on the transport type
func NewMCPClient(
	ctx context.Context,
	def *MCPDefinition,
	storage Storage,
	config *ClientManagerConfig,
) (*MCPClient, error) {
	mcpClient, needPing, needManualStart, err := createMCPClient(def)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	return &MCPClient{
		definition:      def.Clone(),
		status:          NewMCPStatus(def.Name),
		storage:         storage,
		config:          config,
		mcpClient:       mcpClient,
		needPing:        needPing,
		needManualStart: needManualStart,
		managerCtx:      ctx,
		pingDone:        make(chan struct{}),
	}, nil
}

// GetDefinition returns the MCP definition
func (c *MCPClient) GetDefinition() *MCPDefinition {
	return c.definition.Clone()
}

// GetStatus returns the current status (thread-safe copy)
func (c *MCPClient) GetStatus() *MCPStatus {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	// Return a thread-safe copy with calculated uptime
	return status.SafeCopy()
}

// IsConnected returns true if the client is connected
func (c *MCPClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// updateStatus updates the client status
func (c *MCPClient) updateStatus(status ConnectionStatus, errorMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.UpdateStatus(status, errorMsg)
	c.connected = (status == StatusConnected)
}

// recordRequest records a successful request
func (c *MCPClient) recordRequest(responseTime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.RecordRequest(responseTime)
}

// recordError records an error (thread-safe)
func (c *MCPClient) recordError() {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	status.IncrementErrors()
}

// createMCPClient creates the appropriate MCP client based on transport type
func createMCPClient(def *MCPDefinition) (*mcpclient.Client, bool, bool, error) {
	switch def.Transport {
	case TransportStdio:
		return createStdioMCPClient(def)
	case TransportSSE:
		return createSSEMCPClient(def)
	case TransportStreamableHTTP:
		return createStreamableHTTPMCPClient(def)
	default:
		return nil, false, false, fmt.Errorf("unsupported transport type: %s", def.Transport)
	}
}

// createStdioMCPClient creates a stdio MCP client
func createStdioMCPClient(def *MCPDefinition) (*mcpclient.Client, bool, bool, error) {
	envs := make([]string, 0, len(def.Env))
	for key, value := range def.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}

	client, err := mcpclient.NewStdioMCPClient(def.Command, envs, def.Args...)
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to create stdio client: %w", err)
	}

	return client, false, false, nil
}

// createSSEMCPClient creates an SSE MCP client
func createSSEMCPClient(def *MCPDefinition) (*mcpclient.Client, bool, bool, error) {
	var options []transport.ClientOption
	if len(def.Headers) > 0 {
		options = append(options, transport.WithHeaders(def.Headers))
	}

	client, err := mcpclient.NewSSEMCPClient(def.URL, options...)
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to create SSE client: %w", err)
	}

	return client, true, true, nil
}

// createStreamableHTTPMCPClient creates a streamable HTTP MCP client
func createStreamableHTTPMCPClient(def *MCPDefinition) (*mcpclient.Client, bool, bool, error) {
	var options []transport.StreamableHTTPCOption
	if len(def.Headers) > 0 {
		options = append(options, transport.WithHTTPHeaders(def.Headers))
	}
	if def.Timeout > 0 {
		options = append(options, transport.WithHTTPTimeout(def.Timeout))
	}

	client, err := mcpclient.NewStreamableHttpClient(def.URL, options...)
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to create streamable HTTP client: %w", err)
	}

	return client, true, true, nil
}

// Connect establishes connection to the MCP server
func (c *MCPClient) Connect(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Connecting to MCP server", "transport", c.definition.Transport, "mcp_name", c.definition.Name)

	// Check if already connected (without lock to avoid deadlock)
	c.mu.RLock()
	alreadyConnected := c.connected
	c.mu.RUnlock()

	if alreadyConnected {
		return fmt.Errorf("client is already connected")
	}

	// Start the client if needed (for SSE/HTTP transports) - no lock held
	if c.needManualStart {
		if err := c.mcpClient.Start(ctx); err != nil {
			c.updateStatus(StatusError, fmt.Sprintf("failed to start client: %v", err))
			return fmt.Errorf("failed to start MCP client: %w", err)
		}
	}

	// Initialize the MCP connection - no lock held
	if err := c.initializeMCP(ctx); err != nil {
		// Clean up the started client if initialization fails
		if c.needManualStart {
			if closeErr := c.mcpClient.Close(); closeErr != nil {
				log.Error("Failed to close client after initialization failure", "error", closeErr)
			}
		}
		c.updateStatus(StatusError, fmt.Sprintf("failed to initialize: %v", err))
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	// Update state under lock
	c.mu.Lock()
	c.initialized = true
	c.connected = true
	c.status.UpdateStatus(StatusConnected, "")
	c.mu.Unlock()

	// Start ping routine if needed - use manager context for proper lifecycle
	if c.needPing {
		pingCtx, pingCancel := context.WithCancel(c.managerCtx)
		c.mu.Lock()
		c.pingCancel = pingCancel
		c.mu.Unlock()
		pingCtx = logger.ContextWithLogger(pingCtx, log)
		go c.startPingRoutine(pingCtx)
	}

	log.Info("Successfully connected to MCP server", "mcp_name", c.definition.Name)
	return nil
}

// Disconnect closes the connection to the MCP server
func (c *MCPClient) Disconnect(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Disconnecting from MCP server", "mcp_name", c.definition.Name)

	// Check if already disconnected (without lock to avoid deadlock)
	c.mu.RLock()
	alreadyDisconnected := !c.connected
	c.mu.RUnlock()

	if alreadyDisconnected {
		return nil
	}

	// Mark as disconnected and cancel ping routine
	c.mu.Lock()
	c.connected = false
	pingCancel := c.pingCancel
	c.mu.Unlock()

	// Cancel ping routine if it was started
	if c.needPing && pingCancel != nil {
		pingCancel()
		<-c.pingDone // Wait for routine to exit (will be immediate now)
	}

	// Close the MCP client - no lock held during network operation
	if err := c.mcpClient.Close(); err != nil {
		log.Error("Error closing MCP client", "error", err, "mcp_name", c.definition.Name)
	}

	// Update remaining state under lock
	c.mu.Lock()
	c.initialized = false
	c.status.UpdateStatus(StatusDisconnected, "")
	c.mu.Unlock()

	log.Info("Disconnected from MCP server", "mcp_name", c.definition.Name)
	return nil
}

// SendRequest sends a request to the MCP server and returns the response
func (c *MCPClient) SendRequest(ctx context.Context, _ []byte) ([]byte, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	start := time.Now()
	defer func() {
		responseTime := time.Since(start)
		c.recordRequest(responseTime)
	}()

	// For now, we'll implement this as a simple ping to verify connectivity
	// In a full implementation, you'd parse the request and route it appropriately
	err := c.mcpClient.Ping(ctx)
	if err != nil {
		c.recordError()
		return nil, fmt.Errorf("MCP request failed: %w", err)
	}

	// Return a simple success response
	return []byte(`{"result": "success"}`), nil
}

// Health performs a health check on the connection
func (c *MCPClient) Health(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("client is not connected")
	}

	// Use the MCP ping method for health checking
	if err := c.mcpClient.Ping(ctx); err != nil {
		return fmt.Errorf("MCP health check failed: %w", err)
	}

	return nil
}

// initializeMCP performs the MCP initialization handshake
func (c *MCPClient) initializeMCP(ctx context.Context) error {
	log := logger.FromContext(ctx)
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "compozy-mcp-proxy",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{
		Experimental: make(map[string]any),
		Roots:        nil,
		Sampling:     nil,
	}

	_, err := c.mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		return fmt.Errorf("MCP initialization failed: %w", err)
	}

	log.Info("MCP client initialized successfully", "mcp_name", c.definition.Name)
	return nil
}

// startPingRoutine starts a background ping routine for connection health
func (c *MCPClient) startPingRoutine(ctx context.Context) {
	log := logger.FromContext(ctx)
	log.Debug("Starting ping routine", "mcp_name", c.definition.Name)
	defer close(c.pingDone) // Signal completion when routine exits

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug("Context canceled, stopping ping routine", "mcp_name", c.definition.Name)
			return
		case <-ticker.C:
			if !c.IsConnected() {
				log.Debug("Client disconnected, stopping ping routine", "mcp_name", c.definition.Name)
				return
			}

			pingCtx, cancel := context.WithTimeout(ctx, DefaultConnectTimeout)
			err := c.mcpClient.Ping(pingCtx)
			cancel()

			if err != nil {
				log.Warn("Ping failed", "error", err, "mcp_name", c.definition.Name)
				c.updateStatus(StatusError, fmt.Sprintf("Ping failed: %v", err))
			}
		}
	}
}

// ListTools returns available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	var allTools []mcp.Tool
	request := mcp.ListToolsRequest{}

	for {
		response, err := c.mcpClient.ListTools(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}

		allTools = append(allTools, response.Tools...)

		if response.NextCursor == "" {
			break
		}
		request.Params.Cursor = response.NextCursor
	}

	return allTools, nil
}

// CallTool calls a specific tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	start := time.Now()
	defer func() {
		responseTime := time.Since(start)
		c.recordRequest(responseTime)
	}()

	result, err := c.mcpClient.CallTool(ctx, request)
	if err != nil {
		c.recordError()
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	return result, nil
}

// ListPrompts returns available prompts from the MCP server
func (c *MCPClient) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	var allPrompts []mcp.Prompt
	request := mcp.ListPromptsRequest{}

	for {
		response, err := c.mcpClient.ListPrompts(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list prompts: %w", err)
		}

		allPrompts = append(allPrompts, response.Prompts...)

		if response.NextCursor == "" {
			break
		}
		request.Params.Cursor = response.NextCursor
	}

	return allPrompts, nil
}

// GetPrompt gets a specific prompt from the MCP server
func (c *MCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	start := time.Now()
	defer func() {
		responseTime := time.Since(start)
		c.recordRequest(responseTime)
	}()

	result, err := c.mcpClient.GetPrompt(ctx, request)
	if err != nil {
		c.recordError()
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	return result, nil
}

// ListResources returns available resources from the MCP server
func (c *MCPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	var allResources []mcp.Resource
	request := mcp.ListResourcesRequest{}

	for {
		response, err := c.mcpClient.ListResources(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}

		allResources = append(allResources, response.Resources...)

		if response.NextCursor == "" {
			break
		}
		request.Params.Cursor = response.NextCursor
	}

	return allResources, nil
}

// ReadResource reads a specific resource from the MCP server
func (c *MCPClient) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	start := time.Now()
	defer func() {
		responseTime := time.Since(start)
		c.recordRequest(responseTime)
	}()

	result, err := c.mcpClient.ReadResource(ctx, request)
	if err != nil {
		c.recordError()
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	return result, nil
}

// ListPromptsWithCursor returns prompts with cursor for pagination
func (c *MCPClient) ListPromptsWithCursor(ctx context.Context, cursor string) ([]mcp.Prompt, string, error) {
	if !c.IsConnected() {
		return nil, "", fmt.Errorf("client is not connected")
	}

	request := mcp.ListPromptsRequest{}
	if cursor != "" {
		request.Params.Cursor = mcp.Cursor(cursor)
	}

	response, err := c.mcpClient.ListPrompts(ctx, request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list prompts: %w", err)
	}

	return response.Prompts, string(response.NextCursor), nil
}

// ListResourcesWithCursor returns resources with cursor for pagination
func (c *MCPClient) ListResourcesWithCursor(ctx context.Context, cursor string) ([]mcp.Resource, string, error) {
	if !c.IsConnected() {
		return nil, "", fmt.Errorf("client is not connected")
	}

	request := mcp.ListResourcesRequest{}
	if cursor != "" {
		request.Params.Cursor = mcp.Cursor(cursor)
	}

	response, err := c.mcpClient.ListResources(ctx, request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list resources: %w", err)
	}

	return response.Resources, string(response.NextCursor), nil
}

// ListResourceTemplates returns available resource templates from the MCP server
func (c *MCPClient) ListResourceTemplates(ctx context.Context) ([]mcp.ResourceTemplate, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("client is not connected")
	}

	var allTemplates []mcp.ResourceTemplate
	request := mcp.ListResourceTemplatesRequest{}

	for {
		response, err := c.mcpClient.ListResourceTemplates(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource templates: %w", err)
		}

		allTemplates = append(allTemplates, response.ResourceTemplates...)

		if response.NextCursor == "" {
			break
		}
		request.Params.Cursor = response.NextCursor
	}

	return allTemplates, nil
}

// ListResourceTemplatesWithCursor returns resource templates with cursor for pagination
func (c *MCPClient) ListResourceTemplatesWithCursor(
	ctx context.Context,
	cursor string,
) ([]mcp.ResourceTemplate, string, error) {
	if !c.IsConnected() {
		return nil, "", fmt.Errorf("client is not connected")
	}

	request := mcp.ListResourceTemplatesRequest{}
	if cursor != "" {
		request.Params.Cursor = mcp.Cursor(cursor)
	}

	response, err := c.mcpClient.ListResourceTemplates(ctx, request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list resource templates: %w", err)
	}

	return response.ResourceTemplates, string(response.NextCursor), nil
}

// WaitUntilConnected waits for the client to be connected, with timeout
func (c *MCPClient) WaitUntilConnected(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if c.IsConnected() {
				return nil
			}
			// Check if client is in error state
			status := c.GetStatus()
			if status.Status == StatusError {
				return fmt.Errorf("client connection failed: %s", status.LastError)
			}
		}
	}
}
