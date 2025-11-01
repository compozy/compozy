package compozy

import (
	"testing"
	"time"

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

func TestMergeStandaloneTemporalConfigOverrides(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	override := &StandaloneTemporalConfig{
		DatabaseFile: "custom.db",
		FrontendPort: 7234,
		BindIP:       "0.0.0.0",
		Namespace:    "custom-ns",
		ClusterName:  "custom-cluster",
		EnableUI:     true,
		UIPort:       7443,
		LogLevel:     "debug",
		StartTimeout: 3 * time.Second,
	}
	embeddedCfg := mergeStandaloneTemporalConfig(cfg, override)
	assert.Equal(t, "custom.db", cfg.Temporal.Standalone.DatabaseFile)
	assert.Equal(t, 7234, cfg.Temporal.Standalone.FrontendPort)
	assert.Equal(t, "0.0.0.0", cfg.Temporal.Standalone.BindIP)
	assert.Equal(t, "custom-ns", cfg.Temporal.Standalone.Namespace)
	assert.Equal(t, "custom-cluster", cfg.Temporal.Standalone.ClusterName)
	assert.True(t, cfg.Temporal.Standalone.EnableUI)
	assert.Equal(t, 7443, cfg.Temporal.Standalone.UIPort)
	assert.Equal(t, "debug", cfg.Temporal.Standalone.LogLevel)
	assert.Equal(t, 3*time.Second, cfg.Temporal.Standalone.StartTimeout)
	assert.Equal(t, cfg.Temporal.Standalone.DatabaseFile, embeddedCfg.DatabaseFile)
	assert.Equal(t, cfg.Temporal.Standalone.FrontendPort, embeddedCfg.FrontendPort)
	assert.Equal(t, cfg.Temporal.Standalone.Namespace, embeddedCfg.Namespace)
	assert.Equal(t, cfg.Temporal.Standalone.ClusterName, embeddedCfg.ClusterName)
	assert.Equal(t, cfg.Temporal.Standalone.EnableUI, embeddedCfg.EnableUI)
	assert.Equal(t, cfg.Temporal.Standalone.UIPort, embeddedCfg.UIPort)
	assert.Equal(t, cfg.Temporal.Standalone.LogLevel, embeddedCfg.LogLevel)
	assert.Equal(t, cfg.Temporal.Standalone.StartTimeout, embeddedCfg.StartTimeout)
}

func TestMergeStandaloneRedisConfigOverrides(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	override := &StandaloneRedisConfig{
		Persistence:      true,
		PersistenceDir:   "/tmp/redis-data",
		SnapshotInterval: 5 * time.Minute,
	}
	mergeStandaloneRedisConfig(cfg, override)
	assert.True(t, cfg.Redis.Standalone.Persistence.Enabled)
	assert.Equal(t, "/tmp/redis-data", cfg.Redis.Standalone.Persistence.DataDir)
	assert.Equal(t, 5*time.Minute, cfg.Redis.Standalone.Persistence.SnapshotInterval)
	mergeStandaloneRedisConfig(nil, override)
	mergeStandaloneRedisConfig(cfg, nil)
}

func TestUpdateRedisConfigAppliesConnectionDetails(t *testing.T) {
	cfg := &appconfig.Config{}
	updateRedisConfig(cfg, "127.0.0.1:6379")
	assert.Equal(t, "redis://127.0.0.1:6379", cfg.Redis.URL)
	assert.Equal(t, "127.0.0.1", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	updateRedisConfig(cfg, "")
	assert.Equal(t, "redis://127.0.0.1:6379", cfg.Redis.URL)
}

func TestParseRedisHostPortHandlesMissingPort(t *testing.T) {
	host, port := parseRedisHostPort("cache.local")
	assert.Equal(t, "cache.local", host)
	assert.Equal(t, "", port)
}
