package config

import (
	"context"
	"os"
	"sync"
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

		// Track callbacks with WaitGroup for deterministic synchronization
		var mu sync.Mutex
		callbackCount := 0
		var wg sync.WaitGroup
		wg.Add(1) // Expect 1 callback to be invoked
		watcher.OnChange(func() {
			mu.Lock()
			callbackCount++
			mu.Unlock()
			wg.Done() // Signal callback completion
		})

		// Start watching
		ctx := t.Context()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Modify file
		err = os.WriteFile(tmpFile.Name(), []byte("test: value2"), 0644)
		require.NoError(t, err)

		// Wait for callback with timeout for deterministic synchronization
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Callback executed successfully
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for callback")
		}

		// Check callback was invoked at least once (fsnotify can emit duplicates)
		mu.Lock()
		assert.GreaterOrEqual(t, callbackCount, 1)
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

		// Register multiple callbacks with counters and WaitGroup for deterministic synchronization
		var mu sync.Mutex
		callbackCounts := make([]int, 3)
		var wg sync.WaitGroup
		wg.Add(3) // Expect 3 callbacks to be invoked

		for i := range 3 {
			index := i // capture loop variable
			watcher.OnChange(func() {
				mu.Lock()
				callbackCounts[index]++
				mu.Unlock()
				wg.Done() // Signal callback completion
			})
		}

		// Start watching
		ctx := t.Context()

		err = watcher.Watch(ctx, tmpFile.Name())
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Trigger change
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Wait for all callbacks with timeout for deterministic synchronization
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All callbacks executed successfully
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for callbacks")
		}

		// Verify all callbacks were invoked exactly once
		mu.Lock()
		for i, count := range callbackCounts {
			assert.Equal(t, 1, count, "callback %d should have been invoked exactly once", i)
		}
		mu.Unlock()
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
