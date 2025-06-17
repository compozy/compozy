package mcpproxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/mark3labs/mcp-go/mcp"
)

// isNotFoundError checks if an error is a "not found" error from storage
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

// MCPService provides a unified interface for MCP operations, acting as a facade
// over storage, client management, and proxy registration to reduce coupling
type MCPService struct {
	storage       Storage
	clientManager ClientManager
	proxyHandlers *ProxyHandlers
}

// NewMCPService creates a new MCP service with the required dependencies
func NewMCPService(storage Storage, clientManager ClientManager, proxyHandlers *ProxyHandlers) *MCPService {
	return &MCPService{
		storage:       storage,
		clientManager: clientManager,
		proxyHandlers: proxyHandlers,
	}
}

// CreateMCP creates a new MCP definition with coordinated storage, client setup, and proxy registration
func (s *MCPService) CreateMCP(ctx context.Context, def *MCPDefinition) error {
	log := logger.FromContext(ctx)
	// Validate definition
	if err := def.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDefinition, err)
	}
	// Check for existing MCP
	existing, err := s.storage.LoadMCP(ctx, def.Name)
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	if existing != nil {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, def.Name)
	}
	// Set timestamps
	now := time.Now()
	def.CreatedAt = now
	def.UpdatedAt = now
	// Save to storage first
	if err := s.storage.SaveMCP(ctx, def); err != nil {
		return fmt.Errorf("%w: failed to save MCP definition: %v", ErrStorageError, err)
	}
	// Add client to the manager. The manager will handle the connection asynchronously.
	if err := s.clientManager.AddClient(ctx, def); err != nil {
		// Attempt to roll back the storage save on failure.
		if delErr := s.storage.DeleteMCP(ctx, def.Name); delErr != nil {
			log.Debug("Failed to roll back MCP creation from storage", "name", def.Name, "error", delErr)
		}
		return fmt.Errorf("failed to add client to manager: %w", err)
	}
	// Register the proxy. The proxy will wait for the client to connect.
	if s.proxyHandlers != nil {
		if err := s.proxyHandlers.RegisterMCPProxy(ctx, def.Name, def); err != nil {
			// Roll back client addition to maintain consistency
			if removeErr := s.clientManager.RemoveClient(ctx, def.Name); removeErr != nil {
				log.Debug(
					"Failed to roll back client addition after proxy registration failure",
					"name", def.Name, "remove_error", removeErr,
				)
			}
			// Also roll back storage save
			if delErr := s.storage.DeleteMCP(ctx, def.Name); delErr != nil {
				log.Debug(
					"Failed to roll back MCP storage after proxy registration failure",
					"name", def.Name, "delete_error", delErr,
				)
			}
			return fmt.Errorf("%w: %v", ErrProxyRegFailed, err)
		}
	}
	return nil
}

// UpdateMCP updates an existing MCP definition with hot reload
func (s *MCPService) UpdateMCP(ctx context.Context, name string, def *MCPDefinition) (*MCPDefinition, error) {
	// Validate definition
	def.Name = name
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDefinition, err)
	}
	// Check if MCP exists
	existing, err := s.storage.LoadMCP(ctx, name)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	// Update timestamps
	def.UpdatedAt = time.Now()
	def.CreatedAt = existing.CreatedAt
	// Save updated definition first
	if err := s.storage.SaveMCP(ctx, def); err != nil {
		return nil, fmt.Errorf("%w: failed to update MCP definition: %v", ErrStorageError, err)
	}
	// Perform hot reload
	if err := s.performHotReload(ctx, name, def); err != nil {
		return def, fmt.Errorf("%w: %v", ErrHotReloadFailed, err)
	}
	return def, nil
}

// DeleteMCP removes an MCP definition and cleans up associated resources
func (s *MCPService) DeleteMCP(ctx context.Context, name string) error {
	// Check if MCP exists
	_, err := s.storage.LoadMCP(ctx, name)
	if err != nil {
		if isNotFoundError(err) {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	// Remove from storage first
	if err := s.storage.DeleteMCP(ctx, name); err != nil {
		return fmt.Errorf("%w: failed to delete MCP definition: %v", ErrStorageError, err)
	}
	// Clean up runtime components
	s.cleanupRuntimeComponents(ctx, name)
	return nil
}

// ListMCPs returns all MCP definitions with their current status
func (s *MCPService) ListMCPs(ctx context.Context) ([]MCPDetailsResponse, error) {
	mcps, err := s.storage.ListMCPs(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to retrieve MCP definitions: %v", ErrStorageError, err)
	}
	result := make([]MCPDetailsResponse, len(mcps))
	for i, mcp := range mcps {
		status, statusErr := s.clientManager.GetClientStatus(mcp.Name)
		if statusErr != nil {
			status = &MCPStatus{
				Name:   mcp.Name,
				Status: StatusDisconnected,
			}
		}
		result[i] = MCPDetailsResponse{
			Definition: mcp,
			Status:     status,
		}
	}
	return result, nil
}

// GetMCP returns a specific MCP definition with its current status
func (s *MCPService) GetMCP(ctx context.Context, name string) (*MCPDetailsResponse, error) {
	mcp, err := s.storage.LoadMCP(ctx, name)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	status, statusErr := s.clientManager.GetClientStatus(name)
	if statusErr != nil {
		status = &MCPStatus{
			Name:   name,
			Status: StatusDisconnected,
		}
	}
	return &MCPDetailsResponse{
		Definition: mcp,
		Status:     status,
	}, nil
}

// ListAllTools returns all available tools from all registered MCPs
func (s *MCPService) ListAllTools(ctx context.Context) ([]MCPToolDefinition, error) {
	log := logger.FromContext(ctx)
	mcps, err := s.storage.ListMCPs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MCPs: %w", err)
	}
	var allTools []MCPToolDefinition
	for _, mcpDef := range mcps {
		client, err := s.clientManager.GetClient(mcpDef.Name)
		if err != nil {
			log.Warn("Failed to get client for MCP, skipping", "mcp_name", mcpDef.Name, "error", err)
			continue
		}
		tools, err := client.ListTools(ctx)
		if err != nil {
			log.Warn("Failed to list tools for MCP, skipping", "mcp_name", mcpDef.Name, "error", err)
			continue
		}
		for i := range tools {
			tool := &tools[i]
			toolDef := MCPToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: convertToolSchema(tool.InputSchema),
				MCPName:     mcpDef.Name,
			}
			allTools = append(allTools, toolDef)
		}
		log.Debug("Listed tools for MCP", "mcp_name", mcpDef.Name, "tool_count", len(tools))
	}
	return allTools, nil
}

// CallTool executes a tool on a specific MCP server
func (s *MCPService) CallTool(
	ctx context.Context,
	mcpName, toolName string,
	arguments map[string]any,
) (*mcp.CallToolResult, error) {
	log := logger.FromContext(ctx)
	client, err := s.clientManager.GetClient(mcpName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrClientNotConnected, mcpName)
	}
	toolCallReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}
	result, err := client.CallTool(ctx, toolCallReq)
	if err != nil {
		log.Error("Failed to call tool", "mcp_name", mcpName, "tool_name", toolName, "error", err)
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	log.Debug("Tool executed successfully", "mcp_name", mcpName, "tool_name", toolName)
	return result, nil
}

// performHotReload removes old client and adds updated client
func (s *MCPService) performHotReload(ctx context.Context, name string, def *MCPDefinition) error {
	log := logger.FromContext(ctx)
	// Remove existing client and proxy
	if err := s.clientManager.RemoveClient(ctx, name); err != nil {
		log.Debug("Failed to remove client during update", "name", name, "error", err)
	}
	if s.proxyHandlers != nil {
		if err := s.proxyHandlers.UnregisterMCPProxy(ctx, name); err != nil {
			log.Debug("Failed to unregister proxy during update", "name", name, "error", err)
		}
	}

	// Add the updated client - the manager will handle connection asynchronously
	if err := s.clientManager.AddClient(ctx, def); err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	// Register the proxy - it will wait for the client to connect
	if s.proxyHandlers != nil {
		if err := s.proxyHandlers.RegisterMCPProxy(ctx, def.Name, def); err != nil {
			log.Warn("Proxy registration failed but client is being managed", "name", def.Name, "error", err)
		}
	}

	return nil
}

// cleanupRuntimeComponents removes client and proxy registration
func (s *MCPService) cleanupRuntimeComponents(ctx context.Context, name string) {
	log := logger.FromContext(ctx)
	if err := s.clientManager.RemoveClient(ctx, name); err != nil {
		log.Error("Failed to remove client during deletion", "name", name, "error", err)
	}
	if s.proxyHandlers != nil {
		if err := s.proxyHandlers.UnregisterMCPProxy(ctx, name); err != nil {
			log.Error("Failed to unregister proxy during deletion", "name", name, "error", err)
		}
	}
}

// convertToolSchema converts tool input schema to generic map
func convertToolSchema(inputSchema any) map[string]any {
	if inputSchema == nil {
		return nil
	}
	// Try direct type assertion first
	if schema, ok := inputSchema.(map[string]any); ok {
		return schema
	}
	// If that fails, we'll return nil and let the caller handle it
	return nil
}
