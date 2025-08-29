package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/llm"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redisTestClient wraps a redis.Client to implement cache.RedisInterface
type redisTestClient struct {
	*redis.Client
}

func (r *redisTestClient) Pipeline() redis.Pipeliner {
	return r.Client.Pipeline()
}

func setupTestRedis(t *testing.T) (*redisTestClient, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	return &redisTestClient{Client: client}, func() {
		client.Close()
		s.Close()
	}
}

func TestRedisMemoryStore_AppendMessage(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should append single message successfully", func(t *testing.T) {
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Hello, world!",
		}

		err := store.AppendMessage(ctx, "test-key", msg)
		require.NoError(t, err)

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

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should append message and update token count atomically", func(t *testing.T) {
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Hello, world!",
		}

		err := store.AppendMessageWithTokenCount(ctx, "test-key", msg, 10)
		require.NoError(t, err)

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

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should return empty slice for non-existent key", func(t *testing.T) {
		messages, err := store.ReadMessages(ctx, "non-existent-key")
		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}

func TestRedisMemoryStore_CountMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should count messages correctly", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}

		err := store.AppendMessages(ctx, "test-key", messages)
		require.NoError(t, err)

		count, err := store.CountMessages(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestRedisMemoryStore_DeleteMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should delete messages and metadata", func(t *testing.T) {
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Hello, world!",
		}

		err := store.AppendMessageWithTokenCount(ctx, "test-key", msg, 10)
		require.NoError(t, err)

		err = store.DeleteMessages(ctx, "test-key")
		require.NoError(t, err)

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

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should set TTL for key", func(t *testing.T) {
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Hello, world!",
		}

		err := store.AppendMessage(ctx, "test-key", msg)
		require.NoError(t, err)

		err = store.SetExpiration(ctx, "test-key", 60*time.Second)
		require.NoError(t, err)

		ttl, err := store.GetKeyTTL(ctx, "test-key")
		require.NoError(t, err)
		assert.Greater(t, ttl, 50*time.Second)
		assert.LessOrEqual(t, ttl, 60*time.Second)
	})

	t.Run("Should reject zero or negative TTL", func(t *testing.T) {
		err := store.SetExpiration(ctx, "test-key", 0)
		assert.Error(t, err)

		err = store.SetExpiration(ctx, "test-key", -1*time.Second)
		assert.Error(t, err)
	})
}

func TestRedisMemoryStore_FlushPending(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should handle flush pending flag correctly", func(t *testing.T) {
		// Initially should not be pending
		pending, err := store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.False(t, pending)

		// Set pending
		err = store.MarkFlushPending(ctx, "test-key", true)
		require.NoError(t, err)

		pending, err = store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.True(t, pending)

		// Clear pending
		err = store.MarkFlushPending(ctx, "test-key", false)
		require.NoError(t, err)

		pending, err = store.IsFlushPending(ctx, "test-key")
		require.NoError(t, err)
		assert.False(t, pending)
	})
}

func TestRedisMemoryStore_ReplaceMessagesWithMetadata(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should replace all messages and update metadata", func(t *testing.T) {
		// Add initial messages
		initialMessages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Initial message 1"},
			{Role: llm.MessageRoleUser, Content: "Initial message 2"},
		}
		err := store.AppendMessagesWithMetadata(ctx, "test-key", initialMessages, 20)
		require.NoError(t, err)

		// Replace with new messages
		newMessages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "New message 1"},
		}
		err = store.ReplaceMessagesWithMetadata(ctx, "test-key", newMessages, 10)
		require.NoError(t, err)

		// Verify replacement
		messages, err := store.ReadMessages(ctx, "test-key")
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "New message 1", messages[0].Content)

		tokenCount, err := store.GetTokenCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 10, tokenCount)

		messageCount, err := store.GetMessageCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 1, messageCount)
	})
}

func TestRedisMemoryStore_TrimMessagesWithMetadata(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should trim messages and update metadata", func(t *testing.T) {
		// Add multiple messages
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleUser, Content: "Message 2"},
			{Role: llm.MessageRoleUser, Content: "Message 3"},
		}
		err := store.AppendMessagesWithMetadata(ctx, "test-key", messages, 30)
		require.NoError(t, err)

		// Trim to keep only 2 messages
		err = store.TrimMessagesWithMetadata(ctx, "test-key", 2, 20)
		require.NoError(t, err)

		// Verify trimming
		remainingMessages, err := store.ReadMessages(ctx, "test-key")
		require.NoError(t, err)
		require.Len(t, remainingMessages, 2)
		assert.Equal(t, "Message 2", remainingMessages[0].Content)
		assert.Equal(t, "Message 3", remainingMessages[1].Content)

		tokenCount, err := store.GetTokenCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 20, tokenCount)

		messageCount, err := store.GetMessageCount(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, 2, messageCount)
	})
}

func TestRedisMemoryStore_ReadMessagesPaginated(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewRedisMemoryStore(client, "")
	ctx := context.Background()

	t.Run("Should return empty for non-existent key", func(t *testing.T) {
		messages, totalCount, err := store.ReadMessagesPaginated(ctx, "non-existent", 0, 10)
		require.NoError(t, err)
		assert.Empty(t, messages)
		assert.Equal(t, 0, totalCount)
	})

	t.Run("Should paginate messages correctly", func(t *testing.T) {
		// Append 10 messages
		messages := make([]llm.Message, 10)
		for i := range 10 {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: fmt.Sprintf("Message %d", i+1),
			}
		}

		err := store.AppendMessages(ctx, "test-paginate", messages)
		require.NoError(t, err)

		// Test first page (offset 0, limit 3)
		page1, total1, err := store.ReadMessagesPaginated(ctx, "test-paginate", 0, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total1)
		assert.Len(t, page1, 3)
		assert.Equal(t, "Message 1", page1[0].Content)
		assert.Equal(t, "Message 2", page1[1].Content)
		assert.Equal(t, "Message 3", page1[2].Content)

		// Test second page (offset 3, limit 3)
		page2, total2, err := store.ReadMessagesPaginated(ctx, "test-paginate", 3, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total2)
		assert.Len(t, page2, 3)
		assert.Equal(t, "Message 4", page2[0].Content)
		assert.Equal(t, "Message 5", page2[1].Content)
		assert.Equal(t, "Message 6", page2[2].Content)

		// Test last partial page (offset 8, limit 5 - should return only 2)
		page3, total3, err := store.ReadMessagesPaginated(ctx, "test-paginate", 8, 5)
		require.NoError(t, err)
		assert.Equal(t, 10, total3)
		assert.Len(t, page3, 2)
		assert.Equal(t, "Message 9", page3[0].Content)
		assert.Equal(t, "Message 10", page3[1].Content)

		// Test offset beyond available data
		page4, total4, err := store.ReadMessagesPaginated(ctx, "test-paginate", 15, 5)
		require.NoError(t, err)
		assert.Equal(t, 10, total4)
		assert.Empty(t, page4)
	})
}
