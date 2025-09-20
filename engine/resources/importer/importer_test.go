package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/exporter"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestImporter_StrategiesAndRoundTrip(t *testing.T) {
	t.Run("Should honor seed_only and overwrite_conflicts and preserve round-trip", func(t *testing.T) {
		ctx := context.Background()
		project := "proj"
		store := resources.NewMemoryResourceStore()

		// Prepare repo-like directory with YAML files
		repo := t.TempDir()
		writeFile(t, repo, "agents/writer.yaml", "id: writer\ntype: agent\nconfig:\n  a: 1\n  b: 2\n")
		writeFile(t, repo, "tools/fmt.yaml", "id: fmt\ntype: tool\nargs: [\"a\", \"z\"]\n")

		// First import (seed_only) writes both
		out, err := ImportFromDir(ctx, project, store, repo, SeedOnly, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Imported[resources.ResourceAgent])
		require.Equal(t, 1, out.Imported[resources.ResourceTool])

		// Second import (seed_only) skips both
		out, err = ImportFromDir(ctx, project, store, repo, SeedOnly, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Skipped[resources.ResourceAgent])
		require.Equal(t, 1, out.Skipped[resources.ResourceTool])

		// Overwrite conflict for tool only by changing args order
		writeFile(t, repo, "tools/fmt.yaml", "id: fmt\ntype: tool\nargs: [\"z\", \"a\"]\n")
		out, err = ImportFromDir(ctx, project, store, repo, OverwriteConflicts, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Overwritten[resources.ResourceTool])

		// Round-trip: export -> import -> export should be equal
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		_, err = exporter.ExportToDir(ctx, project, store, dir1)
		require.NoError(t, err)
		// clear store and re-import
		store2 := resources.NewMemoryResourceStore()
		_, err = ImportFromDir(ctx, project, store2, repo, SeedOnly, "tester")
		require.NoError(t, err)
		_, err = exporter.ExportToDir(ctx, project, store2, dir2)
		require.NoError(t, err)
		b1, err := os.ReadFile(filepath.Join(dir1, "agents", "writer.yaml"))
		require.NoError(t, err)
		b2, err := os.ReadFile(filepath.Join(dir2, "agents", "writer.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(b1), string(b2))
	})
}
