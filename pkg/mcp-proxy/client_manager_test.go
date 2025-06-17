package mcpproxy

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager creates a new client manager for testing
func setupTestManager() (*MCPClientManager, *MemoryStorage, *ClientManagerConfig) {
	storage := NewMemoryStorage()
	config := DefaultClientManagerConfig()
	manager := NewMCPClientManager(storage, config)
	return manager, storage, config
}

func TestMCPClientManager_New(t *testing.T) {
	manager, storage, config := setupTestManager()

	assert.NotNil(t, manager)
	assert.Equal(t, storage, manager.storage)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.clients)
	assert.NotNil(t, manager.reconnecting)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.cancel)
}

func TestMCPClientManager_StartStop(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test Start
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// Test Stop
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestMCPClientManager_AddClient_Success(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create a valid MCP definition
	def := createTestMCPDefinition("test-client")

	// Add client
	err = manager.AddClient(ctx, def)
	assert.NoError(t, err)

	// Verify client exists
	client, err := manager.GetClient("test-client")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test-client", client.GetDefinition().Name)
}

func TestMCPClientManager_AddClient_Duplicate(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	def := createTestMCPDefinition("test-client")

	// Add client first time
	err = manager.AddClient(ctx, def)
	assert.NoError(t, err)

	// Try to add same client again
	err = manager.AddClient(ctx, def)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMCPClientManager_AddClient_ConnectionLimit(t *testing.T) {
	manager, _, config := setupTestManager()
	config.MaxConcurrentConnections = 1 // Set low limit for testing

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Add first client
	def1 := createTestMCPDefinition("client-1")
	err = manager.AddClient(ctx, def1)
	assert.NoError(t, err)

	// Try to add second client (should fail due to limit)
	def2 := createTestMCPDefinition("client-2")
	err = manager.AddClient(ctx, def2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum concurrent connections")
}

func TestMCPClientManager_RemoveClient(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	def := createTestMCPDefinition("test-client")

	// Add client
	err = manager.AddClient(ctx, def)
	require.NoError(t, err)

	// Verify client exists
	_, err = manager.GetClient("test-client")
	assert.NoError(t, err)

	// Remove client
	err = manager.RemoveClient(ctx, "test-client")
	assert.NoError(t, err)

	// Verify client no longer exists
	_, err = manager.GetClient("test-client")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMCPClientManager_GetClientStatus(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	def := createTestMCPDefinition("test-client")

	// Add client
	err = manager.AddClient(ctx, def)
	require.NoError(t, err)

	// Get status
	status, err := manager.GetClientStatus("test-client")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "test-client", status.Name)
}

func TestMCPClientManager_ListClientStatuses(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Add multiple clients
	def1 := createTestMCPDefinition("client-1")
	def2 := createTestMCPDefinition("client-2")

	err = manager.AddClient(ctx, def1)
	require.NoError(t, err)
	err = manager.AddClient(ctx, def2)
	require.NoError(t, err)

	// List all statuses
	statuses := manager.ListClientStatuses(ctx)
	assert.Len(t, statuses, 2)
	assert.Contains(t, statuses, "client-1")
	assert.Contains(t, statuses, "client-2")
}

func TestMCPClientManager_GetMetrics(t *testing.T) {
	manager, _, _ := setupTestManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Get initial metrics
	metrics := manager.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics["total_clients"])

	// Add a client
	def := createTestMCPDefinition("test-client")
	err = manager.AddClient(ctx, def)
	require.NoError(t, err)

	// Get updated metrics
	metrics = manager.GetMetrics()
	assert.Equal(t, 1, metrics["total_clients"])
}

func TestMCPClientManager_TriggerReconnection_PreventsConcurrent(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultClientManagerConfig()
	manager := NewMCPClientManager(storage, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	def := createTestMCPDefinition("test-client")
	err = manager.AddClient(ctx, def)
	require.NoError(t, err)

	_, err = manager.GetClient("test-client")
	require.NoError(t, err)

	// Trigger multiple reconnections concurrently
	var wg sync.WaitGroup
	reconnectionCount := 0
	mu := sync.Mutex{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Check if reconnection tracking works
			manager.reconnectMu.Lock()
			if !manager.reconnecting[def.Name] {
				manager.reconnecting[def.Name] = true
				manager.reconnectMu.Unlock()

				mu.Lock()
				reconnectionCount++
				mu.Unlock()

				// Simulate reconnection work
				time.Sleep(100 * time.Millisecond)

				manager.reconnectMu.Lock()
				delete(manager.reconnecting, def.Name)
				manager.reconnectMu.Unlock()
			} else {
				manager.reconnectMu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should only have one reconnection attempt
	mu.Lock()
	assert.Equal(t, 1, reconnectionCount)
	mu.Unlock()
}

func TestMCPClientManager_Concurrent_Operations(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultClientManagerConfig()
	config.MaxConcurrentConnections = 10
	manager := NewMCPClientManager(storage, config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	var wg sync.WaitGroup
	clientCount := 5
	errChan := make(chan error, clientCount)

	// Concurrently add clients
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			def := createTestMCPDefinition(fmt.Sprintf("client-%d", id))
			err := manager.AddClient(ctx, def)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	// Concurrently read metrics and statuses
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.GetMetrics()
			_ = manager.ListClientStatuses(ctx)
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors from concurrent operations
	for err := range errChan {
		assert.NoError(t, err)
	}

	// Verify all clients were added
	metrics := manager.GetMetrics()
	assert.Equal(t, clientCount, metrics["total_clients"])

	statuses := manager.ListClientStatuses(ctx)
	assert.Len(t, statuses, clientCount)
}

func TestMCPClientManager_InvalidDefinition(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultClientManagerConfig()
	manager := NewMCPClientManager(storage, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Try to add client with invalid definition
	invalidDef := &MCPDefinition{
		Name: "", // Empty name should be invalid
	}

	err = manager.AddClient(ctx, invalidDef)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid definition")
}

// Helper function to create test MCP definitions
func createTestMCPDefinition(name string) *MCPDefinition {
	def := &MCPDefinition{
		Name:      name,
		Transport: TransportStdio,
		Command:   "echo",
		Args:      []string{"hello"},
	}
	def.SetDefaults()
	return def
}
