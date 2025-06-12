package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	adapter "github.com/i2y/langchaingo-mcp-adapter"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

// Connection represents an active connection to an MCP server
type Connection struct {
	config  *Config
	adapter *adapter.MCPAdapter
	client  client.MCPClient // For any transport
	closed  bool
}

// NewConnection creates a new MCP connection based on the configuration
func NewConnection(config *Config) (*Connection, error) {
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	connection := &Connection{
		config: config.Clone(),
	}
	return connection.initHTTP()
}

// initHTTP initializes an HTTP-based MCP connection
func (c *Connection) initHTTP() (*Connection, error) {
	var mcpClient client.MCPClient
	var err error

	// Choose transport based on configuration
	switch c.config.Transport {
	case TransportSSE:
		mcpClient, err = createSSEHttpClient(c.config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSE client: %w", err)
		}
	case TransportStreamableHTTP:
		mcpClient, err = createStreamableHTTPClient(c.config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to create StreamableHTTP client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", c.config.Transport)
	}

	// Now create adapter after the client is properly initialized
	mcpAdapter, err := adapter.New(mcpClient)
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to create MCP adapter: %w", err)
	}

	c.adapter = mcpAdapter
	c.client = mcpClient
	return c, nil
}

// GetAdapter returns the underlying adapter
func (c *Connection) GetAdapter() *adapter.MCPAdapter {
	return c.adapter
}

// Close closes the MCP connection and cleans up resources
func (c *Connection) Close() error {
	if c.closed {
		return nil
	}
	var closeErr error
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close MCP client: %w", err)
		}
		c.client = nil
	}
	// Mark as closed regardless of any errors
	c.closed = true
	return closeErr
}

// IsClosed returns true if the connection has been closed
func (c *Connection) IsClosed() bool {
	return c.closed
}

// Config returns a copy of the connection configuration
func (c *Connection) Config() *Config {
	return c.config.Clone()
}

func (c *Connection) GetTools() map[string]tools.Tool {
	allTools := make(map[string]tools.Tool)
	adapter := c.GetAdapter()
	if adapter == nil {
		return nil
	}
	tools, err := adapter.Tools()
	if err != nil {
		// Log error but continue with other connections
		fmt.Printf("Warning: failed to get tools from MCP connection %s: %v\n", c.Config().ID, err)
		return nil
	}
	// Convert from langchaingo tools to llms.Tool structs
	for _, tool := range tools {
		allTools[tool.Name()] = tool
	}
	return allTools
}

func (c *Connection) ConvertoToLLMTool(tool tools.Tool) llms.Tool {
	llmTool := llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			// For MCP tools, we'll use a generic object schema since we don't have detailed schema info
			Parameters: map[string]any{
				"type":        "object",
				"description": "Tool parameters",
			},
		},
	}
	return llmTool
}

func createSSEHttpClient(url string) (*client.Client, error) {
	httpTransport, err := transport.NewSSE(url,
		transport.WithHeaders(map[string]string{
			"User-Agent":   "MyApp/1.0",
			"Accept":       "application/json, text/event-stream",
			"Content-Type": "application/json",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE transport: %w", err)
	}
	return client.NewClient(httpTransport), nil
}

func createStreamableHTTPClient(url string) (*client.Client, error) {
	// Basic StreamableHTTP client
	httpTransport, err := transport.NewStreamableHTTP(url,
		// Set timeout
		transport.WithHTTPTimeout(30*time.Second),
		// Set custom headers
		transport.WithHTTPHeaders(map[string]string{
			"User-Agent":       "MyApp/1.0",
			"Accept":           "application/json",
			"Content-Type":     "application/json",
			"X-Custom-Header":  "custom-value",
			"Y-Another-Header": "another-value",
		}),
		// With custom HTTP client
		transport.WithHTTPBasicClient(&http.Client{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create StreamableHTTP transport: %w", err)
	}
	return client.NewClient(httpTransport), nil
}

func InitConnections(_ context.Context, configs []Config) (map[string]*Connection, error) {
	connections := make(map[string]*Connection)
	for i := range configs {
		config := &configs[i]
		connection, err := NewConnection(config)
		if err != nil {
			CloseConnections(connections)
			return nil, fmt.Errorf("failed to create MCP connection for %s: %w", config.ID, err)
		}
		connections[config.ID] = connection
	}
	return connections, nil
}

func CloseConnections(connections map[string]*Connection) {
	for _, connection := range connections {
		if connection != nil && !connection.IsClosed() {
			if err := connection.Close(); err != nil {
				fmt.Printf("Warning: failed to close MCP connection %s: %v\n", connection.Config().ID, err)
			}
		}
	}
}
