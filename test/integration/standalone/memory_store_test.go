package standalone

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/test/helpers"
)

// setup creates a Redis-backed memory store using miniredis with a unique key prefix
func setup(t *testing.T) (*helpers.RedisHelper, *store.RedisMemoryStore) {
	t.Helper()
	rh := helpers.NewRedisHelper(t)
	t.Cleanup(func() { rh.Cleanup(t) })
	ms := store.NewRedisMemoryStore(rh.GetClient(), rh.GetKeyPrefix())
	return rh, ms
}

func TestMemoryStore_MiniredisCompatibility(t *testing.T) {
	t.Run("Should execute Lua scripts (append)", func(t *testing.T) {
		ctx := t.Context()
		rh, ms := setup(t)

		key := "lua-append"
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello, world!"}

		err := ms.AppendMessage(ctx, key, msg)
		require.NoError(t, err)

		// Verify list length and metadata counters updated
		msgs, err := ms.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, msgs, 1)
		assert.Equal(t, msg.Role, msgs[0].Role)
		assert.Equal(t, msg.Content, msgs[0].Content)

		count, err := ms.GetMessageCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// token count not incremented by AppendMessage (no token arg)
		tokens, err := ms.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 0, tokens)

		// Now append with token increment to validate metadata persistence
		err = ms.AppendMessageWithTokenCount(ctx, key, llm.Message{Role: llm.MessageRoleAssistant, Content: "Hi!"}, 7)
		require.NoError(t, err)

		tokens, err = ms.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 7, tokens)

		count, err = ms.GetMessageCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Ensure keys exist under the prefixed namespace
		rh.AssertKeyExists(ctx, t, key)
		rh.AssertKeyExists(ctx, t, key+":metadata")
	})

	t.Run("Should handle concurrent message appends without data loss", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)

		key := "concurrent"
		total := 50
		var wg sync.WaitGroup
		for i := 0; i < total; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = ms.AppendMessage(
					ctx,
					key,
					llm.Message{Role: llm.MessageRoleUser, Content: fmt.Sprintf("Message %d", i)},
				)
			}()
		}
		wg.Wait()

		// Verify all messages stored
		n, err := ms.CountMessages(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, total, n)
	})

	t.Run("Should preserve metadata counters across operations", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)

		key := "metadata"
		// Append with token increments in multiple steps
		require.NoError(
			t,
			ms.AppendMessageWithTokenCount(ctx, key, llm.Message{Role: llm.MessageRoleUser, Content: "A"}, 3),
		)
		require.NoError(
			t,
			ms.AppendMessageWithTokenCount(ctx, key, llm.Message{Role: llm.MessageRoleAssistant, Content: "B"}, 5),
		)

		tokens, err := ms.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 8, tokens)

		msgCount, err := ms.GetMessageCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 2, msgCount)
	})

	t.Run("Should maintain conversation history consistency", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)
		key := "consistency"
		sequence := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Question 1"},
			{Role: llm.MessageRoleAssistant, Content: "Answer 1"},
			{Role: llm.MessageRoleUser, Content: "Question 2"},
			{Role: llm.MessageRoleAssistant, Content: "Answer 2"},
		}
		for _, m := range sequence {
			require.NoError(t, ms.AppendMessage(ctx, key, m))
		}
		got, err := ms.ReadMessages(ctx, key)
		require.NoError(t, err)
		require.Len(t, got, len(sequence))
		for i := range sequence {
			assert.Equal(t, sequence[i].Role, got[i].Role)
			assert.Equal(t, sequence[i].Content, got[i].Content)
		}
	})

	t.Run("Should trim conversation history at max length", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)
		key := "trim"
		for i := 0; i < 15; i++ {
			require.NoError(
				t,
				ms.AppendMessage(
					ctx,
					key,
					llm.Message{Role: llm.MessageRoleUser, Content: fmt.Sprintf("Message %d", i)},
				),
			)
		}
		// Keep only last 10; token count not relevant for this test
		require.NoError(t, ms.TrimMessagesWithMetadata(ctx, key, 10, 0))
		msgs, err := ms.ReadMessages(ctx, key)
		require.NoError(t, err)
		require.Len(t, msgs, 10)
		// After trimming to last 10 of 0..14, first is 5 and last is 14
		assert.Equal(t, "Message 5", msgs[0].Content)
		assert.Equal(t, "Message 14", msgs[9].Content)
	})

	t.Run("Should support message retrieval with pagination", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)
		key := "pagination"
		total := 25
		for i := 0; i < total; i++ {
			require.NoError(
				t,
				ms.AppendMessage(
					ctx,
					key,
					llm.Message{Role: llm.MessageRoleUser, Content: fmt.Sprintf("Message %d", i)},
				),
			)
		}

		page1, totalCount, err := ms.ReadMessagesPaginated(ctx, key, 0, 10)
		require.NoError(t, err)
		assert.Equal(t, total, totalCount)
		require.Len(t, page1, 10)
		assert.Equal(t, "Message 0", page1[0].Content)
		assert.Equal(t, "Message 9", page1[9].Content)

		page3, totalCount, err := ms.ReadMessagesPaginated(ctx, key, 20, 10)
		require.NoError(t, err)
		assert.Equal(t, total, totalCount)
		require.Len(t, page3, 5)
		assert.Equal(t, "Message 20", page3[0].Content)
		assert.Equal(t, "Message 24", page3[4].Content)
	})

	t.Run("Should handle empty conversation history", func(t *testing.T) {
		ctx := t.Context()
		_, ms := setup(t)
		key := "empty"
		msgs, err := ms.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, msgs, 0)
	})
}
