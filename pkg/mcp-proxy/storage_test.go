package mcpproxy

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*RedisStorage, func()) {
	// Create a miniredis instance for testing
	mr := miniredis.RunT(t)

	config := &RedisConfig{
		Addr:         mr.Addr(),
		Password:     "",
		DB:           0,
		PoolSize:     5,
		MinIdleConns: 1,
		MaxRetries:   2,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	storage, err := NewRedisStorage(config)
	require.NoError(t, err)

	cleanup := func() {
		storage.Close()
		mr.Close()
	}

	return storage, cleanup
}

// createTestDefinition is now defined in test_helpers.go

func TestNewRedisStorage(t *testing.T) {
	initLogger(t)
	t.Run("With config", func(t *testing.T) {
		// Create miniredis instance for testing
		mr := miniredis.RunT(t)
		defer mr.Close()

		// Ensure miniredis is ready
		time.Sleep(10 * time.Millisecond)

		config := &RedisConfig{
			Addr:         mr.Addr(),
			Password:     "",
			DB:           0,
			PoolSize:     5,
			MinIdleConns: 1,
			MaxRetries:   2,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		}

		storage, err := NewRedisStorage(config)
		require.NoError(t, err)
		assert.NotNil(t, storage)
		assert.Equal(t, "mcp_proxy", storage.prefix)
		defer storage.Close()
	})

	t.Run("With nil config", func(t *testing.T) {
		// This test should actually test the nil config behavior
		// Since nil config defaults to localhost:6379, we should test that it fails appropriately
		// or skip this test if we don't have a real Redis instance
		t.Skip("Skipping nil config test as it requires real Redis instance")
	})
}

func TestDefaultRedisConfig(t *testing.T) {
	initLogger(t)
	config := DefaultRedisConfig()

	assert.Equal(t, "localhost:6379", config.Addr)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 2, config.MinIdleConns)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.DialTimeout)
	assert.Equal(t, 3*time.Second, config.ReadTimeout)
	assert.Equal(t, 3*time.Second, config.WriteTimeout)
}

func TestRedisStorage_Ping(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	err := storage.Ping(ctx)
	assert.NoError(t, err)
}

func TestRedisStorage_SaveMCP(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Valid definition", func(t *testing.T) {
		def := createTestDefinition("test-server")

		err := storage.SaveMCP(ctx, def)
		assert.NoError(t, err)
	})

	t.Run("Nil definition", func(t *testing.T) {
		err := storage.SaveMCP(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "definition cannot be nil")
	})

	t.Run("Invalid definition", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "", // Invalid: empty name
			Transport: TransportStdio,
		}

		err := storage.SaveMCP(ctx, def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid definition")
	})
}

func TestRedisStorage_LoadMCP(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Existing definition", func(t *testing.T) {
		original := createTestDefinition("test-server")

		// Save first
		err := storage.SaveMCP(ctx, original)
		require.NoError(t, err)

		// Load
		loaded, err := storage.LoadMCP(ctx, "test-server")
		require.NoError(t, err)

		assert.Equal(t, original.Name, loaded.Name)
		assert.Equal(t, original.Description, loaded.Description)
		assert.Equal(t, original.Transport, loaded.Transport)
		assert.Equal(t, original.Command, loaded.Command)
		assert.Equal(t, original.Args, loaded.Args)
		assert.Equal(t, original.Env, loaded.Env)
	})

	t.Run("Non-existing definition", func(t *testing.T) {
		_, err := storage.LoadMCP(ctx, "non-existing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Empty name", func(t *testing.T) {
		_, err := storage.LoadMCP(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}

func TestRedisStorage_DeleteMCP(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Existing definition", func(t *testing.T) {
		def := createTestDefinition("test-server")

		// Save first
		err := storage.SaveMCP(ctx, def)
		require.NoError(t, err)

		// Delete
		err = storage.DeleteMCP(ctx, "test-server")
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.LoadMCP(ctx, "test-server")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Non-existing definition", func(t *testing.T) {
		err := storage.DeleteMCP(ctx, "non-existing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Empty name", func(t *testing.T) {
		err := storage.DeleteMCP(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}

func TestRedisStorage_ListMCPs(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Empty list", func(t *testing.T) {
		definitions, err := storage.ListMCPs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, definitions)
	})

	t.Run("Multiple definitions", func(t *testing.T) {
		// Save multiple definitions
		def1 := createTestDefinition("server-1")
		def2 := createTestDefinition("server-2")
		def3 := createTestDefinition("server-3")

		err := storage.SaveMCP(ctx, def1)
		require.NoError(t, err)
		err = storage.SaveMCP(ctx, def2)
		require.NoError(t, err)
		err = storage.SaveMCP(ctx, def3)
		require.NoError(t, err)

		// List all
		definitions, err := storage.ListMCPs(ctx)
		require.NoError(t, err)
		assert.Len(t, definitions, 3)

		// Check that all names are present
		names := make(map[string]bool)
		for _, def := range definitions {
			names[def.Name] = true
		}
		assert.True(t, names["server-1"])
		assert.True(t, names["server-2"])
		assert.True(t, names["server-3"])
	})
}

func TestRedisStorage_SaveStatus(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Valid status", func(t *testing.T) {
		status := NewMCPStatus("test-server")
		status.UpdateStatus(StatusConnected, "")

		err := storage.SaveStatus(ctx, status)
		assert.NoError(t, err)
	})

	t.Run("Nil status", func(t *testing.T) {
		err := storage.SaveStatus(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status cannot be nil")
	})

	t.Run("Empty name", func(t *testing.T) {
		status := &MCPStatus{Name: ""}

		err := storage.SaveStatus(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status name cannot be empty")
	})
}

func TestRedisStorage_LoadStatus(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Existing status", func(t *testing.T) {
		original := NewMCPStatus("test-server")
		original.UpdateStatus(StatusConnected, "")
		original.RecordRequest(100 * time.Millisecond)

		// Save first
		err := storage.SaveStatus(ctx, original)
		require.NoError(t, err)

		// Load
		loaded, err := storage.LoadStatus(ctx, "test-server")
		require.NoError(t, err)

		assert.Equal(t, original.Name, loaded.Name)
		assert.Equal(t, original.Status, loaded.Status)
		assert.Equal(t, original.TotalRequests, loaded.TotalRequests)
		assert.Equal(t, original.AvgResponseTime, loaded.AvgResponseTime)
	})

	t.Run("Non-existing status", func(t *testing.T) {
		// Should return default status, not error
		status, err := storage.LoadStatus(ctx, "non-existing")
		require.NoError(t, err)

		assert.Equal(t, "non-existing", status.Name)
		assert.Equal(t, StatusDisconnected, status.Status)
		assert.Equal(t, int64(0), status.TotalRequests)
	})

	t.Run("Empty name", func(t *testing.T) {
		_, err := storage.LoadStatus(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}

func TestRedisStorage_KeyMethods(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	t.Run("getMCPKey", func(t *testing.T) {
		key := storage.getMCPKey("test-server")
		assert.Equal(t, "mcp_proxy:mcps:test-server", key)
	})

	t.Run("getStatusKey", func(t *testing.T) {
		key := storage.getStatusKey("test-server")
		assert.Equal(t, "mcp_proxy:status:test-server", key)
	})

	t.Run("ExtractNameFromKey", func(t *testing.T) {
		key := "mcp_proxy:mcps:test-server"
		name := storage.ExtractNameFromKey(key)
		assert.Equal(t, "test-server", name)

		// Test with invalid key
		invalidKey := "invalid:key"
		name = storage.ExtractNameFromKey(invalidKey)
		assert.Equal(t, "", name)
	})
}

func TestRedisStorage_Health(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	err := storage.Health(ctx)
	assert.NoError(t, err)
}

func TestRedisStorage_Stats(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	stats := storage.Stats()
	assert.NotNil(t, stats)
}

func TestRedisStorage_Integration(t *testing.T) {
	initLogger(t)
	storage, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Test complete workflow
	t.Run("Complete workflow", func(t *testing.T) {
		// Create definition
		def := createTestDefinition("integration-test")

		// Save definition
		err := storage.SaveMCP(ctx, def)
		require.NoError(t, err)

		// Create and save status
		status := NewMCPStatus("integration-test")
		status.UpdateStatus(StatusConnected, "")
		err = storage.SaveStatus(ctx, status)
		require.NoError(t, err)

		// Load definition
		loadedDef, err := storage.LoadMCP(ctx, "integration-test")
		require.NoError(t, err)
		assert.Equal(t, def.Name, loadedDef.Name)

		// Load status
		loadedStatus, err := storage.LoadStatus(ctx, "integration-test")
		require.NoError(t, err)
		assert.Equal(t, StatusConnected, loadedStatus.Status)

		// List definitions
		definitions, err := storage.ListMCPs(ctx)
		require.NoError(t, err)
		assert.Len(t, definitions, 1)
		assert.Equal(t, "integration-test", definitions[0].Name)

		// Delete definition (also removes status)
		err = storage.DeleteMCP(ctx, "integration-test")
		require.NoError(t, err)

		// Verify deletion
		_, err = storage.LoadMCP(ctx, "integration-test")
		assert.Error(t, err)

		// Status should return default when not found
		defaultStatus, err := storage.LoadStatus(ctx, "integration-test")
		require.NoError(t, err)
		assert.Equal(t, StatusDisconnected, defaultStatus.Status)
	})
}
