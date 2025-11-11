package compozy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	engineproject "github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingStore struct {
	resources.ResourceStore
}

func (f *failingStore) Put(_ context.Context, _ resources.ResourceKey, _ any) (resources.ETag, error) {
	return "", fmt.Errorf("forced failure")
}

func TestEngineCleanupUtilities(t *testing.T) {
	t.Parallel()
	t.Run("Should join mode cleanup errors", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		called := 0
		engine.modeCleanups = []modeCleanup{
			func(context.Context) error {
				called++
				return nil
			},
			func(context.Context) error {
				called++
				return errors.New("cleanup failure")
			},
		}
		err := engine.cleanupModeResources(ctx)
		require.Error(t, err)
		assert.Equal(t, 2, called)
	})
	t.Run("Should close resource store safely", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		store := resources.NewMemoryResourceStore()
		require.NotPanics(t, func() { engine.cleanupStore(ctx, store) })
	})
	t.Run("Should fail tool registration when store errors", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{ctx: ctx}
		engine.project = &engineproject.Config{Name: "cleanup"}
		engine.resourceStore = &failingStore{ResourceStore: resources.NewMemoryResourceStore()}
		require.Error(t, engine.RegisterTool(&enginetool.Config{ID: "cleanup-tool"}))
		assert.Empty(t, engine.tools)
	})
}
