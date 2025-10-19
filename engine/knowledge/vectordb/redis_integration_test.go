package vectordb

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisStore_UpsertSearchAndDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)

	dsn := startRedisTestInstance(ctx, t)
	cfg := &Config{
		ID:         "redis_vectors",
		Provider:   ProviderRedis,
		DSN:        dsn,
		Collection: "kb_vectors",
		Dimension:  4,
		MaxTopK:    10,
	}
	store, err := newRedisStore(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close(ctx))
	})

	records := []Record{
		{
			ID:        "doc-1",
			Text:      "alpha document",
			Embedding: []float32{1, 0, 0, 0},
			Metadata: map[string]any{
				"knowledge_base_id": "kb1",
				"lang":              "en",
			},
		},
		{
			ID:        "doc-2",
			Text:      "beta document",
			Embedding: []float32{0, 1, 0, 0},
			Metadata: map[string]any{
				"knowledge_base_id": "kb2",
				"lang":              "es",
			},
		},
	}
	query := []float32{1, 0, 0, 0}
	var skipVectorTests bool

	t.Run("Upsert", func(t *testing.T) {
		err := store.Upsert(ctx, records)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unknown command") {
				skipVectorTests = true
				t.Skipf("vector sets not available in Redis server: %v", err)
			}
			require.NoError(t, err)
		}
	})

	if skipVectorTests {
		t.Skip("vector sets not available in Redis server")
	}

	t.Run("Search", func(t *testing.T) {
		matches, err := store.Search(ctx, query, SearchOptions{TopK: 2})
		require.NoError(t, err)
		require.NotEmpty(t, matches)
		assert.Equal(t, "doc-1", matches[0].ID)
		assert.Equal(t, "alpha document", matches[0].Text)
		assert.Equal(t, "kb1", matches[0].Metadata["knowledge_base_id"])
	})

	t.Run("FilteredSearch", func(t *testing.T) {
		filtered, err := store.Search(ctx, query, SearchOptions{
			TopK:    2,
			Filters: map[string]string{"lang": "en"},
		})
		require.NoError(t, err)
		require.Len(t, filtered, 1)
		assert.Equal(t, "doc-1", filtered[0].ID)
	})

	t.Run("DeleteByMetadata", func(t *testing.T) {
		require.NoError(t, store.Delete(ctx, Filter{Metadata: map[string]string{"knowledge_base_id": "kb1"}}))
		afterDelete, err := store.Search(ctx, query, SearchOptions{TopK: 2})
		require.NoError(t, err)
		require.Len(t, afterDelete, 1)
		assert.Equal(t, "doc-2", afterDelete[0].ID)
	})

	t.Run("DeleteByIDs", func(t *testing.T) {
		require.NoError(t, store.Delete(ctx, Filter{IDs: []string{"doc-2"}}))
		finalMatches, err := store.Search(ctx, query, SearchOptions{TopK: 2})
		require.NoError(t, err)
		assert.Empty(t, finalMatches)
	})
}

func startRedisTestInstance(ctx context.Context, t *testing.T) string {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "redis/redis-stack-server:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(2 * time.Minute),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cleanupCancel()
		_ = container.Terminate(cleanupCtx)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	return fmt.Sprintf("redis://%s:%s", host, port.Port())
}
