package memory

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redisTestClient wraps a redis.Client to implement cache.RedisInterface for integration tests.
type redisTestClient struct {
	*redis.Client
}

func (r *redisTestClient) Pipeline() redis.Pipeliner {
	return r.Client.Pipeline()
}

func (r *redisTestClient) TxPipeline() redis.Pipeliner {
	return r.Client.TxPipeline()
}

func setupTestRedis(t *testing.T) (*redisTestClient, func()) {
	t.Helper()

	s, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	return &redisTestClient{Client: client}, func() {
		_ = client.Close()
		s.Close()
	}
}

func TestRedisMemoryStore_AppendMessage(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should append single message successfully", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello, world!"}
		require.NoError(t, store.AppendMessage(ctx, "test-key", msg))
		messages, err := store.ReadMessages(ctx, "test-key")
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, msg.Role, messages[0].Role)
		assert.Equal(t, msg.Content, messages[0].Content)
	})
}

func TestRedisMemoryStore_AppendMessageWithTokenCount(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should append message and update token count atomically", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello, world!"}
		require.NoError(t, store.AppendMessageWithTokenCount(ctx, "test-key", msg, 10))
		messages, err := store.ReadMessages(ctx, "test-key")
		require.NoError(t, err)
		require.Len(t, messages, 1)
		tokenCount, err := store.GetTokenCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 10, tokenCount)
	})
}

func TestRedisMemoryStore_ReadMessages_EmptyKey(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should return empty slice for non-existent key", func(t *testing.T) {
		messages, err := store.ReadMessages(ctx, "non-existent-key")
		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}

func TestRedisMemoryStore_CountMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should count messages correctly", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		require.NoError(t, store.AppendMessages(ctx, "test-key", messages))
		count, err := store.CountMessages(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestRedisMemoryStore_DeleteMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should delete messages and metadata", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello, world!"}
		require.NoError(t, store.AppendMessageWithTokenCount(ctx, "test-key", msg, 10))
		require.NoError(t, store.DeleteMessages(ctx, "test-key"))
		messages, err := store.ReadMessages(ctx, "test-key")
		require.NoError(t, err)
		assert.Empty(t, messages)
		tokenCount, err := store.GetTokenCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 0, tokenCount)
	})
}

func TestRedisMemoryStore_SetExpiration(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should set TTL for key", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello, world!"}
		require.NoError(t, store.AppendMessage(ctx, "test-key", msg))
		require.NoError(t, store.SetExpiration(ctx, "test-key", 60*time.Second))
		ttl, err := store.GetKeyTTL(ctx, "test-key")
		require.NoError(t, err)
		assert.Greater(t, ttl, 50*time.Second)
		assert.LessOrEqual(t, ttl, 60*time.Second)
	})

	t.Run("Should reject zero or negative TTL", func(t *testing.T) {
		assert.Error(t, store.SetExpiration(ctx, "test-key", 0))
		assert.Error(t, store.SetExpiration(ctx, "test-key", -1*time.Second))
	})
}

func TestRedisMemoryStore_FlushPending(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should handle flush pending flag correctly", func(t *testing.T) {
		pending, err := store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.False(t, pending)
		require.NoError(t, store.MarkFlushPending(ctx, "test-key", true))
		pending, err = store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.True(t, pending)
		require.NoError(t, store.MarkFlushPending(ctx, "test-key", false))
		pending, err = store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.False(t, pending)
	})
}

func TestRedisMemoryStore_ReadPaginated(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := store.NewRedisMemoryStore(client, "")
	ctx := t.Context()

	t.Run("Should support pagination", func(t *testing.T) {
		for i := range 10 {
			msg := llm.Message{Role: llm.MessageRoleUser, Content: fmt.Sprintf("Message %d", i+1)}
			require.NoError(t, store.AppendMessage(ctx, "test-key", msg))
		}
		page1, total, err := store.ReadMessagesPaginated(ctx, "test-key", 0, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		require.Len(t, page1, 3)
		assert.Equal(t, "Message 1", page1[0].Content)
		page2, total, err := store.ReadMessagesPaginated(ctx, "test-key", 3, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		require.Len(t, page2, 3)
		assert.Equal(t, "Message 4", page2[0].Content)
	})
}
