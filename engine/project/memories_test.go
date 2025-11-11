package project

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestProject_IndexMemoriesToResourceStore(t *testing.T) {
	t.Run("Should index project memories into the ResourceStore", func(t *testing.T) {
		ctx := t.Context()
		store := resources.NewMemoryResourceStore()

		p := &Config{
			Name: "demo-mem",
			Memories: []*memory.Config{
				{
					ID:   "conversation",
					Type: memcore.BufferMemory,
					Persistence: memcore.PersistenceConfig{
						Type: memcore.InMemoryPersistence,
						// TTL optional for in_memory
					},
				},
			},
		}

		require.NoError(t, p.IndexToResourceStore(ctx, store))

		// Memory retrievable under the correct key
		v, _, err := store.Get(
			ctx,
			resources.ResourceKey{Project: "demo-mem", Type: resources.ResourceMemory, ID: "conversation"},
		)
		require.NoError(t, err)
		require.NotNil(t, v)
	})
}

func TestProject_ValidateMemories(t *testing.T) {
	t.Run("Should validate inline memory config with defaults", func(t *testing.T) {
		cwd, err := core.CWDFromPath(".")
		require.NoError(t, err)
		p := &Config{
			Name: "demo-validate",
			CWD:  cwd,
			Memories: []*memory.Config{
				{
					// Resource is intentionally omitted to verify defaulting
					ID:   "conv",
					Type: memcore.BufferMemory,
					Persistence: memcore.PersistenceConfig{
						Type: memcore.InMemoryPersistence,
					},
				},
			},
		}

		err = p.Validate(t.Context())
		require.NoError(t, err)
	})
}
