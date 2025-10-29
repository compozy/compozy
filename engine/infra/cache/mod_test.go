package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestSetupCache_ModeAware(t *testing.T) {
	t.Run("Should create miniredis in standalone mode", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		ctx = config.ContextWithManager(ctx, mgr)
		cfg := mgr.Get()
		cfg.Mode = "distributed"      // global mode
		cfg.Redis.Mode = "standalone" // component override

		cache, cleanup, err := SetupCache(ctx)
		require.NoError(t, err)
		require.NotNil(t, cleanup)
		t.Cleanup(cleanup)
		assert.NotNil(t, cache)
		assert.NotNil(t, cache.Redis)
		// simple operation
		err = cache.Redis.Set(ctx, "test-key", "test-value", 0).Err()
		assert.NoError(t, err)
	})

	t.Run("Should handle Redis connection errors in distributed mode", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		ctx = config.ContextWithManager(ctx, mgr)
		cfg := mgr.Get()
		cfg.Mode = "distributed"
		cfg.Redis.Mode = "distributed"
		cfg.Redis.URL = "redis://invalid:0"
		_, _, err = SetupCache(ctx)
		assert.Error(t, err)
	})
}
