package mcpproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/version"
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

	clonedDef, cerr := def.Clone()
	if cerr != nil {
		return nil, fmt.Errorf("failed to clone definition: %w", cerr)
	}
	client := &MCPClient{
		definition:      clonedDef,
		status:          NewMCPStatus(def.Name),
		storage:         storage,
		config:          config,
		mcpClient:       mcpClient,
		needPing:        needPing,
		needManualStart: needManualStart,
		managerCtx:      ctx,
		pingDone:        make(chan struct{}),
	}
	client.startStderrLogger()
	return client, nil
}

// GetDefinition returns the MCP definition
func (c *MCPClient) GetDefinition() *MCPDefinition {
	if cloned, err := c.definition.Clone(); err == nil && cloned != nil {
		return cloned
	}
	return c.definition
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

func (c *MCPClient) startStderrLogger() {
	if reader, ok := mcpclient.GetStderr(c.mcpClient); ok && reader != nil {
		log := logger.FromContext(c.managerCtx)
		go func() {
			scanner := bufio.NewScanner(reader)
			// Allow up to 1MB per line to avoid truncation of long stderr lines.
			scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
			for scanner.Scan() {
				log.Debug("MCP client stderr", "mcp_name", c.definition.Name, "line", scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				log.Debug("Error reading MCP client stderr", "mcp_name", c.definition.Name, "error", err)
			}
		}()
	}
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

	// For stdio transports, the upstream client already starts the process when
	// constructed via NewStdioMCPClient. Calling Start twice can cause races on
	// shared stdio readers. Avoid manual Start in Connect for stdio.
	return client, false, false, nil
}

// createSSEMCPClient creates an SSE MCP client
func createSSEMCPClient(def *MCPDefinition) (*mcpclient.Client, bool, bool, error) {
	var options []transport.ClientOption
	if len(def.Headers) > 0 {
		hdr := core.CloneMap(def.Headers)
		options = append(options, transport.WithHeaders(hdr))
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
		hdr := core.CloneMap(def.Headers)
		options = append(options, transport.WithHTTPHeaders(hdr))
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

	if err := c.ensureDisconnected(); err != nil {
		return err
	}
	if err := c.startClientIfRequired(ctx); err != nil {
		return err
	}
	if err := c.initializeConnection(ctx); err != nil {
		return err
	}

	c.markAsConnected()
	c.startPingRoutineIfNeeded(ctx)

	log.Info("Successfully connected to MCP server", "mcp_name", c.definition.Name)
	return nil
}

// ensureDisconnected verifies the client is not already connected.
func (c *MCPClient) ensureDisconnected() error {
	c.mu.RLock()
	alreadyConnected := c.connected
	c.mu.RUnlock()
	if alreadyConnected {
		return fmt.Errorf("client is already connected")
	}
	return nil
}

// startClientIfRequired starts transports that require manual startup.
func (c *MCPClient) startClientIfRequired(ctx context.Context) error {
	if !c.needManualStart {
		return nil
	}
	if err := c.mcpClient.Start(ctx); err != nil {
		c.updateStatus(StatusError, fmt.Sprintf("failed to start client: %v", err))
		return fmt.Errorf("failed to start MCP client: %w", err)
	}
	return nil
}

// initializeConnection initializes the MCP connection and closes on failure.
func (c *MCPClient) initializeConnection(ctx context.Context) error {
	if err := c.initializeMCP(ctx); err != nil {
		if closeErr := c.mcpClient.Close(); closeErr != nil {
			logger.FromContext(ctx).Error("Failed to close client after initialization failure", "error", closeErr)
		}
		c.updateStatus(StatusError, fmt.Sprintf("failed to initialize: %v", err))
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	return nil
}

// markAsConnected updates internal state to reflect a successful connection.
func (c *MCPClient) markAsConnected() {
	c.mu.Lock()
	c.initialized = true
	c.connected = true
	c.status.UpdateStatus(StatusConnected, "")
	c.mu.Unlock()
}

// startPingRoutineIfNeeded launches the ping routine when required by the transport.
func (c *MCPClient) startPingRoutineIfNeeded(ctx context.Context) {
	if !c.needPing {
		return
	}
	pingCtx, pingCancel := context.WithCancel(c.managerCtx)
	done := make(chan struct{})
	c.mu.Lock()
	c.pingCancel = pingCancel
	c.pingDone = done
	c.mu.Unlock()
	pingCtx = logger.ContextWithLogger(pingCtx, logger.FromContext(ctx))
	go c.startPingRoutine(pingCtx, done)
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
		if isExpectedCloseError(err) {
			log.Debug("MCP client closed with expected status", "mcp_name", c.definition.Name, "error", err)
		} else {
			log.Error("Error closing MCP client", "error", err, "mcp_name", c.definition.Name)
		}
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
		Version: version.Get().Version,
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
func (c *MCPClient) startPingRoutine(ctx context.Context, done chan struct{}) {
	log := logger.FromContext(ctx)
	log.Debug("Starting ping routine", "mcp_name", c.definition.Name)
	defer close(done) // Signal completion when routine exits

	interval := DefaultHealthCheckInterval
	if c.definition != nil && c.definition.HealthCheckInterval > 0 {
		interval = c.definition.HealthCheckInterval
	}
	ticker := time.NewTicker(interval)
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

func isExpectedCloseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code == 143 || code == -1 {
			return true
		}
	}
	return false
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
	log := logger.FromContext(ctx)
	log.Info("MCP tool call result",
		"mcp_name", c.definition.Name,
		"tool_name", request.Params.Name,
		"result", summarizeCallToolResult(result),
	)
	return result, nil
}

// summarizeCallToolResult creates a compact, safe-to-log summary of a tool result
func summarizeCallToolResult(result *mcp.CallToolResult) any {
	if result == nil {
		return nil
	}
	if len(result.Content) == 0 {
		return map[string]any{"info": "no content"}
	}
	content := result.Content[0]
	switch typed := content.(type) {
	case mcp.TextContent:
		text := typed.Text
		const limit = 500
		if len(text) > limit {
			text = text[:limit] + "…(truncated)"
		}
		return map[string]any{"type": "text", "text": text}
	case mcp.ImageContent:
		return map[string]any{"type": "image", "mimeType": typed.MIMEType, "bytes": len(typed.Data)}
	default:
		return map[string]any{"type": "unknown"}
	}
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
	const pollInterval = 100 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
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
