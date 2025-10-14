package config

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLProvider_MultipleWatchCalls(t *testing.T) {
	t.Run("Should handle multiple Watch() calls correctly", func(t *testing.T) {
		// Create temp YAML file with initial content
		tmpFile, err := os.CreateTemp("", "test-multiple-watch-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write initial content
		initialContent := []byte("initial: content")
		err = os.WriteFile(tmpFile.Name(), initialContent, 0644)
		require.NoError(t, err)

		// Create provider
		provider := NewYAMLProvider(tmpFile.Name())

		ctx := t.Context()

		// Create a WaitGroup to coordinate file modification
		var wg sync.WaitGroup
		wg.Add(2) // Expecting 2 callbacks

		// Track callback invocations
		var callbackCount int32

		var firstCallbackOnce sync.Once
		// Register first callback
		err = provider.Watch(ctx, func() {
			firstCallbackOnce.Do(func() {
				atomic.AddInt32(&callbackCount, 1)
				wg.Done()
			})
		})
		require.NoError(t, err)

		var secondCallbackOnce sync.Once
		// Register second callback (should not start watching again)
		err = provider.Watch(ctx, func() {
			secondCallbackOnce.Do(func() {
				atomic.AddInt32(&callbackCount, 10)
				wg.Done()
			})
		})
		require.NoError(t, err)

		// Use a goroutine to modify the file after a short delay
		// This ensures the watcher has time to initialize
		go func() {
			// Use a very short delay just to ensure goroutine scheduling
			<-time.After(10 * time.Millisecond)

			// Modify file to trigger callbacks
			if writeErr := os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644); writeErr != nil {
				t.Errorf("Failed to write file: %v", writeErr)
			}
		}()

		// Wait for callbacks with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Both callbacks completed
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for callbacks")
		}

		// Both callbacks should have been invoked
		count := atomic.LoadInt32(&callbackCount)
		assert.Equal(t, int32(11), count, "Expected both callbacks to be invoked (1 + 10 = 11)")
	})
}
