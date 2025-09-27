package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFilesHandler(t *testing.T) {
	t.Run("Should list files within directory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "b.txt"), []byte("text"), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{"dir": "."})
		require.Nil(t, errResult)
		files := output["files"].([]string)
		assert.Equal(t, []string{"a.go", "b.txt"}, files)
	})

	t.Run("Should apply exclusion pattern string", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "keep.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "drop_test.go"), []byte("package main"), 0o644))
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{
			"dir":     ".",
			"exclude": "*_test.go",
		})
		require.Nil(t, errResult)
		files := output["files"].([]string)
		assert.Equal(t, []string{"keep.go"}, files)
	})

	t.Run("Should apply multiple exclusion patterns", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "b_test.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "c.spec.ts"), []byte("export {}"), 0o644))
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{
			"dir":     ".",
			"exclude": []string{"*_test.go", "*.spec.*"},
		})
		require.Nil(t, errResult)
		files := output["files"].([]string)
		assert.Equal(t, []string{"a.go"}, files)
	})

	t.Run("Should honor negated exclusion pattern", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "keep.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "drop.ts"), []byte("export {}"), 0o644))
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{
			"dir":     ".",
			"exclude": "!*.go",
		})
		require.Nil(t, errResult)
		files := output["files"].([]string)
		assert.Equal(t, []string{"keep.go"}, files)
	})

	t.Run("Should expand brace patterns", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "a.jsx"), []byte("export"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "b.tsx"), []byte("export"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "c.ts"), []byte("export"), 0o644))
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{
			"dir":     ".",
			"exclude": "*.{jsx,tsx}",
		})
		require.Nil(t, errResult)
		files := output["files"].([]string)
		assert.Equal(t, []string{"c.ts"}, files)
	})

	t.Run("Should return not found for missing directory", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		_, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{"dir": "missing"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeFileNotFound, errResult.Code)
	})

	t.Run("Should reject file path", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "file.txt")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
		_, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{"dir": "file.txt"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})

	t.Run("Should reject invalid exclude value", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		_, errResult := callHandler(ctx, t, ListFilesDefinition().Handler, map[string]any{
			"dir":     ".",
			"exclude": 123,
		})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})
}
