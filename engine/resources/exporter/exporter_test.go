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

func TestExportTypeToDir_Filtering(t *testing.T) {
	t.Run("Should export only requested type", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		project := "proj"
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: "writer"},
			map[string]any{"id": "writer"},
		)
		require.NoError(t, err)
		_, err = store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceTask, ID: "compile-report"},
			map[string]any{"id": "compile-report", "type": "basic"},
		)
		require.NoError(t, err)
		_, err = store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceTask, ID: ".."},
			map[string]any{"id": "..", "type": "basic"},
		)
		require.NoError(t, err)
		dir := t.TempDir()
		out, err := ExportTypeToDir(ctx, project, store, dir, resources.ResourceTask)
		require.NoError(t, err)
		require.Equal(t, 2, out.Written[resources.ResourceTask])
		_, err = os.Stat(filepath.Join(dir, "tasks", "compile-report.yaml"))
		require.NoError(t, err)
		filename := filepath.Join(dir, "tasks", "-.yaml")
		_, err = os.Stat(filename)
		require.NoError(t, err)
		content, err := os.ReadFile(filename)
		require.NoError(t, err)
		require.Contains(t, string(content), "id: ..")
		_, err = os.Stat(filepath.Join(dir, "agents"))
		require.True(t, os.IsNotExist(err))
	})
	t.Run("Should export project resources to project directory", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		project := "proj"
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceProject, ID: "main"},
			map[string]any{"id": "main", "name": "Demo"},
		)
		require.NoError(t, err)
		dir := t.TempDir()
		out, err := ExportTypeToDir(ctx, project, store, dir, resources.ResourceProject)
		require.NoError(t, err)
		require.Equal(t, 1, out.Written[resources.ResourceProject])
		_, err = os.Stat(filepath.Join(dir, "project", "main.yaml"))
		require.NoError(t, err)
	})
}
