package mcpproxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
)

var once sync.Once

func initLogger() {
	once.Do(func() {
		if err := logger.InitForTests(); err != nil {
			// Log the error but don't fail test initialization
			fmt.Printf("Warning: failed to initialize logger for tests: %v\n", err)
		}
	})
}

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
	initLogger()
	storage := NewMemoryStorage()
	clientManager := NewMCPClientManager(storage, nil)
	return NewServer(config, storage, clientManager)
}

// MockClientManager implements the basic interface needed for testing without actual connections
type MockClientManager struct {
	clients map[string]*MCPStatus
	mu      sync.RWMutex
}

func NewMockClientManager() *MockClientManager {
	return &MockClientManager{
		clients: make(map[string]*MCPStatus),
	}
}

func (m *MockClientManager) AddClient(_ context.Context, def *MCPDefinition) error {
	// Simulate successful add without actual connection
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[def.Name] = &MCPStatus{
		Name:   def.Name,
		Status: StatusConnected,
	}
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
	if status, exists := m.clients[name]; exists {
		// Return a copy to prevent external modification
		return status.SafeCopy(), nil
	}
	return &MCPStatus{Name: name, Status: StatusDisconnected}, nil
}

func (m *MockClientManager) GetClient(_ string) (*MCPClient, error) {
	// For testing, return a nil client - proxy handlers should handle this gracefully
	return nil, fmt.Errorf("mock client manager: client not found")
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
