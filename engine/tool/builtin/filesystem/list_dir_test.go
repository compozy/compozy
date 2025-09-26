package filesystem

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDirHandler(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "one.txt"), []byte("1"), 0o644))
	subDir := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "two.log"), []byte("log"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "three.txt"), []byte("data"), 0o644))
	ctx := testContext(t, root)

	t.Run("Should list directory entries", func(t *testing.T) {
		payload := map[string]any{"path": "."}
		output, errResult := callHandler(ctx, t, ListDirDefinition().Handler, payload)
		require.Nil(t, errResult)
		entries := extractNames(output["entries"])
		sort.Strings(entries)
		assert.Equal(t, []string{"/nested", "/one.txt"}, entries)
	})

	t.Run("Should paginate recursively", func(t *testing.T) {
		payload := map[string]any{"path": ".", "recursive": true, "page_size": 2}
		output, errResult := callHandler(ctx, t, ListDirDefinition().Handler, payload)
		require.Nil(t, errResult)
		entries := extractNames(output["entries"])
		assert.Len(t, entries, 2)
		next := output["next_page_token"].(string)
		payload["page_token"] = next
		output, errResult = callHandler(ctx, t, ListDirDefinition().Handler, payload)
		require.Nil(t, errResult)
		entries = extractNames(output["entries"])
		assert.NotEmpty(t, entries)
	})

	t.Run("Should filter with glob", func(t *testing.T) {
		payload := map[string]any{"path": ".", "recursive": true, "pattern": "**/*.txt"}
		output, errResult := callHandler(ctx, t, ListDirDefinition().Handler, payload)
		require.Nil(t, errResult)
		entries := extractNames(output["entries"])
		sort.Strings(entries)
		assert.Equal(t, []string{"/nested/three.txt", "/one.txt"}, entries)
	})

	t.Run("Should exclude directories when requested", func(t *testing.T) {
		payload := map[string]any{"path": ".", "include_dirs": false}
		output, errResult := callHandler(ctx, t, ListDirDefinition().Handler, payload)
		require.Nil(t, errResult)
		entries := extractNames(output["entries"])
		assert.Equal(t, []string{"/one.txt"}, entries)
	})
}

func extractNames(raw any) []string {
	switch v := raw.(type) {
	case []map[string]any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, item["path"].(string))
		}
		return result
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			m := item.(map[string]any)
			result = append(result, m["path"].(string))
		}
		return result
	default:
		return nil
	}
}
