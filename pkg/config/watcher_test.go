package config

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_Creation(t *testing.T) {
	t.Run("Should create new watcher successfully", func(t *testing.T) {
		watcher, err := NewWatcher()
		require.NoError(t, err)
		require.NotNil(t, watcher)
		require.NoError(t, watcher.Close())
	})
}

func TestWatcher_Watch(t *testing.T) {
	t.Run("Should handle absolute file paths", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create watcher
		watcher, err := NewWatcher()
		require.NoError(t, err)
		defer watcher.Close()

		// Should handle absolute path
		ctx := t.Context()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)
	})

	t.Run("Should stop watching on context cancellation", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create watcher
		watcher, err := NewWatcher()
		require.NoError(t, err)
		defer watcher.Close()

		// Track callbacks (avoid data races)
		var callbackInvoked atomic.Bool
		watcher.OnChange(func() {
			callbackInvoked.Store(true)
		})

		// Start watching with cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Cancel context
		cancel()

		// Modify file after context canceled
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Verify no callback is invoked post-cancel within a grace period
		assert.Never(t, callbackInvoked.Load, 300*time.Millisecond, 10*time.Millisecond)
	})
}

func TestWatcher_Close(t *testing.T) {
	t.Run("Should close watcher gracefully", func(t *testing.T) {
		watcher, err := NewWatcher()
		require.NoError(t, err)

		err = watcher.Close()
		assert.NoError(t, err)
	})

	t.Run("Should wait for event handler to finish", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create watcher
		watcher, err := NewWatcher()
		require.NoError(t, err)

		// Start watching
		ctx := t.Context()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Close should complete without hanging
		done := make(chan bool)
		go func() {
			err := watcher.Close()
			assert.NoError(t, err)
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for close")
		}
	})
}
