package compozy

import (
	"testing"

	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeStandaloneTemporalConfig(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	override := &StandaloneTemporalConfig{
		DatabaseFile: ":memory:",
		Namespace:    "standalone-test",
		EnableUI:     false,
		LogLevel:     "debug",
	}
	result := mergeStandaloneTemporalConfig(cfg, override)
	require.NotNil(t, result)
	assert.Equal(t, ":memory:", result.DatabaseFile)
	assert.Equal(t, "standalone-test", result.Namespace)
	assert.False(t, result.EnableUI)
	assert.Equal(t, "debug", result.LogLevel)
	assert.Equal(t, ":memory:", cfg.Temporal.Standalone.DatabaseFile)
}

func TestShouldUseStandaloneRedis(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	engine, err := New(ctx, WithWorkflow(&engineworkflow.Config{ID: "mode-check"}))
	require.NoError(t, err)
	assert.False(t, engine.shouldUseStandaloneRedis(cfg))
	engine.standaloneRedis = &StandaloneRedisConfig{}
	assert.True(t, engine.shouldUseStandaloneRedis(cfg))
	cfg.Redis.Mode = string(ModeStandalone)
	engine.standaloneRedis = nil
	assert.True(t, engine.shouldUseStandaloneRedis(cfg))
}
