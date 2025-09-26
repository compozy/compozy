package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrepHandler(t *testing.T) {
	t.Run("Should find pattern matches in file", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "notes.txt")
		content := "alpha\nbeta\nalpha beta\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		payload := map[string]any{"path": "notes.txt", "pattern": "alpha"}
		output, errResult := callHandler(ctx, t, GrepDefinition().Handler, payload)
		require.Nil(t, errResult)
		matches := output["matches"].([]map[string]any)
		assert.Len(t, matches, 2)
		item := matches[0]
		assert.Equal(t, "/notes.txt", item["file"])
	})

	t.Run("Should perform case insensitive search", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "data.txt")
		require.NoError(t, os.WriteFile(path, []byte("First\nsecond\n"), 0o644))
		payload := map[string]any{"path": "data.txt", "pattern": "FIRST", "ignore_case": true}
		output, errResult := callHandler(ctx, t, GrepDefinition().Handler, payload)
		require.Nil(t, errResult)
		matches := output["matches"].([]map[string]any)
		assert.Len(t, matches, 1)
	})

	t.Run("Should cap matches at max_results", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		path := filepath.Join(root, "many.txt")
		require.NoError(t, os.WriteFile(path, []byte("a\na\na\n"), 0o644))
		payload := map[string]any{"path": "many.txt", "pattern": "a", "max_results": 2}
		output, errResult := callHandler(ctx, t, GrepDefinition().Handler, payload)
		require.Nil(t, errResult)
		matches := output["matches"].([]map[string]any)
		assert.Len(t, matches, 2)
	})

	t.Run("Should skip binary files", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		require.NoError(t, os.WriteFile(filepath.Join(root, "bin"), []byte{0x00, 0x01}, 0o644))
		payload := map[string]any{"path": "bin", "pattern": "a"}
		output, errResult := callHandler(ctx, t, GrepDefinition().Handler, payload)
		require.Nil(t, errResult)
		matches := output["matches"].([]map[string]any)
		assert.Len(t, matches, 0)
	})

	t.Run("Should enforce files visited limit", func(t *testing.T) {
		root := t.TempDir()
		ctx := testContext(t, root)
		require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
		for i := 0; i < 3; i++ {
			name := filepath.Join(root, "dir", fmt.Sprintf("file%d.txt", i))
			require.NoError(t, os.WriteFile(name, []byte("value"), 0o644))
		}
		payload := map[string]any{"path": "dir", "pattern": "value", "recursive": true, "max_files_visited": 2}
		_, errResult := callHandler(ctx, t, GrepDefinition().Handler, payload)
		require.NotNil(t, errResult)
		assert.Equal(t, builtin.CodeInvalidArgument, errResult.Code)
	})
}
