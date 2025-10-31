package compozy

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineLifecycle(t *testing.T) {
	t.Run("Should start and stop standalone engine with memory store", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine, err := New(
			ctx,
			WithWorkflow(&engineworkflow.Config{ID: "workflow-start"}),
			WithHost(loopbackHostname),
			WithPort(0),
		)
		require.NoError(t, err)
		require.NotNil(t, engine)

		err = engine.Start(ctx)
		require.NoError(t, err)
		assert.True(t, engine.IsStarted())

		store := engine.ResourceStore()
		require.IsType(t, &resources.MemoryResourceStore{}, store)
		assert.NotNil(t, engine.Server())
		assert.NotNil(t, engine.Router())
		assert.NotNil(t, engine.Config())
		assert.Equal(t, ModeStandalone, engine.Mode())

		err = engine.Start(ctx)
		require.ErrorIs(t, err, ErrAlreadyStarted)

		require.NoError(t, engine.Stop(ctx))
		assert.False(t, engine.IsStarted())
		assert.Nil(t, engine.Server())
		assert.Nil(t, engine.Router())

		require.NoError(t, engine.Stop(ctx))
		engine.Wait()

		memStore := store.(*resources.MemoryResourceStore)
		_, putErr := memStore.Put(ctx, resources.ResourceKey{
			Project: "test",
			Type:    resources.ResourceWorkflow,
			ID:      "after-stop",
		}, &engineworkflow.Config{ID: "after-stop"})
		require.Error(t, putErr)
		assert.ErrorContains(t, putErr, "store is closed")
	})

	t.Run("Should fail to start distributed mode without external configuration", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		cfg := appconfig.FromContext(ctx)
		require.NotNil(t, cfg)
		cfg.Mode = string(ModeDistributed)
		cfg.Redis.URL = ""
		cfg.Redis.Host = ""
		cfg.Redis.Port = ""
		engine, err := New(ctx, WithMode(ModeDistributed), WithWorkflow(&engineworkflow.Config{ID: "distributed"}))
		require.NoError(t, err)
		err = engine.Start(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "redis")
		assert.False(t, engine.IsStarted())
	})
}

func lifecycleTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	service := appconfig.NewService()
	manager := appconfig.NewManager(ctx, service)
	_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
	require.NoError(t, err)
	ctx = appconfig.ContextWithManager(ctx, manager)
	return ctx
}
