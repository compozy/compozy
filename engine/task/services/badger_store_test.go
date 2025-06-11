package services

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

func TestBadgerConfigStore_SaveAndGet(t *testing.T) {
	t.Run("Should save and retrieve task config", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
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

		ctx := context.Background()
		taskExecID := "test-exec-123"

		// Save config
		err = store.Save(ctx, taskExecID, config)
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
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		nonExistentID := "non-existent-123"

		// Try to get non-existent config
		config, err := store.Get(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "config not found")
	})

	t.Run("Should validate input parameters", func(t *testing.T) {
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		config := &task.Config{}
		config.ID = "test"

		// Test empty taskExecID
		err = store.Save(ctx, "", config)
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

func TestBadgerConfigStore_Delete(t *testing.T) {
	t.Run("Should delete existing config", func(t *testing.T) {
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		config := &task.Config{}
		config.ID = "test-task"
		ctx := context.Background()
		taskExecID := "test-exec-123"

		// Save config
		err = store.Save(ctx, taskExecID, config)
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
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		nonExistentID := "non-existent-123"

		// Delete non-existent config should not error
		err = store.Delete(ctx, nonExistentID)
		assert.NoError(t, err)
	})

	t.Run("Should validate taskExecID parameter", func(t *testing.T) {
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Test empty taskExecID
		err = store.Delete(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "taskExecID cannot be empty")
	})
}

func TestBadgerConfigStore_Persistence(t *testing.T) {
	t.Run("Should persist data after reopening store", func(t *testing.T) {
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		config := &task.Config{}
		config.ID = "persistent-task"
		config.Type = task.TaskTypeBasic
		config.Action = "test_action"
		ctx := context.Background()
		taskExecID := "persistent-exec-123"

		// Create first store instance and save config
		{
			store, err := NewBadgerConfigStore(storeDir)
			require.NoError(t, err)

			err = store.Save(ctx, taskExecID, config)
			require.NoError(t, err)

			err = store.Close()
			require.NoError(t, err)
		}

		// Create second store instance and verify config is still there
		{
			store, err := NewBadgerConfigStore(storeDir)
			require.NoError(t, err)
			defer store.Close()

			retrievedConfig, err := store.Get(ctx, taskExecID)
			require.NoError(t, err)
			require.NotNil(t, retrievedConfig)

			assert.Equal(t, config.ID, retrievedConfig.ID)
			assert.Equal(t, config.Type, retrievedConfig.Type)
			assert.Equal(t, config.Action, retrievedConfig.Action)
		}
	})
}

func TestBadgerConfigStore_DefaultDirectory(t *testing.T) {
	t.Run("Should use default directory when empty string provided", func(t *testing.T) {
		// Create store with empty directory (should use default location)
		store, err := NewBadgerConfigStore("")
		require.NoError(t, err)
		defer store.Close()

		// The store should be created successfully without errors
		// which means the directory resolution and creation worked
		assert.NotNil(t, store, "Store should be created successfully")

		// Test that we can save and retrieve a config to verify the store works
		testConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-id",
				Type: task.TaskTypeBasic,
			},
		}

		err = store.Save(context.Background(), "test-exec-id", testConfig)
		assert.NoError(t, err, "Should be able to save config")

		retrievedConfig, err := store.Get(context.Background(), "test-exec-id")
		assert.NoError(t, err, "Should be able to retrieve config")
		assert.Equal(t, testConfig.ID, retrievedConfig.ID)
	})
}

func TestBadgerConfigStore_MetadataOperations(t *testing.T) {
	t.Run("Should save and retrieve metadata successfully", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		key := "test-metadata-key"
		data := []byte(`{"test": "data", "count": 42}`)

		// Act - Save metadata
		err = store.SaveMetadata(context.Background(), key, data)
		require.NoError(t, err)

		// Act - Retrieve metadata
		retrievedData, err := store.GetMetadata(context.Background(), key)
		require.NoError(t, err)

		// Assert
		assert.Equal(t, data, retrievedData)
	})

	t.Run("Should return error for non-existent metadata", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		// Act
		_, err = store.GetMetadata(context.Background(), "non-existent-key")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata not found")
	})

	t.Run("Should delete metadata successfully", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		key := "test-metadata-key"
		data := []byte(`{"test": "data"}`)

		// Save metadata first
		err = store.SaveMetadata(context.Background(), key, data)
		require.NoError(t, err)

		// Act - Delete metadata
		err = store.DeleteMetadata(context.Background(), key)
		require.NoError(t, err)

		// Assert - Metadata should no longer exist
		_, err = store.GetMetadata(context.Background(), key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata not found")
	})

	t.Run("Should not interfere with task config storage", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		metadata := []byte(`{"type": "metadata"}`)

		// Act - Store both task config and metadata with similar keys
		err = store.Save(context.Background(), "test-key", taskConfig)
		require.NoError(t, err)

		err = store.SaveMetadata(context.Background(), "test-key", metadata)
		require.NoError(t, err)

		// Assert - Both should be retrievable independently
		retrievedConfig, err := store.Get(context.Background(), "test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-task", retrievedConfig.ID)

		retrievedMetadata, err := store.GetMetadata(context.Background(), "test-key")
		require.NoError(t, err)
		assert.Equal(t, metadata, retrievedMetadata)
	})

	t.Run("Should validate metadata input parameters", func(t *testing.T) {
		// Create temporary directory for test store
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "test_store")

		// Create store
		store, err := NewBadgerConfigStore(storeDir)
		require.NoError(t, err)
		defer store.Close()

		data := []byte(`{"test": "data"}`)

		// Act & Assert - Empty key
		err = store.SaveMetadata(context.Background(), "", data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")

		// Act & Assert - Nil data
		err = store.SaveMetadata(context.Background(), "test-key", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")

		// Act & Assert - Empty key for get
		_, err = store.GetMetadata(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")

		// Act & Assert - Empty key for delete
		err = store.DeleteMetadata(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})
}
