package vectordb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_PgvectorDSN(t *testing.T) {
	t.Run("Should allow empty DSN for pgvector provider", func(t *testing.T) {
		cfg := &Config{
			ID:        "test-pg",
			Provider:  ProviderPGVector,
			DSN:       "",
			Dimension: 384,
		}
		err := validateConfig(cfg)
		assert.NoError(t, err, "pgvector allows empty DSN as it can fall back to global config")
	})

	t.Run("Should accept explicit DSN for pgvector provider", func(t *testing.T) {
		cfg := &Config{
			ID:        "test-pg",
			Provider:  ProviderPGVector,
			DSN:       "postgresql://localhost:5432/db",
			Dimension: 384,
		}
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("Should require DSN for qdrant provider", func(t *testing.T) {
		cfg := &Config{
			ID:        "test-qdrant",
			Provider:  ProviderQdrant,
			DSN:       "",
			Dimension: 384,
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "dsn is required")
	})

	t.Run("Should require path for filesystem provider", func(t *testing.T) {
		cfg := &Config{
			ID:        "test-fs",
			Provider:  ProviderFilesystem,
			Dimension: 128,
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("Should reject invalid dimension", func(t *testing.T) {
		cfg := &Config{
			ID:        "test-pg",
			Provider:  ProviderPGVector,
			DSN:       "postgresql://localhost:5432/db",
			Dimension: 0,
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "dimension must be greater than zero")
	})
}

func TestFileStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	cfg := &Config{
		ID:        "fs",
		Provider:  ProviderFilesystem,
		Path:      path,
		Dimension: 2,
	}
	store, err := newFileStore(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close(context.Background()))
	})
	records := []Record{
		{ID: "chunk-1", Text: "hello world", Embedding: []float32{1, 0}, Metadata: map[string]any{"kb": "demo"}},
	}
	require.NoError(t, store.Upsert(context.Background(), records))
	reloaded, err := newFileStore(cfg)
	require.NoError(t, err)
	defer reloaded.Close(context.Background())
	matches, err := reloaded.Search(context.Background(), []float32{1, 0}, SearchOptions{TopK: 1})
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "chunk-1", matches[0].ID)
}
