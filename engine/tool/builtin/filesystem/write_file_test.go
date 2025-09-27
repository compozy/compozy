package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileHandler(t *testing.T) {
	t.Run("Should write file and create parent directories", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		payload := map[string]any{
			"path":    "nested/dir/file.txt",
			"content": "hello",
		}
		output, errResult := callHandler(ctx, t, WriteFileDefinition().Handler, payload)
		require.Nil(t, errResult)
		assert.Equal(t, true, output["success"])
		written, err := os.ReadFile(filepath.Join(root, "nested", "dir", "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(written))
	})

	t.Run("Should append to existing file", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "log.txt")
		require.NoError(t, os.WriteFile(path, []byte("start"), 0o644))
		payload := map[string]any{
			"path":    "log.txt",
			"content": "-more",
			"append":  true,
		}
		_, errResult := callHandler(ctx, t, WriteFileDefinition().Handler, payload)
		require.Nil(t, errResult)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "start-more", string(content))
	})

	t.Run("Should reject payload exceeding size limit", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		large := strings.Repeat("a", int(defaultMaxFileBytes)+1)
		payload := map[string]any{
			"path":    "large.txt",
			"content": large,
		}
		_, errResult := callHandler(ctx, t, WriteFileDefinition().Handler, payload)
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})
}
