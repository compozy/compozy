package cache

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func newCacheTestContext(t *testing.T) (context.Context, *config.Manager) {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	t.Cleanup(func() { _ = manager.Close(ctx) })
	ctx = config.ContextWithManager(ctx, manager)
	return ctx, manager
}

func TestSetupCache_MemoryMode_DisablesPersistence(t *testing.T) {
	ctx, mgr := newCacheTestContext(t)
	cfg := mgr.Get()
	cfg.Mode = config.ModeMemory
	cfg.Redis.Mode = ""
	cfg.Redis.Standalone.Persistence.Enabled = true
	cfg.Redis.Standalone.Persistence.DataDir = filepath.Join(t.TempDir(), "redis")

	cache, cleanup, err := SetupCache(ctx)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	t.Cleanup(cleanup)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.Redis)
	assert.NotNil(t, cache.embedded)
	assert.Nil(t, cache.embedded.snapshot, "persistence should remain disabled")
	assert.False(t, cfg.Redis.Standalone.Persistence.Enabled)
}

func TestSetupCache_PersistentMode_Defaults(t *testing.T) {
	ctx, mgr := newCacheTestContext(t)
	cfg := mgr.Get()
	cfg.Mode = config.ModePersistent
	cfg.Redis.Mode = ""
	cfg.Redis.Standalone.Persistence.Enabled = false
	cfg.Redis.Standalone.Persistence.DataDir = ""

	cache, cleanup, err := SetupCache(ctx)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	t.Cleanup(cleanup)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.Redis)
	assert.NotNil(t, cache.embedded)
	assert.NotNil(t, cache.embedded.snapshot, "persistence manager should be configured")
	assert.True(t, cfg.Redis.Standalone.Persistence.Enabled)
	assert.Equal(t, defaultPersistenceDataDir, cfg.Redis.Standalone.Persistence.DataDir)
}

func TestSetupCache_PersistentMode_CustomPersistence(t *testing.T) {
	ctx, mgr := newCacheTestContext(t)
	cfg := mgr.Get()
	cfg.Mode = config.ModePersistent
	cfg.Redis.Mode = config.ModePersistent
	customDir := filepath.Join(t.TempDir(), "redis-data")
	cfg.Redis.Standalone.Persistence.Enabled = true
	cfg.Redis.Standalone.Persistence.DataDir = customDir

	cache, cleanup, err := SetupCache(ctx)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	t.Cleanup(cleanup)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.embedded)
	assert.NotNil(t, cache.embedded.snapshot)
	assert.True(t, cfg.Redis.Standalone.Persistence.Enabled)
	assert.Equal(t, customDir, cfg.Redis.Standalone.Persistence.DataDir)
}

func TestSetupCache_DistributedMode_Error(t *testing.T) {
	ctx, mgr := newCacheTestContext(t)
	cfg := mgr.Get()
	cfg.Mode = config.ModeDistributed
	cfg.Redis.Mode = config.ModeDistributed
	cfg.Redis.URL = "redis://invalid:0"
	_, _, err := SetupCache(ctx)
	assert.Error(t, err)
}
