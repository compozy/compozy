package services

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
)

// createTestRedisStore creates a Redis store for testing using miniredis
func createTestRedisStore(t *testing.T) ConfigStore {
	// Create miniredis instance for testing
	mr := miniredis.RunT(t)

	// Create Redis configuration for testing
	config := &cache.Config{
		RedisConfig: &config.RedisConfig{
			Host:        mr.Host(),
			Port:        mr.Port(),
			Password:    "", // miniredis doesn't use password
			DB:          0,  // Use default DB for tests
			PingTimeout: 1 * time.Second,
		},
	}

	ctx := t.Context()
	redis, err := cache.NewRedis(ctx, config)
	require.NoError(t, err, "Failed to connect to Redis for testing")

	// Ensure cleanup when test finishes
	t.Cleanup(func() {
		redis.Close()
		mr.Close()
	})

	// Use a short TTL for tests
	return NewRedisConfigStore(redis, 10*time.Minute)
}

func TestRedisConfigStore_SaveAndGet(t *testing.T) {
	t.Run("Should save and retrieve task config", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		// Create test config
		config := &task.Config{}
		config.ID = "test-task"
		config.Type = task.TaskTypeBasic
		config.Action = "test_action"
		with := &core.Input{
			"param1": "value1",
			"param2": float64(42),
		}
		config.With = with

		ctx := t.Context()
		taskExecID := "test-exec-123"

		// Save config
		err := store.Save(ctx, taskExecID, config)
		require.NoError(t, err)

		// Retrieve config
		retrievedConfig, err := store.Get(ctx, taskExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)

		// Verify config matches
		assert.Equal(t, config.ID, retrievedConfig.ID)
		assert.Equal(t, config.Type, retrievedConfig.Type)
		assert.Equal(t, config.Action, retrievedConfig.Action)
		assert.Equal(t, config.With, retrievedConfig.With)
	})

	t.Run("Should return error for non-existent config", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		ctx := t.Context()
		nonExistentID := "non-existent-123"

		// Try to get non-existent config
		config, err := store.Get(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "config not found")
	})

	t.Run("Should validate input parameters", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		ctx := t.Context()
		config := &task.Config{}
		config.ID = "test"

		// Test empty taskExecID
		err := store.Save(ctx, "", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "taskExecID cannot be empty")

		// Test nil config
		err = store.Save(ctx, "valid-id", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")

		// Test empty taskExecID for Get
		_, err = store.Get(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "taskExecID cannot be empty")
	})
}

func TestRedisConfigStore_Delete(t *testing.T) {
	t.Run("Should delete existing config", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		config := &task.Config{}
		config.ID = "test-task"
		ctx := t.Context()
		taskExecID := "test-exec-123"

		// Save config
		err := store.Save(ctx, taskExecID, config)
		require.NoError(t, err)

		// Verify it exists
		_, err = store.Get(ctx, taskExecID)
		require.NoError(t, err)

		// Delete config
		err = store.Delete(ctx, taskExecID)
		require.NoError(t, err)

		// Verify it's deleted
		_, err = store.Get(ctx, taskExecID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config not found")
	})

	t.Run("Should not error when deleting non-existent config", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		ctx := t.Context()
		nonExistentID := "non-existent-123"

		// Delete non-existent config should not error
		err := store.Delete(ctx, nonExistentID)
		assert.NoError(t, err)
	})

	t.Run("Should validate taskExecID parameter", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		ctx := t.Context()

		// Test empty taskExecID
		err := store.Delete(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "taskExecID cannot be empty")
	})
}

func TestRedisConfigStore_TTL(t *testing.T) {
	t.Run("Should apply TTL to stored configs", func(t *testing.T) {
		// Create miniredis instance for testing
		mr := miniredis.RunT(t)
		defer mr.Close()

		// Create store with very short TTL for testing
		config := &cache.Config{
			RedisConfig: &config.RedisConfig{
				Host:        mr.Host(),
				Port:        mr.Port(),
				Password:    "", // miniredis doesn't use password
				DB:          0,  // Use default DB for tests
				PingTimeout: 1 * time.Second,
			},
		}

		ctx := t.Context()
		redis, err := cache.NewRedis(ctx, config)
		require.NoError(t, err)
		defer redis.Close()

		store := NewRedisConfigStore(redis, 2*time.Second) // 2 second TTL
		defer store.Close()

		taskConfig := &task.Config{}
		taskConfig.ID = "test-task"
		taskExecID := "test-exec-ttl"

		// Save config
		err = store.Save(ctx, taskExecID, taskConfig)
		require.NoError(t, err)

		// Verify config exists immediately
		_, err = store.Get(ctx, taskExecID)
		require.NoError(t, err)

		// Fast forward time in miniredis to simulate TTL expiration
		mr.FastForward(3 * time.Second)

		// Verify config has expired
		_, err = store.Get(ctx, taskExecID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config not found")
	})
}

func TestRedisConfigStore_MetadataOperations(t *testing.T) {
	t.Run("Should save and retrieve metadata successfully", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		key := "test-metadata-key"
		data := []byte(`{"test": "data", "count": 42}`)

		// Act - Save metadata
		err := store.SaveMetadata(t.Context(), key, data)
		require.NoError(t, err)

		// Act - Retrieve metadata
		retrievedData, err := store.GetMetadata(t.Context(), key)
		require.NoError(t, err)

		// Assert
		assert.Equal(t, data, retrievedData)
	})

	t.Run("Should return error for non-existent metadata", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		// Act
		_, err := store.GetMetadata(t.Context(), "non-existent-key")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata not found")
	})

	t.Run("Should delete metadata successfully", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		key := "test-metadata-key"
		data := []byte(`{"test": "data"}`)

		// Save metadata first
		err := store.SaveMetadata(t.Context(), key, data)
		require.NoError(t, err)

		// Act - Delete metadata
		err = store.DeleteMetadata(t.Context(), key)
		require.NoError(t, err)

		// Assert - Metadata should no longer exist
		_, err = store.GetMetadata(t.Context(), key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata not found")
	})

	t.Run("Should not interfere with task config storage", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		metadata := []byte(`{"type": "metadata"}`)

		// Act - Store both task config and metadata with similar keys
		err := store.Save(t.Context(), "test-key", taskConfig)
		require.NoError(t, err)

		err = store.SaveMetadata(t.Context(), "test-key", metadata)
		require.NoError(t, err)

		// Assert - Both should be retrievable independently
		retrievedConfig, err := store.Get(t.Context(), "test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-task", retrievedConfig.ID)

		retrievedMetadata, err := store.GetMetadata(t.Context(), "test-key")
		require.NoError(t, err)
		assert.Equal(t, metadata, retrievedMetadata)
	})

	t.Run("Should validate metadata input parameters", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		data := []byte(`{"test": "data"}`)

		// Act & Assert - Empty key
		err := store.SaveMetadata(t.Context(), "", data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")

		// Act & Assert - Nil data
		err = store.SaveMetadata(t.Context(), "test-key", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")

		// Act & Assert - Empty key for get
		_, err = store.GetMetadata(t.Context(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")

		// Act & Assert - Empty key for delete
		err = store.DeleteMetadata(t.Context(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})
}

func TestRedisConfigStore_HealthCheck(t *testing.T) {
	t.Run("Should perform health check successfully", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		// Health check should pass for a working Redis connection
		err := store.(*redisConfigStore).HealthCheck(t.Context())
		assert.NoError(t, err)
	})
}

func TestRedisConfigStore_ExtendTTL(t *testing.T) {
	t.Run("Should extend TTL for existing config", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		taskConfig := &task.Config{}
		taskConfig.ID = "test-task"
		taskExecID := "test-exec-extend-ttl"
		ctx := t.Context()

		// Save config with default TTL
		err := store.Save(ctx, taskExecID, taskConfig)
		require.NoError(t, err)

		// Extend TTL
		redisStore := store.(*redisConfigStore)
		err = redisStore.ExtendTTL(ctx, taskExecID, 1*time.Hour)
		assert.NoError(t, err)

		// Verify TTL was set
		ttl, err := redisStore.GetTTL(ctx, taskExecID)
		assert.NoError(t, err)
		assert.Greater(t, ttl, 59*time.Minute) // Should be close to 1 hour
	})

	t.Run("Should validate taskExecID parameter", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		redisStore := store.(*redisConfigStore)
		err := redisStore.ExtendTTL(t.Context(), "", time.Hour)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "taskExecID cannot be empty")
	})
}

func TestRedisConfigStore_GetKeys(t *testing.T) {
	t.Run("Should retrieve all config keys", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		redisStore := store.(*redisConfigStore)
		ctx := t.Context()

		// Save multiple configs
		configs := []string{"config-1", "config-2", "config-3"}
		for _, configID := range configs {
			config := &task.Config{}
			config.ID = configID
			err := store.Save(ctx, configID, config)
			require.NoError(t, err)
		}

		// Get all config keys
		keys, err := redisStore.GetAllConfigKeys(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), 3) // At least our 3 configs

		// Verify our keys are present
		for _, configID := range configs {
			expectedKey := ConfigKeyPrefix + configID
			assert.Contains(t, keys, expectedKey)
		}
	})

	t.Run("Should retrieve all metadata keys", func(t *testing.T) {
		store := createTestRedisStore(t)
		defer store.Close()

		redisStore := store.(*redisConfigStore)
		ctx := t.Context()

		// Save multiple metadata entries
		metadataKeys := []string{"meta-1", "meta-2", "meta-3"}
		testData := []byte("test-data")
		for _, key := range metadataKeys {
			err := store.SaveMetadata(ctx, key, testData)
			require.NoError(t, err)
		}

		// Get all metadata keys
		keys, err := redisStore.GetAllMetadataKeys(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), 3) // At least our 3 metadata entries

		// Verify our keys are present
		for _, metaKey := range metadataKeys {
			expectedKey := MetadataKeyPrefix + metaKey
			assert.Contains(t, keys, expectedKey)
		}
	})
}

func TestRedisConfigStore_GetExtendsTTL(t *testing.T) {
	t.Run("Should extend TTL on config retrieval", func(t *testing.T) {
		// Create miniredis instance for testing
		mr := miniredis.RunT(t)

		// Create Redis configuration for testing
		config := &cache.Config{
			RedisConfig: &config.RedisConfig{
				Host:        mr.Host(),
				Port:        mr.Port(),
				Password:    "",
				DB:          0,
				PingTimeout: 1 * time.Second,
			},
		}

		ctx := t.Context()
		redis, err := cache.NewRedis(ctx, config)
		require.NoError(t, err, "Failed to connect to Redis for testing")

		store := NewRedisConfigStore(redis, 2*time.Second)
		defer func() {
			store.Close()
			mr.Close()
		}()

		redisStore := store.(*redisConfigStore)
		taskExecID := "test-task-123"

		// Create test config
		taskConfig := &task.Config{}
		taskConfig.ID = "test-task"
		taskConfig.Type = task.TaskTypeBasic
		taskConfig.Action = "test_action"

		// Save config with 2-second TTL
		err = store.Save(t.Context(), taskExecID, taskConfig)
		assert.NoError(t, err)

		// Wait 1 second, then retrieve (should extend TTL)
		time.Sleep(1 * time.Second)
		retrievedConfig, err := store.Get(t.Context(), taskExecID)
		assert.NoError(t, err)
		assert.Equal(t, taskConfig.ID, retrievedConfig.ID)

		// Check that TTL was extended - should be close to 2 seconds again
		ttl, err := redisStore.GetTTL(t.Context(), taskExecID)
		assert.NoError(t, err)
		assert.Greater(t, ttl, 1*time.Second, "TTL should have been extended")
	})

	t.Run("Should extend TTL on metadata retrieval", func(t *testing.T) {
		// Create miniredis instance for testing
		mr := miniredis.RunT(t)

		// Create Redis configuration for testing
		config := &cache.Config{
			RedisConfig: &config.RedisConfig{
				Host:        mr.Host(),
				Port:        mr.Port(),
				Password:    "",
				DB:          0,
				PingTimeout: 1 * time.Second,
			},
		}

		ctx := t.Context()
		redis, err := cache.NewRedis(ctx, config)
		require.NoError(t, err, "Failed to connect to Redis for testing")

		store := NewRedisConfigStore(redis, 2*time.Second)
		defer func() {
			store.Close()
			mr.Close()
		}()

		redisStore := store.(*redisConfigStore)
		key := "test-metadata"
		data := []byte("test data")

		// Save metadata with 2-second TTL
		err = redisStore.SaveMetadata(t.Context(), key, data)
		assert.NoError(t, err)

		// Wait 1 second, then retrieve (should extend TTL)
		time.Sleep(1 * time.Second)
		retrievedData, err := redisStore.GetMetadata(t.Context(), key)
		assert.NoError(t, err)
		assert.Equal(t, data, retrievedData)

		// Check that TTL was extended for metadata
		prefixedKey := "metadata:" + key
		ttl, err := redisStore.redis.TTL(t.Context(), prefixedKey).Result()
		assert.NoError(t, err)
		assert.Greater(t, ttl, 1*time.Second, "Metadata TTL should have been extended")
	})
}
