package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileHandler(t *testing.T) {
	t.Run("Should read UTF-8 file inside sandbox", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "example.txt")
		require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))
		output, errResult := callHandler(ctx, t, ReadFileDefinition().Handler, map[string]any{"path": "example.txt"})
		require.Nil(t, errResult)
		assert.Equal(t, "hello world", output["content"])
		metadata := output["metadata"].(map[string]any)
		assert.Equal(t, "/example.txt", metadata["path"])
	})

	t.Run("Should reject binary file", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		binary := []byte{0x00, 0x01, 0x02, 0x03}
		require.NoError(t, os.WriteFile(filepath.Join(root, "bin.dat"), binary, 0o644))
		_, errResult := callHandler(ctx, t, ReadFileDefinition().Handler, map[string]any{"path": "bin.dat"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})

	t.Run("Should return not found", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		_, errResult := callHandler(ctx, t, ReadFileDefinition().Handler, map[string]any{"path": "missing.txt"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeFileNotFound, errResult.Code)
	})

	t.Run("Should deny symbolic link targets", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "outside.txt")
		require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))
		symlink := filepath.Join(root, "link.txt")
		require.NoError(t, os.Symlink(target, symlink))
		ctx := testContext(t, root)
		_, errResult := callHandler(ctx, t, ReadFileDefinition().Handler, map[string]any{"path": "link.txt"})
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodePermissionDenied, errResult.Code)
	})

	t.Run("Should read file from additional root", func(t *testing.T) {
		primary := t.TempDir()
		extra := t.TempDir()
		ctx := testContext(t, primary, extra)
		path := filepath.Join(extra, "extra.txt")
		require.NoError(t, os.WriteFile(path, []byte("from extra"), 0o644))
		output, errResult := callHandler(ctx, t, ReadFileDefinition().Handler, map[string]any{"path": path})
		require.Nil(t, errResult)
		assert.Equal(t, "from extra", output["content"])
		metadata := output["metadata"].(map[string]any)
		assert.Equal(t, filepath.Clean(path), metadata["path"])
	})
}
