package compozy

import (
	"context"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/resources"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDistributedConfigRequiresRedis(t *testing.T) {
	cfg := &appconfig.Config{}
	cfg.Temporal.HostPort = "localhost:7233"
	assert.Error(t, validateDistributedConfig(cfg))
}

func TestBootstrapDistributedCreatesRedisStore(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	cfg.Mode = string(ModeDistributed)
	listenCfg := net.ListenConfig{}
	temporalListener, err := listenCfg.Listen(context.WithoutCancel(t.Context()), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer temporalListener.Close()
	cfg.Temporal.HostPort = temporalListener.Addr().String()
	mr := miniredis.NewMiniRedis()
	require.NoError(t, mr.Start())
	defer mr.Close()
	cfg.Redis.URL = "redis://" + mr.Addr()
	cfg.Redis.Mode = string(ModeDistributed)
	engine, err := New(ctx, WithMode(ModeDistributed), WithWorkflow(&engineworkflow.Config{ID: "distributed-store"}))
	require.NoError(t, err)
	store, err := engine.buildResourceStore(ctx, cfg)
	require.NoError(t, err)
	assert.IsType(t, &resources.RedisResourceStore{}, store)
	assert.NoError(t, store.Close())
	assert.NoError(t, engine.cleanupModeResources(ctx))
}

func TestHasRedisConnectionCombinations(t *testing.T) {
	t.Parallel()
	assert.True(t, hasRedisConnection(&appconfig.Config{Redis: appconfig.RedisConfig{URL: "redis://cache"}}))
	assert.False(t, hasRedisConnection(&appconfig.Config{Redis: appconfig.RedisConfig{Host: "localhost"}}))
	assert.True(t, hasRedisConnection(&appconfig.Config{Redis: appconfig.RedisConfig{Host: "localhost", Port: "6379"}}))
}

func TestDescribeRedisEndpointFormatsOutput(t *testing.T) {
	t.Parallel()
	cfg := &appconfig.Config{Redis: appconfig.RedisConfig{URL: "redis://override"}}
	assert.Equal(t, "redis://override", describeRedisEndpoint(cfg))
	cfg = &appconfig.Config{Redis: appconfig.RedisConfig{Host: "cache", Port: "6380"}}
	assert.Equal(t, "cache:6380", describeRedisEndpoint(cfg))
	assert.Equal(t, "", describeRedisEndpoint(&appconfig.Config{}))
}
