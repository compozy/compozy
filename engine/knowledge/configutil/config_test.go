package configutil

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	appconfig "github.com/compozy/compozy/pkg/config"
)

func TestToVectorStoreConfig_DefaultFilesystemPath(t *testing.T) {
	t.Run("defaults relative when cwd missing", func(t *testing.T) {
		cfg := &knowledge.VectorDBConfig{
			ID:   "filesystem_demo",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Dimension: 1536,
			},
		}
		storeCfg, err := ToVectorStoreConfig(context.Background(), "demo", cfg)
		require.NoError(t, err)
		expected := filepath.Join(".compozy", "cache", "filesystem_demo.store")
		assert.Equal(t, expected, storeCfg.Path)
	})

	t.Run("resolves with cli cwd", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("COMPOZY_CWD", tmpDir)
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider(), appconfig.NewEnvProvider())
		require.NoError(t, err)
		ctx := appconfig.ContextWithManager(context.Background(), manager)

		cfg := &knowledge.VectorDBConfig{
			ID:   "filesystem_demo",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Dimension: 1536,
			},
		}
		storeCfg, err := ToVectorStoreConfig(ctx, "demo", cfg)
		require.NoError(t, err)
		expected := filepath.Join(tmpDir, ".compozy", "cache", "filesystem_demo.store")
		assert.Equal(t, expected, storeCfg.Path)
	})
}
