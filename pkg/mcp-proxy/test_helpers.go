package mcpproxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/mock"
)

// createTestDefinition creates a test MCP definition that won't hang tests
// Uses a command that exits immediately instead of trying to establish real MCP connections
func createTestDefinition(name string) *MCPDefinition {
	def := &MCPDefinition{
		Name:        name,
		Description: "Test MCP server",
		Transport:   TransportStdio,
		Command:     "true", // Command that exits immediately
		Args:        []string{},
		Env:         map[string]string{"DEBUG": "true"},
	}
	def.SetDefaults()
	return def
}

func newTestServer(config *Config) *Server {
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()
	return NewServer(config, storage, clientManager)
}

// MockMCPClient is a testify mock for MCPClient
type MockMCPClient struct {
	mock.Mock
	definition *MCPDefinition
	status     *MCPStatus
}

// NewMockMCPClient creates a new mock MCP client
func NewMockMCPClient(name string) *MockMCPClient {
	definition := &MCPDefinition{
		Name:        name,
		Description: "Mock MCP client",
		Transport:   TransportStdio,
		Command:     "echo",
		Args:        []string{"hello"},
	}
	definition.SetDefaults()

	return &MockMCPClient{
		definition: definition,
		status:     NewMCPStatus(name),
	}
}

// GetDefinition returns the mock definition
func (m *MockMCPClient) GetDefinition() *MCPDefinition {
	if cloned, err := m.definition.Clone(); err == nil && cloned != nil {
		return cloned
	}
	return m.definition
}

// GetStatus returns the mock status
func (m *MockMCPClient) GetStatus() *MCPStatus {
	args := m.Called()
	if result := args.Get(0); result != nil {
		if status, ok := result.(*MCPStatus); ok {
			return status
		}
	}
	return m.status.SafeCopy()
}

// IsConnected returns whether the mock client is connected
func (m *MockMCPClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// Connect simulates connecting to an MCP server
func (m *MockMCPClient) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Disconnect simulates disconnecting from an MCP server
func (m *MockMCPClient) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// WaitUntilConnected simulates waiting for connection
func (m *MockMCPClient) WaitUntilConnected(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Health simulates a health check
func (m *MockMCPClient) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// ListTools simulates listing tools
func (m *MockMCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return []mcp.Tool{}, args.Error(1)
	}
	if tools, ok := result.([]mcp.Tool); ok {
		return tools, args.Error(1)
	}
	return []mcp.Tool{}, args.Error(1)
}

// CallTool simulates calling a tool
func (m *MockMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := m.Called(ctx, request)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	if callResult, ok := result.(*mcp.CallToolResult); ok {
		return callResult, args.Error(1)
	}
	return nil, args.Error(1)
}

// ListPrompts simulates listing prompts
func (m *MockMCPClient) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return []mcp.Prompt{}, args.Error(1)
	}
	if prompts, ok := result.([]mcp.Prompt); ok {
		return prompts, args.Error(1)
	}
	return []mcp.Prompt{}, args.Error(1)
}

// GetPrompt simulates getting a prompt
func (m *MockMCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := m.Called(ctx, request)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	if promptResult, ok := result.(*mcp.GetPromptResult); ok {
		return promptResult, args.Error(1)
	}
	return nil, args.Error(1)
}

// ListResources simulates listing resources
func (m *MockMCPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return []mcp.Resource{}, args.Error(1)
	}
	if resources, ok := result.([]mcp.Resource); ok {
		return resources, args.Error(1)
	}
	return []mcp.Resource{}, args.Error(1)
}

// ReadResource simulates reading a resource
func (m *MockMCPClient) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	args := m.Called(ctx, request)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	if resourceResult, ok := result.(*mcp.ReadResourceResult); ok {
		return resourceResult, args.Error(1)
	}
	return nil, args.Error(1)
}

// ListResourceTemplates simulates listing resource templates
func (m *MockMCPClient) ListResourceTemplates(ctx context.Context) ([]mcp.ResourceTemplate, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return []mcp.ResourceTemplate{}, args.Error(1)
	}
	if templates, ok := result.([]mcp.ResourceTemplate); ok {
		return templates, args.Error(1)
	}
	return []mcp.ResourceTemplate{}, args.Error(1)
}

// ListPromptsWithCursor simulates listing prompts with cursor
func (m *MockMCPClient) ListPromptsWithCursor(ctx context.Context, cursor string) ([]mcp.Prompt, string, error) {
	args := m.Called(ctx, cursor)
	result := args.Get(0)
	if result == nil {
		return []mcp.Prompt{}, args.String(1), args.Error(2)
	}
	if prompts, ok := result.([]mcp.Prompt); ok {
		return prompts, args.String(1), args.Error(2)
	}
	return []mcp.Prompt{}, args.String(1), args.Error(2)
}

// ListResourcesWithCursor simulates listing resources with cursor
func (m *MockMCPClient) ListResourcesWithCursor(ctx context.Context, cursor string) ([]mcp.Resource, string, error) {
	args := m.Called(ctx, cursor)
	result := args.Get(0)
	if result == nil {
		return []mcp.Resource{}, args.String(1), args.Error(2)
	}
	if resources, ok := result.([]mcp.Resource); ok {
		return resources, args.String(1), args.Error(2)
	}
	return []mcp.Resource{}, args.String(1), args.Error(2)
}

// ListResourceTemplatesWithCursor simulates listing resource templates with cursor
func (m *MockMCPClient) ListResourceTemplatesWithCursor(
	ctx context.Context,
	cursor string,
) ([]mcp.ResourceTemplate, string, error) {
	args := m.Called(ctx, cursor)
	result := args.Get(0)
	if result == nil {
		return []mcp.ResourceTemplate{}, args.String(1), args.Error(2)
	}
	if templates, ok := result.([]mcp.ResourceTemplate); ok {
		return templates, args.String(1), args.Error(2)
	}
	return []mcp.ResourceTemplate{}, args.String(1), args.Error(2)
}

// MockClientManager implements the basic interface needed for testing without actual connections
type MockClientManager struct {
	clients map[string]*MockMCPClient
	mu      sync.RWMutex
}

func NewMockClientManager() *MockClientManager {
	return &MockClientManager{
		clients: make(map[string]*MockMCPClient),
	}
}

func (m *MockClientManager) AddClient(_ context.Context, def *MCPDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mockClient := NewMockMCPClient(def.Name)
	// Set up default mock behavior
	mockClient.On("GetStatus").Return((*MCPStatus)(nil)).Maybe()
	mockClient.On("IsConnected").Return(true).Maybe()
	mockClient.On("WaitUntilConnected", mock.Anything).Return(nil).Maybe()
	mockClient.On("ListTools", mock.Anything).Return([]mcp.Tool{}, nil).Maybe()

	m.clients[def.Name] = mockClient
	return nil
}

func (m *MockClientManager) RemoveClient(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, name)
	return nil
}

func (m *MockClientManager) GetClientStatus(name string) (*MCPStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client, exists := m.clients[name]; exists {
		return client.GetStatus(), nil
	}
	return NewMCPStatus(name), nil
}

func (m *MockClientManager) GetClient(name string) (MCPClientInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mockClient, exists := m.clients[name]; exists {
		return mockClient, nil
	}
	return nil, fmt.Errorf("mock client manager: client '%s' not found", name)
}

func (m *MockClientManager) Start(_ context.Context) error {
	return nil
}

func (m *MockClientManager) Stop(_ context.Context) error {
	return nil
}

func (m *MockClientManager) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]any{
		"total_clients":  len(m.clients),
		"total_requests": 0,
	}
}

// MockClientManagerWithClient is a mock that returns a working mock client
type MockClientManagerWithClient struct {
	*MockClientManager
}

func NewMockClientManagerWithClient() *MockClientManagerWithClient {
	return &MockClientManagerWithClient{
		MockClientManager: NewMockClientManager(),
	}
}
