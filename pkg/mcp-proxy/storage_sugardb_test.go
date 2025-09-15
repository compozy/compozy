package mcpproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSugarDBStorage_BasicLifecycle(t *testing.T) {
	storage, err := NewSugarDBStorage(context.Background())
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	t.Run("Should save and load MCP definition", func(t *testing.T) {
		def := createTestDefinition("sugar-one")
		require.NoError(t, storage.SaveMCP(ctx, def))

		loaded, err := storage.LoadMCP(ctx, "sugar-one")
		require.NoError(t, err)
		assert.Equal(t, def.Name, loaded.Name)
		assert.Equal(t, def.Transport, loaded.Transport)
	})

	t.Run("Should list definitions via index", func(t *testing.T) {
		// add a second one
		def := createTestDefinition("sugar-two")
		require.NoError(t, storage.SaveMCP(ctx, def))

		list, err := storage.ListMCPs(ctx)
		require.NoError(t, err)
		names := map[string]bool{}
		for _, d := range list {
			names[d.Name] = true
		}
		assert.True(t, names["sugar-one"])
		assert.True(t, names["sugar-two"])
	})

	t.Run("Should save and load status with default fallback", func(t *testing.T) {
		st := NewMCPStatus("sugar-one")
		st.UpdateStatus(StatusConnected, "")
		require.NoError(t, storage.SaveStatus(ctx, st))

		got, err := storage.LoadStatus(ctx, "sugar-one")
		require.NoError(t, err)
		assert.Equal(t, StatusConnected, got.Status)

		// non-existing => default
		defSt, err := storage.LoadStatus(ctx, "missing")
		require.NoError(t, err)
		assert.Equal(t, "missing", defSt.Name)
		assert.Equal(t, StatusDisconnected, defSt.Status)
	})

	t.Run("Should delete definition and cleanup", func(t *testing.T) {
		err := storage.DeleteMCP(ctx, "sugar-two")
		require.NoError(t, err)
		_, err = storage.LoadMCP(ctx, "sugar-two")
		assert.Error(t, err)
	})
}
