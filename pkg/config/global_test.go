package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalConfig(t *testing.T) {
	t.Run("Should panic when accessing uninitialized config", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		assert.Panics(t, func() {
			Get()
		}, "Should panic when getting uninitialized config")

		assert.Panics(t, func() {
			OnChange(func(*Config) {})
		}, "Should panic when registering callback on uninitialized config")

		assert.Panics(t, func() {
			_ = Reload(context.Background())
		}, "Should panic when reloading uninitialized config")
	})

	t.Run("Should initialize global config successfully", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		ctx := context.Background()
		err := Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err, "Should initialize without error")

		// Should be able to get config after initialization
		cfg := Get()
		assert.NotNil(t, cfg, "Should return non-nil config")
		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, 5001, cfg.Server.Port)
	})

	t.Run("Should handle initialization errors", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		// Create a failing source
		failingSource := &mockFailingSource{}

		ctx := context.Background()
		err := Initialize(ctx, nil, failingSource)
		require.Error(t, err, "Should return error from failed initialization")
		assert.Contains(t, err.Error(), "failed to initialize global config")
	})

	t.Run("Should only initialize once", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		ctx := context.Background()

		// First initialization
		err1 := Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err1)

		cfg1 := Get()

		// Second initialization attempt - should be ignored
		err2 := Initialize(ctx, nil, NewCLIProvider(map[string]any{
			"server.port": 9090,
		}))
		require.NoError(t, err2)

		cfg2 := Get()

		// Config should remain unchanged from first initialization
		assert.Equal(t, cfg1.Server.Port, cfg2.Server.Port)
		assert.Equal(t, 5001, cfg2.Server.Port)
	})

	t.Run("Should support callbacks for config changes", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		ctx := context.Background()
		err := Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err)

		// Register callback
		var callbackCalled bool
		OnChange(func(cfg *Config) {
			callbackCalled = true
			assert.NotNil(t, cfg, "Callback should receive non-nil config")
		})

		// Trigger reload
		err = Reload(ctx)
		require.NoError(t, err)

		// Since config hasn't changed, callback shouldn't be called
		assert.False(t, callbackCalled, "Callback should not be called if config hasn't changed")
	})

	t.Run("Should close global config cleanly", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		ctx := context.Background()
		err := Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err)

		// Close should work
		err = Close(ctx)
		assert.NoError(t, err)

		// Second close should be idempotent
		err = Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("Should allow re-initialization after close", func(t *testing.T) {
		// Reset for clean state
		resetForTest()

		ctx := context.Background()

		// Initialize
		err := Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err)

		// Close
		err = Close(ctx)
		require.NoError(t, err)

		// Re-initialize should work after proper reset
		resetForTest()
		err = Initialize(ctx, nil, NewDefaultProvider())
		require.NoError(t, err)

		cfg := Get()
		assert.NotNil(t, cfg)
	})
}

// mockFailingSource is a test source that always fails
type mockFailingSource struct{}

func (m *mockFailingSource) Load() (map[string]any, error) {
	return nil, assert.AnError
}

func (m *mockFailingSource) Watch(_ context.Context, _ func()) error {
	return nil
}

func (m *mockFailingSource) Type() SourceType {
	return "mock"
}

func (m *mockFailingSource) Close() error {
	return nil
}
