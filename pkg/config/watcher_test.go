package config

import (
	"context"
	"os"
	"sync"
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
	t.Run("Should watch file for changes", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write initial content
		_, err = tmpFile.WriteString("test: value1")
		require.NoError(t, err)
		require.NoError(t, tmpFile.Close())

		// Create watcher
		watcher, err := NewWatcher()
		require.NoError(t, err)
		defer watcher.Close()

		// Track callbacks
		var mu sync.Mutex
		callbackCount := 0
		watcher.OnChange(func() {
			mu.Lock()
			callbackCount++
			mu.Unlock()
		})

		// Start watching
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Modify file
		err = os.WriteFile(tmpFile.Name(), []byte("test: value2"), 0644)
		require.NoError(t, err)

		// Wait for callback
		time.Sleep(200 * time.Millisecond)

		// Check callback was invoked
		mu.Lock()
		assert.Equal(t, 1, callbackCount)
		mu.Unlock()
	})

	t.Run("Should handle multiple callbacks", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create watcher
		watcher, err := NewWatcher()
		require.NoError(t, err)
		defer watcher.Close()

		// Register multiple callbacks
		var wg sync.WaitGroup
		wg.Add(3)

		for i := 0; i < 3; i++ {
			watcher.OnChange(func() {
				wg.Done()
			})
		}

		// Start watching
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Trigger change
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Wait for all callbacks
		done := make(chan bool)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			// Success - all callbacks invoked
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for callbacks")
		}
	})

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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = watcher.Watch(ctx, tmpFile.Name())
		assert.NoError(t, err)
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

		// Track callbacks
		callbackInvoked := false
		watcher.OnChange(func() {
			callbackInvoked = true
		})

		// Start watching with cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Cancel context
		cancel()

		// Give watcher time to stop
		time.Sleep(100 * time.Millisecond)

		// Modify file after context canceled
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Wait a bit
		time.Sleep(200 * time.Millisecond)

		// Callback should not be invoked
		assert.False(t, callbackInvoked)
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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
