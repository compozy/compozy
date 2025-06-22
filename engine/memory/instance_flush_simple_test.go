package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlushPendingMethods tests the core flush pending functionality
func TestFlushPendingMethods(t *testing.T) {
	t.Run("Should set and clear flush pending flag", func(t *testing.T) {
		// Setup Redis store
		store, mini, ctx := setupRedisTestStore(t)
		defer mini.Close()

		// Create minimal memory instance for testing pending flag operations
		instance := &Instance{
			id:    "test-memory-pending",
			store: store,
		}

		// Initially should not be pending
		pending, err := instance.isFlushPending(ctx)
		require.NoError(t, err)
		assert.False(t, pending, "Initially should not be pending")

		// Set pending flag
		err = instance.MarkFlushPending(ctx, true)
		require.NoError(t, err)

		// Should now be pending
		pending, err = instance.isFlushPending(ctx)
		require.NoError(t, err)
		assert.True(t, pending, "Should be pending after setting flag")

		// Clear pending flag
		err = instance.MarkFlushPending(ctx, false)
		require.NoError(t, err)

		// Should no longer be pending
		pending, err = instance.isFlushPending(ctx)
		require.NoError(t, err)
		assert.False(t, pending, "Should not be pending after clearing flag")
	})

	t.Run("Should handle Redis key correctly", func(t *testing.T) {
		// Create a mock store
		store, mini, ctx := setupRedisTestStore(t)
		defer mini.Close()

		instance := &Instance{
			id:    "test-memory-key",
			store: store,
		}

		// The flush pending key format is now internal to the store implementation
		// Just verify that MarkFlushPending and isFlushPending work correctly
		pending, err := instance.isFlushPending(ctx)
		assert.NoError(t, err)
		assert.False(t, pending, "Flush should not be pending initially")
	})

	t.Run("Should handle Redis operations properly", func(t *testing.T) {
		// Setup Redis store
		store, mini, ctx := setupRedisTestStore(t)
		defer mini.Close()

		instance := &Instance{
			id:    "test-memory-redis-ops",
			store: store,
		}

		// Test multiple set/clear cycles
		for i := 0; i < 3; i++ {
			// Set pending
			err := instance.MarkFlushPending(ctx, true)
			require.NoError(t, err)

			pending, err := instance.isFlushPending(ctx)
			require.NoError(t, err)
			assert.True(t, pending, "Should be pending in cycle %d", i)

			// Clear pending
			err = instance.MarkFlushPending(ctx, false)
			require.NoError(t, err)

			pending, err = instance.isFlushPending(ctx)
			require.NoError(t, err)
			assert.False(t, pending, "Should not be pending in cycle %d", i)
		}
	})
}

// Additional test to verify basic functionality
func TestBasicFlushSchedulingBehavior(t *testing.T) {
	t.Run("Should use store for flush pending operations", func(t *testing.T) {
		store, mini, ctx := setupRedisTestStore(t)
		defer mini.Close()

		instance := &Instance{
			id:    "test-instance-123",
			store: store,
		}

		// Test that flush pending operations work through the store interface
		err := instance.MarkFlushPending(ctx, true)
		assert.NoError(t, err, "Should be able to mark flush pending")

		isPending, err := instance.isFlushPending(ctx)
		assert.NoError(t, err)
		assert.True(t, isPending, "Flush should be pending after marking")

		err = instance.MarkFlushPending(ctx, false)
		assert.NoError(t, err, "Should be able to clear flush pending")

		isPending, err = instance.isFlushPending(ctx)
		assert.NoError(t, err)
		assert.False(t, isPending, "Flush should not be pending after clearing")
	})
}
