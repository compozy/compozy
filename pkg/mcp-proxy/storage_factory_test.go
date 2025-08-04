package mcpproxy

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStorageConfig(t *testing.T) {
	t.Run("Should create default Redis storage configuration", func(t *testing.T) {
		config := DefaultStorageConfig()

		assert.Equal(t, StorageTypeRedis, config.Type)
		assert.NotNil(t, config.Redis)
		assert.Equal(t, "localhost:6379", config.Redis.Addr)
	})
}

func TestNewStorage(t *testing.T) {
	t.Run("Should create Redis storage with proper configuration", func(t *testing.T) {
		mr := miniredis.RunT(t)
		defer mr.Close()

		time.Sleep(10 * time.Millisecond)

		config := &StorageConfig{
			Type: StorageTypeRedis,
			Redis: &RedisConfig{
				Addr:         mr.Addr(),
				Password:     "",
				DB:           0,
				PoolSize:     5,
				MinIdleConns: 1,
				MaxRetries:   2,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
			},
		}

		storage, err := NewStorage(config)
		require.NoError(t, err)
		assert.IsType(t, &RedisStorage{}, storage)
		defer storage.Close()
	})

	t.Run("Should create memory storage with proper type", func(t *testing.T) {
		config := &StorageConfig{
			Type: StorageTypeMemory,
		}

		storage, err := NewStorage(config)
		require.NoError(t, err)
		assert.IsType(t, &MemoryStorage{}, storage)
		defer storage.Close()
	})

	t.Run("Should reject nil config with connection error", func(t *testing.T) {
		storage, err := NewStorage(nil)
		assert.ErrorContains(t, err, "failed to connect to Redis")
		assert.Nil(t, storage)
	})

	t.Run("Should reject unsupported storage type with specific error", func(t *testing.T) {
		config := &StorageConfig{
			Type: StorageType("unsupported"),
		}

		storage, err := NewStorage(config)
		assert.ErrorContains(t, err, "unsupported storage type")
		assert.Nil(t, storage)
	})
}

func TestMemoryStorage_SaveMCP(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should save valid MCP definition successfully", func(t *testing.T) {
		def := createTestDefinition("test-server")

		err := storage.SaveMCP(ctx, def)
		assert.NoError(t, err)
	})

	t.Run("Should reject nil definition with validation error", func(t *testing.T) {
		err := storage.SaveMCP(ctx, nil)
		assert.ErrorContains(t, err, "definition cannot be nil")
	})

	t.Run("Should reject invalid definition with validation error", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "",
			Transport: TransportStdio,
		}

		err := storage.SaveMCP(ctx, def)
		assert.ErrorContains(t, err, "invalid definition")
	})
}

func TestMemoryStorage_LoadMCP(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should load existing definition with all fields intact", func(t *testing.T) {
		original := createTestDefinition("test-server")

		err := storage.SaveMCP(ctx, original)
		require.NoError(t, err)

		loaded, err := storage.LoadMCP(ctx, "test-server")
		require.NoError(t, err)

		assert.Equal(t, original.Name, loaded.Name)
		assert.Equal(t, original.Description, loaded.Description)
		assert.Equal(t, original.Transport, loaded.Transport)
		assert.Equal(t, original.Command, loaded.Command)

		assert.NotSame(t, original, loaded)
	})

	t.Run("Should return not found error for non-existing definition", func(t *testing.T) {
		_, err := storage.LoadMCP(ctx, "non-existing")
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("Should reject empty name with validation error", func(t *testing.T) {
		_, err := storage.LoadMCP(ctx, "")
		assert.ErrorContains(t, err, "name cannot be empty")
	})
}

func TestMemoryStorage_DeleteMCP(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should delete existing definition and associated status", func(t *testing.T) {
		def := createTestDefinition("test-server")

		err := storage.SaveMCP(ctx, def)
		require.NoError(t, err)

		status := NewMCPStatus("test-server")
		err = storage.SaveStatus(ctx, status)
		require.NoError(t, err)

		err = storage.DeleteMCP(ctx, "test-server")
		assert.NoError(t, err)

		_, err = storage.LoadMCP(ctx, "test-server")
		assert.ErrorContains(t, err, "not found")

		loadedStatus, err := storage.LoadStatus(ctx, "test-server")
		require.NoError(t, err)
		assert.Equal(t, StatusDisconnected, loadedStatus.Status)
	})

	t.Run("Should return not found error for non-existing definition", func(t *testing.T) {
		err := storage.DeleteMCP(ctx, "non-existing")
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("Should reject empty name with validation error", func(t *testing.T) {
		err := storage.DeleteMCP(ctx, "")
		assert.ErrorContains(t, err, "name cannot be empty")
	})
}

func TestMemoryStorage_ListMCPs(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should return empty list when no definitions exist", func(t *testing.T) {
		definitions, err := storage.ListMCPs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, definitions)
	})

	t.Run("Should return all saved definitions with correct names", func(t *testing.T) {
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

func TestMemoryStorage_SaveStatus(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should save valid status successfully", func(t *testing.T) {
		status := NewMCPStatus("test-server")
		status.UpdateStatus(StatusConnected, "")

		err := storage.SaveStatus(ctx, status)
		assert.NoError(t, err)
	})

	t.Run("Should reject nil status with validation error", func(t *testing.T) {
		err := storage.SaveStatus(ctx, nil)
		assert.ErrorContains(t, err, "status cannot be nil")
	})

	t.Run("Should reject status with empty name", func(t *testing.T) {
		status := &MCPStatus{Name: ""}

		err := storage.SaveStatus(ctx, status)
		assert.ErrorContains(t, err, "status name cannot be empty")
	})
}

func TestMemoryStorage_LoadStatus(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should load existing status with all fields intact", func(t *testing.T) {
		original := NewMCPStatus("test-server")
		original.UpdateStatus(StatusConnected, "")
		original.TotalRequests = 10

		err := storage.SaveStatus(ctx, original)
		require.NoError(t, err)

		loaded, err := storage.LoadStatus(ctx, "test-server")
		require.NoError(t, err)

		assert.Equal(t, original.Name, loaded.Name)
		assert.Equal(t, original.Status, loaded.Status)
		assert.Equal(t, original.TotalRequests, loaded.TotalRequests)

		assert.NotSame(t, original, loaded)
	})

	t.Run("Should return default status for non-existing entry", func(t *testing.T) {
		status, err := storage.LoadStatus(ctx, "non-existing")
		require.NoError(t, err)

		assert.Equal(t, "non-existing", status.Name)
		assert.Equal(t, StatusDisconnected, status.Status)
		assert.Equal(t, int64(0), status.TotalRequests)
	})

	t.Run("Should reject empty name with validation error", func(t *testing.T) {
		_, err := storage.LoadStatus(ctx, "")
		assert.ErrorContains(t, err, "name cannot be empty")
	})
}

func TestMemoryStorage_Close(t *testing.T) {
	t.Run("Should close storage without errors", func(t *testing.T) {
		storage := NewMemoryStorage()
		err := storage.Close()
		assert.NoError(t, err)
	})
}

func TestMemoryStorage_Integration(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	t.Run("Should handle complete CRUD workflow correctly", func(t *testing.T) {
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
