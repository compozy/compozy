package exporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestExportToDir_Deterministic(t *testing.T) {
	t.Run("Should export stable YAML with sorted keys", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		project := "testproj"
		// Seed store with a couple of resources (unordered maps)
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: "writer"},
			map[string]any{
				"type": "agent",
				"id":   "writer",
				"config": map[string]any{
					"b": 2,
					"a": 1,
				},
			},
		)
		require.NoError(t, err)
		_, err = store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceTool, ID: "fmt"},
			map[string]any{
				"type": "tool",
				"id":   "fmt",
				"args": []any{"z", "a"},
			},
		)
		require.NoError(t, err)

		dir1 := t.TempDir()
		dir2 := t.TempDir()
		_, err = ExportToDir(ctx, project, store, dir1)
		require.NoError(t, err)
		_, err = ExportToDir(ctx, project, store, dir2)
		require.NoError(t, err)

		// Compare bytes of a sample file across exports
		f1 := filepath.Join(dir1, "agents", "writer.yaml")
		f2 := filepath.Join(dir2, "agents", "writer.yaml")
		b1, err := os.ReadFile(f1)
		require.NoError(t, err)
		b2, err := os.ReadFile(f2)
		require.NoError(t, err)
		require.Equal(t, string(b1), string(b2))
	})
}
