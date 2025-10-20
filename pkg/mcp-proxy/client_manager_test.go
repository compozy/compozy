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
func setupTestManager(t *testing.T) (*MCPClientManager, *MemoryStorage, *ClientManagerConfig) {
	t.Helper()
	storage := NewMemoryStorage()
	config := DefaultClientManagerConfig()
	manager := NewMCPClientManager(t.Context(), storage, config)
	return manager, storage, config
}

func TestMCPClientManager_New(t *testing.T) {
	t.Run("Should initialize client manager with proper dependencies and state", func(t *testing.T) {
		manager, storage, config := setupTestManager(t)

		assert.NotNil(t, manager)
		assert.Equal(t, storage, manager.storage)
		assert.Equal(t, config, manager.config)
		assert.NotNil(t, manager.clients)
		assert.NotNil(t, manager.reconnecting)
		assert.NotNil(t, manager.ctx)
		assert.NotNil(t, manager.cancel)
	})
}

func TestMCPClientManager_StartStop(t *testing.T) {
	t.Run("Should start and stop manager without errors", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		assert.NoError(t, err)

		err = manager.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestMCPClientManager_AddClient(t *testing.T) {
	t.Run("Should add client successfully and make it available for retrieval", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def := createTestMCPDefinition("test-client")

		err = manager.AddClient(ctx, def)
		assert.NoError(t, err)

		client, err := manager.GetClient("test-client")
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "test-client", client.GetDefinition().Name)
	})

	t.Run("Should reject duplicate client with specific error", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def := createTestMCPDefinition("test-client")

		err = manager.AddClient(ctx, def)
		assert.NoError(t, err)

		err = manager.AddClient(ctx, def)
		assert.ErrorContains(t, err, "already exists")
	})

	t.Run("Should enforce connection limits and reject excess clients", func(t *testing.T) {
		manager, _, config := setupTestManager(t)
		config.MaxConcurrentConnections = 1

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def1 := createTestMCPDefinition("client-1")
		err = manager.AddClient(ctx, def1)
		assert.NoError(t, err)

		def2 := createTestMCPDefinition("client-2")
		err = manager.AddClient(ctx, def2)
		assert.ErrorContains(t, err, "maximum concurrent connections")
	})

	t.Run("Should reject invalid client definition with validation error", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		invalidDef := &MCPDefinition{
			Name: "",
		}

		err = manager.AddClient(ctx, invalidDef)
		assert.ErrorContains(t, err, "invalid definition")
	})
}

func TestMCPClientManager_RemoveClient(t *testing.T) {
	t.Run("Should remove client and make it unavailable for retrieval", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def := createTestMCPDefinition("test-client")

		err = manager.AddClient(ctx, def)
		require.NoError(t, err)

		_, err = manager.GetClient("test-client")
		assert.NoError(t, err)

		err = manager.RemoveClient(ctx, "test-client")
		assert.NoError(t, err)

		_, err = manager.GetClient("test-client")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestMCPClientManager_GetClientStatus(t *testing.T) {
	t.Run("Should return client status with correct name and state", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def := createTestMCPDefinition("test-client")

		err = manager.AddClient(ctx, def)
		require.NoError(t, err)

		status, err := manager.GetClientStatus("test-client")
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "test-client", status.Name)
	})
}

func TestMCPClientManager_ListClientStatuses(t *testing.T) {
	t.Run("Should return all client statuses with correct names", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		def1 := createTestMCPDefinition("client-1")
		def2 := createTestMCPDefinition("client-2")

		err = manager.AddClient(ctx, def1)
		require.NoError(t, err)
		err = manager.AddClient(ctx, def2)
		require.NoError(t, err)

		statuses := manager.ListClientStatuses(ctx)
		assert.Len(t, statuses, 2)
		assert.Contains(t, statuses, "client-1")
		assert.Contains(t, statuses, "client-2")
	})
}

func TestMCPClientManager_GetMetrics(t *testing.T) {
	t.Run("Should track client count metrics accurately", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		metrics := manager.GetMetrics()
		assert.NotNil(t, metrics)
		assert.Equal(t, 0, metrics["total_clients"])

		def := createTestMCPDefinition("test-client")
		err = manager.AddClient(ctx, def)
		require.NoError(t, err)

		metrics = manager.GetMetrics()
		assert.Equal(t, 1, metrics["total_clients"])
	})
}

func TestMCPClientManager_ReconnectionPrevention(t *testing.T) {
	t.Run("Should prevent concurrent reconnection attempts for same client", func(t *testing.T) {
		manager, _, _ := setupTestManager(t)

		def := createTestMCPDefinition("test-client")
		err := manager.AddClient(t.Context(), def)
		require.NoError(t, err)

		_, err = manager.GetClient("test-client")
		require.NoError(t, err)

		var wg sync.WaitGroup
		reconnectionCount := 0
		mu := sync.Mutex{}

		for range 5 {
			wg.Go(func() {
				manager.reconnectMu.Lock()
				if !manager.reconnecting[def.Name] {
					manager.reconnecting[def.Name] = true
					manager.reconnectMu.Unlock()

					mu.Lock()
					reconnectionCount++
					mu.Unlock()

					time.Sleep(100 * time.Millisecond)

					manager.reconnectMu.Lock()
					delete(manager.reconnecting, def.Name)
					manager.reconnectMu.Unlock()
				} else {
					manager.reconnectMu.Unlock()
				}
			})
		}

		wg.Wait()

		mu.Lock()
		assert.Equal(t, 1, reconnectionCount)
		mu.Unlock()
	})
}

func TestMCPClientManager_ConcurrentOperations(t *testing.T) {
	t.Run("Should handle concurrent client operations without data corruption", func(t *testing.T) {
		storage := NewMemoryStorage()
		config := DefaultClientManagerConfig()
		config.MaxConcurrentConnections = 10
		manager := NewMCPClientManager(t.Context(), storage, config)

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop(ctx)

		var wg sync.WaitGroup
		clientCount := 5
		errChan := make(chan error, clientCount)

		for i := range clientCount {
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

		for range 10 {
			wg.Go(func() {
				_ = manager.GetMetrics()
				_ = manager.ListClientStatuses(ctx)
			})
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			assert.NoError(t, err)
		}

		metrics := manager.GetMetrics()
		assert.Equal(t, clientCount, metrics["total_clients"])

		statuses := manager.ListClientStatuses(ctx)
		assert.Len(t, statuses, clientCount)
	})
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
