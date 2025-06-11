package services

import (
	"context"
	"os"
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
		// This test verifies the store can be created with default directory
		// We won't actually create the store to avoid side effects in real directories
		// But we can test that the directory creation logic works

		// Create a temporary directory that we'll use as the base
		tempDir := t.TempDir()

		// Change to temp directory temporarily
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalWd)

		err = os.Chdir(tempDir)
		require.NoError(t, err)

		// Create store with empty directory (should use default)
		store, err := NewBadgerConfigStore("")
		require.NoError(t, err)
		defer store.Close()

		// Verify it created the default directory structure
		_, err = os.Stat(DefaultConfigStoreDir)
		assert.NoError(t, err, "Default directory should be created")
	})
}
