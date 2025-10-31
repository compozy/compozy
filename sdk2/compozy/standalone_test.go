package compozy

import (
	"testing"

	"github.com/compozy/compozy/engine/resources"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandaloneResourceStoreDefaultsToMemory(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	engine, err := New(ctx, WithWorkflow(&engineworkflow.Config{ID: "standalone-memory"}))
	require.NoError(t, err)
	store, err := engine.buildResourceStore(ctx, cfg)
	require.NoError(t, err)
	assert.IsType(t, &resources.MemoryResourceStore{}, store)
	assert.NoError(t, store.Close())
	assert.NoError(t, engine.cleanupModeResources(ctx))
}

func TestStandaloneResourceStoreUsesRedisWhenConfigured(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	engine, err := New(ctx,
		WithWorkflow(&engineworkflow.Config{ID: "standalone-redis"}),
		WithStandaloneRedis(&StandaloneRedisConfig{Persistence: false}),
	)
	require.NoError(t, err)
	store, err := engine.buildResourceStore(ctx, cfg)
	require.NoError(t, err)
	assert.IsType(t, &resources.RedisResourceStore{}, store)
	assert.NoError(t, store.Close())
	assert.NoError(t, engine.cleanupModeResources(ctx))
}
