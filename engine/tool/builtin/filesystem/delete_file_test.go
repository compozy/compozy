package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteFileHandler(t *testing.T) {
	t.Run("Should delete file", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "delete.txt")
		require.NoError(t, os.WriteFile(path, []byte("data"), 0o644))
		output, errResult := callHandler(ctx, t, DeleteFileDefinition().Handler, map[string]any{"path": "delete.txt"})
		require.Nil(t, errResult)
		assert.Equal(t, true, output["success"])
		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Should require recursive flag for directories", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		require.NoError(t, os.Mkdir(filepath.Join(root, "folder"), 0o755))
		_, errResult := callHandler(ctx, t, DeleteFileDefinition().Handler, map[string]any{"path": "folder"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})

	t.Run("Should delete directories recursively", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		sub := filepath.Join(root, "dir", "nested")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "file.txt"), []byte("x"), 0o644))
		payload := map[string]any{"path": "dir", "recursive": true}
		output, errResult := callHandler(ctx, t, DeleteFileDefinition().Handler, payload)
		require.Nil(t, errResult)
		assert.Equal(t, true, output["success"])
		_, err := os.Stat(filepath.Join(root, "dir"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Should return success false when path missing", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		output, errResult := callHandler(ctx, t, DeleteFileDefinition().Handler, map[string]any{"path": "missing"})
		require.Nil(t, errResult)
		assert.Equal(t, false, output["success"])
	})
}
