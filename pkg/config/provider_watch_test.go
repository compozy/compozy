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

func TestYAMLProvider_MultipleWatchCalls(t *testing.T) {
	t.Run("Should handle multiple Watch() calls correctly", func(t *testing.T) {
		// Create temp YAML file
		tmpFile, err := os.CreateTemp("", "test-multiple-watch-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create provider
		provider := NewYAMLProvider(tmpFile.Name())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Track callback invocations
		var callbackCount int32

		// Register first callback
		err = provider.Watch(ctx, func() {
			atomic.AddInt32(&callbackCount, 1)
		})
		require.NoError(t, err)

		// Register second callback (should not start watching again)
		err = provider.Watch(ctx, func() {
			atomic.AddInt32(&callbackCount, 10)
		})
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Modify file to trigger callbacks
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Wait for callbacks
		time.Sleep(200 * time.Millisecond)

		// Both callbacks should have been invoked
		count := atomic.LoadInt32(&callbackCount)
		assert.Equal(t, int32(11), count, "Expected both callbacks to be invoked (1 + 10 = 11)")
	})
}
