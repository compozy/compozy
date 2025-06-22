package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/llm"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to setup miniredis and RedisMemoryStore for testing
func setupRedisTestStore(t *testing.T) (*RedisMemoryStore, *miniredis.Miniredis, context.Context) {
	t.Helper()
	s, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")

	// Create a real redis client connected to miniredis
	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// Wrap the real client with our RedisInterface adapter if direct *redis.Client is not assignable
	// For this test, we'll assume cache.Redis can be instantiated with a redis.Client
	// or we use a mockable RedisInterface. For simplicity, let's use a direct client
	// and adapt cache.Redis or use a mockable interface in real scenario.
	// Here, we'll use the actual go-redis client that miniredis supports.
	// The cache.RedisInterface needs to be implemented by this raw client or a wrapper.
	// Let's assume cache.NewRedisFromClient constructor or similar exists,
	// or we directly use a Redis client that fits cache.RedisInterface.

	// For this example, we'll create a simple struct that implements RedisInterface using the rdb.
	mockRedisClient := &testRedisClient{client: rdb, mini: s}

	store := NewRedisMemoryStore(mockRedisClient, "testprefix:")
	ctx := context.Background()
	return store, s, ctx
}

// testRedisClient is a simple wrapper to make *redis.Client fit cache.RedisInterface for testing.
// In a real scenario, you'd use the actual cache.Redis that takes a *redis.Client or ensure
// cache.RedisInterface is directly compatible.
type testRedisClient struct {
	client *redis.Client
	mini   *miniredis.Miniredis // To access miniredis specific commands if needed for setup
}

func (trc *testRedisClient) Ping(ctx context.Context) *redis.StatusCmd { return trc.client.Ping(ctx) }
func (trc *testRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	return trc.client.Set(ctx, key, value, expiration)
}
func (trc *testRedisClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	return trc.client.SetNX(ctx, key, value, expiration)
}
func (trc *testRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	return trc.client.Get(ctx, key)
}
func (trc *testRedisClient) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	return trc.client.GetEx(ctx, key, expiration)
}
func (trc *testRedisClient) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	return trc.client.MGet(ctx, keys...)
}
func (trc *testRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return trc.client.Del(ctx, keys...)
}
func (trc *testRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return trc.client.Exists(ctx, keys...)
}
func (trc *testRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return trc.client.Expire(ctx, key, expiration)
}
func (trc *testRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return trc.client.TTL(ctx, key)
}
func (trc *testRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	return trc.client.Keys(ctx, pattern)
}
func (trc *testRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return trc.client.Scan(ctx, cursor, match, count)
}
func (trc *testRedisClient) Publish(ctx context.Context, channel string, message any) *redis.IntCmd {
	return trc.client.Publish(ctx, channel, message)
}
func (trc *testRedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return trc.client.Subscribe(ctx, channels...)
}
func (trc *testRedisClient) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return trc.client.PSubscribe(ctx, patterns...)
}
func (trc *testRedisClient) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return trc.client.Eval(ctx, script, keys, args...)
}
func (trc *testRedisClient) Pipeline() redis.Pipeliner { return trc.client.Pipeline() }
func (trc *testRedisClient) Close() error              { return trc.client.Close() }
func (trc *testRedisClient) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return trc.client.LRange(ctx, key, start, stop)
}
func (trc *testRedisClient) LLen(ctx context.Context, key string) *redis.IntCmd {
	return trc.client.LLen(ctx, key)
}
func (trc *testRedisClient) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	return trc.client.LTrim(ctx, key, start, stop)
}
func (trc *testRedisClient) RPush(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return trc.client.RPush(ctx, key, values...)
}
func (trc *testRedisClient) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return trc.client.HSet(ctx, key, values...)
}
func (trc *testRedisClient) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return trc.client.HGet(ctx, key, field)
}
func (trc *testRedisClient) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	return trc.client.HIncrBy(ctx, key, field, incr)
}
func (trc *testRedisClient) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	return trc.client.HDel(ctx, key, fields...)
}
func (trc *testRedisClient) TxPipeline() redis.Pipeliner {
	return trc.client.TxPipeline()
}

func TestRedisMemoryStore_AppendAndReadMessages(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testAppendRead"
	msg1 := llm.Message{Role: "user", Content: "Hello"}
	msg2 := llm.Message{Role: "assistant", Content: "Hi there"}

	err := store.AppendMessage(ctx, key, msg1)
	require.NoError(t, err)
	err = store.AppendMessage(ctx, key, msg2)
	require.NoError(t, err)

	messages, err := store.ReadMessages(ctx, key)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, msg1, messages[0])
	assert.Equal(t, msg2, messages[1])

	count, err := store.CountMessages(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestRedisMemoryStore_AppendMessagesBatch(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testAppendBatch"
	msgs := []llm.Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Message 2"},
	}

	err := store.AppendMessages(ctx, key, msgs)
	require.NoError(t, err)

	messages, err := store.ReadMessages(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, msgs, messages)
}

func TestRedisMemoryStore_TrimMessages(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testTrim"
	for i := 0; i < 5; i++ {
		err := store.AppendMessage(ctx, key, llm.Message{Role: "user", Content: fmt.Sprintf("Msg %d", i)})
		require.NoError(t, err)
	}

	err := store.TrimMessages(ctx, key, 2) // Keep last 2
	require.NoError(t, err)

	messages, err := store.ReadMessages(ctx, key)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "Msg 3", messages[0].Content)
	assert.Equal(t, "Msg 4", messages[1].Content)

	err = store.TrimMessages(ctx, key, 0) // Keep 0 (delete all)
	require.NoError(t, err)
	messages, err = store.ReadMessages(ctx, key)
	require.NoError(t, err)
	assert.Len(t, messages, 0)
}

func TestRedisMemoryStore_ReplaceMessages(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testReplace"
	initialMsgs := []llm.Message{{Role: "user", Content: "Initial"}}
	err := store.AppendMessages(ctx, key, initialMsgs)
	require.NoError(t, err)

	// Set TTL on the key
	s.SetTTL(store.fullKey(key), 5*time.Minute)

	newMsgs := []llm.Message{{Role: "assistant", Content: "Replaced"}}
	err = store.ReplaceMessages(ctx, key, newMsgs)
	require.NoError(t, err)

	messages, err := store.ReadMessages(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, newMsgs, messages)

	// Check if TTL was preserved (miniredis might not perfectly emulate PEXPIRE in script for existing TTL)
	// For this test, we'll just check if it's still there or a new one is set by script.
	// The script sets PEXPIRE if ARGV[1] > 0.
	ttl, err := store.GetKeyTTL(ctx, key)
	require.NoError(t, err)
	assert.True(t, ttl > 0, "TTL should be preserved or reset by ReplaceMessages script")
}

func TestRedisMemoryStore_SetAndGetExpiration(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testExpiration"
	err := store.AppendMessage(ctx, key, llm.Message{Role: "user", Content: "Time sensitive"})
	require.NoError(t, err)

	ttlDuration := time.Hour
	err = store.SetExpiration(ctx, key, ttlDuration)
	require.NoError(t, err)

	ttl, err := store.GetKeyTTL(ctx, key)
	require.NoError(t, err)
	// miniredis TTL might not be exact, check within a range
	assert.InDelta(t, ttlDuration.Seconds(), ttl.Seconds(), 1, "TTL not set correctly")

	// Test non-existent key TTL
	ttlNonExistent, err := store.GetKeyTTL(ctx, "nonexistentkey")
	require.NoError(t, err)
	assert.Equal(t, -2*time.Second, ttlNonExistent) // Redis returns -2 for non-existent keys

	// Test key with no TTL (miniredis might default to no expiry)
	s.Set(store.fullKey("noTTLKey"), "value") // Direct miniredis command
	ttlNoExpiry, err := store.GetKeyTTL(ctx, "noTTLKey")
	require.NoError(t, err)
	assert.Equal(t, -1*time.Second, ttlNoExpiry) // Redis returns -1 for keys with no expiry
}

func TestRedisMemoryStore_DeleteMessages(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	key := "testDelete"
	err := store.AppendMessage(ctx, key, llm.Message{Role: "user", Content: "To be deleted"})
	require.NoError(t, err)

	err = store.DeleteMessages(ctx, key)
	require.NoError(t, err)

	messages, err := store.ReadMessages(ctx, key)
	require.NoError(t, err)
	assert.Empty(t, messages, "Messages should be deleted")

	count, err := store.CountMessages(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Message count should be zero after deletion")
}

func TestRedisMemoryStore_ReadMessages_KeyNotFound(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()

	messages, err := store.ReadMessages(ctx, "nonExistentKey")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestRedisMemoryStore_AppendMessage_MarshalError(t *testing.T) {
	// This test requires a way to make json.Marshal fail for llm.Message
	// For example, if llm.Message contained a channel or function.
	// We'll simulate this by using a type that json.Marshal cannot handle.

	// Skip this test for now since llm.Message is a simple struct
	// that can't easily fail marshaling
	t.Skip("llm.Message cannot easily fail JSON marshaling")

	// Note: Testing json.Marshal failure for llm.Message is complex due to its simple structure.
	// The current llm.Message {Role, Content string, Metadata map[string]any} is generally safe.
	// A more robust test would involve a custom type known to fail marshaling.
}

func TestRedisMemoryStore_ReadMessages_UnmarshalError(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()
	mClient := store.client.(*testRedisClient).mini // Get underlying miniredis

	key := "testUnmarshalError"
	fullKey := store.fullKey(key)

	// Directly push malformed JSON into Redis list
	mClient.Lpush(fullKey, `{"role":"user","content":"good"}`)
	mClient.Lpush(fullKey, `this-is-not-json`)
	mClient.Lpush(fullKey, `{"role":"assistant","content":"also good"}`)

	_, err := store.ReadMessages(ctx, key)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidMessage)
	assert.Contains(t, err.Error(), "could not unmarshal message")
}

func TestRedisMemoryStore_ReplaceMessages_ScriptLogic(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()
	mClient := store.client.(*testRedisClient).mini

	key := "replaceScriptKey"
	fullKey := store.fullKey(key)

	// Initial state: 3 messages
	mClient.Lpush(fullKey, `{"role":"user", "content":"m1"}`)
	mClient.Lpush(fullKey, `{"role":"user", "content":"m2"}`)
	mClient.Lpush(fullKey, `{"role":"user", "content":"m3"}`)
	s.SetTTL(fullKey, 10*time.Minute) // Set an initial TTL

	newMsgs := []llm.Message{
		{Role: "assistant", Content: "new1"},
		{Role: "assistant", Content: "new2"},
	}

	err := store.ReplaceMessages(ctx, key, newMsgs)
	require.NoError(t, err)

	// Verify content
	storedMsgsStr, err := store.client.(*testRedisClient).client.LRange(ctx, fullKey, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, storedMsgsStr, 2)

	var m1 llm.Message
	json.Unmarshal([]byte(storedMsgsStr[0]), &m1)
	assert.Equal(t, "new1", m1.Content)

	// Verify TTL was preserved by the script (or attempted to be)
	ttl, err := store.client.(*testRedisClient).client.TTL(ctx, fullKey).Result()
	require.NoError(t, err)
	assert.True(t, ttl > 0, "TTL should have been preserved/reset by the script")
}

func TestRedisMemoryStore_AppendAndTrimScript_Logic(t *testing.T) {
	store, s, ctx := setupRedisTestStore(t)
	defer s.Close()
	mClient := store.client.(*testRedisClient).client // Get underlying redis client

	key := "appendTrimScriptKey"
	fullKey := store.fullKey(key)

	// Args for script: KEYS[1]=key, ARGV[1]=max_len, ARGV[2:]=messages
	msg1Bytes, _ := json.Marshal(llm.Message{Content: "m1"})
	msg2Bytes, _ := json.Marshal(llm.Message{Content: "m2"})
	msg3Bytes, _ := json.Marshal(llm.Message{Content: "m3"})

	// Append 2 messages, max_len 5 (no trim)
	_, err := mClient.Eval(ctx, appendAndTrimScript, []string{fullKey}, 5, string(msg1Bytes), string(msg2Bytes)).
		Result()
	require.NoError(t, err)
	llen, _ := mClient.LLen(ctx, fullKey).Result()
	assert.EqualValues(t, 2, llen)

	// Append 1 message, max_len 2 (trim will occur)
	// Current: m1, m2. Add m3. Max 2. Result: m2, m3
	_, err = mClient.Eval(ctx, appendAndTrimScript, []string{fullKey}, 2, string(msg3Bytes)).Result()
	require.NoError(t, err)
	llen, _ = mClient.LLen(ctx, fullKey).Result()
	assert.EqualValues(t, 2, llen)

	vals, _ := mClient.LRange(ctx, fullKey, 0, -1).Result()
	var rMsg llm.Message
	json.Unmarshal([]byte(vals[0]), &rMsg)
	assert.Equal(t, "m2", rMsg.Content)
	json.Unmarshal([]byte(vals[1]), &rMsg)
	assert.Equal(t, "m3", rMsg.Content)
}

func TestRedisMemoryStore_TokenCountMetadata(t *testing.T) {
	t.Run("Should get and set token count", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testTokenCount"
		// Initial get should return 0 (migration case)
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
		// Set token count
		err = store.SetTokenCount(ctx, key, 100)
		require.NoError(t, err)
		// Get should return the set value
		count, err = store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 100, count)
	})

	t.Run("Should increment token count", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testTokenIncrement"
		// Set initial count
		err := store.SetTokenCount(ctx, key, 50)
		require.NoError(t, err)
		// Increment by 25
		err = store.IncrementTokenCount(ctx, key, 25)
		require.NoError(t, err)
		// Verify new count
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 75, count)
		// Increment by negative value
		err = store.IncrementTokenCount(ctx, key, -10)
		require.NoError(t, err)
		count, err = store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 65, count)
	})

	t.Run("Should handle zero increment", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testZeroIncrement"
		err := store.SetTokenCount(ctx, key, 100)
		require.NoError(t, err)
		// Increment by 0 should be no-op
		err = store.IncrementTokenCount(ctx, key, 0)
		require.NoError(t, err)
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 100, count)
	})

	t.Run("Should maintain metadata with messages", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testWithMessages"
		// Append messages
		msg1 := llm.Message{Role: "user", Content: "Hello"}
		err := store.AppendMessage(ctx, key, msg1)
		require.NoError(t, err)
		// Set token count
		err = store.SetTokenCount(ctx, key, 10)
		require.NoError(t, err)
		// Append another message
		msg2 := llm.Message{Role: "assistant", Content: "Hi there"}
		err = store.AppendMessage(ctx, key, msg2)
		require.NoError(t, err)
		// Increment token count
		err = store.IncrementTokenCount(ctx, key, 15)
		require.NoError(t, err)
		// Verify both messages and metadata exist
		messages, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 25, count)
	})
}

func TestRedisMemoryStore_MetadataAwareMethods(t *testing.T) {
	t.Run("Should append messages with metadata update", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testAppendWithMeta"
		msgs := []llm.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		}
		// Append messages with 50 tokens total
		err := store.AppendMessagesWithMetadata(ctx, key, msgs, 50)
		require.NoError(t, err)
		// Verify messages were added
		readMsgs, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 2)
		// Verify token count was updated
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 50, count)
		// Append more messages with additional tokens
		moreMsgs := []llm.Message{
			{Role: "user", Content: "How are you?"},
		}
		err = store.AppendMessagesWithMetadata(ctx, key, moreMsgs, 10)
		require.NoError(t, err)
		// Verify all messages
		readMsgs, err = store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 3)
		// Verify token count was incremented
		count, err = store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 60, count)
	})
	t.Run("Should replace messages with metadata update", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testReplaceWithMeta"
		// Initial messages
		initialMsgs := []llm.Message{
			{Role: "user", Content: "Old message 1"},
			{Role: "assistant", Content: "Old message 2"},
		}
		err := store.AppendMessagesWithMetadata(ctx, key, initialMsgs, 30)
		require.NoError(t, err)
		// Replace with new messages
		newMsgs := []llm.Message{
			{Role: "system", Content: "Summary: User asked about the weather"},
			{Role: "user", Content: "What's the temperature?"},
		}
		err = store.ReplaceMessagesWithMetadata(ctx, key, newMsgs, 25)
		require.NoError(t, err)
		// Verify messages were replaced
		readMsgs, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 2)
		assert.Equal(t, llm.MessageRole("system"), readMsgs[0].Role)
		assert.Equal(t, "Summary: User asked about the weather", readMsgs[0].Content)
		// Verify token count was updated
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 25, count)
	})
	t.Run("Should trim messages with metadata update", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testTrimWithMeta"
		// Add 5 messages
		msgs := []llm.Message{
			{Role: "user", Content: "Message 1"},
			{Role: "assistant", Content: "Response 1"},
			{Role: "user", Content: "Message 2"},
			{Role: "assistant", Content: "Response 2"},
			{Role: "user", Content: "Message 3"},
		}
		err := store.AppendMessagesWithMetadata(ctx, key, msgs, 100)
		require.NoError(t, err)
		// Trim to keep only last 2 messages
		err = store.TrimMessagesWithMetadata(ctx, key, 2, 40)
		require.NoError(t, err)
		// Verify only last 2 messages remain
		readMsgs, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 2)
		assert.Equal(t, llm.MessageRole("assistant"), readMsgs[0].Role)
		assert.Equal(t, "Response 2", readMsgs[0].Content)
		assert.Equal(t, llm.MessageRole("user"), readMsgs[1].Role)
		assert.Equal(t, "Message 3", readMsgs[1].Content)
		// Verify token count was updated
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 40, count)
	})
	t.Run("Should handle empty trim with metadata", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testEmptyTrim"
		// Add messages
		msgs := []llm.Message{
			{Role: "user", Content: "Will be deleted"},
		}
		err := store.AppendMessagesWithMetadata(ctx, key, msgs, 10)
		require.NoError(t, err)
		// Trim to 0 (delete all)
		err = store.TrimMessagesWithMetadata(ctx, key, 0, 0)
		require.NoError(t, err)
		// Verify no messages remain
		readMsgs, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 0)
		// Verify token count is 0
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
	t.Run("Should delete messages and metadata", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "testDeleteWithMeta"
		// Add messages with metadata
		msgs := []llm.Message{
			{Role: "user", Content: "To be deleted"},
		}
		err := store.AppendMessagesWithMetadata(ctx, key, msgs, 15)
		require.NoError(t, err)
		// Delete messages
		err = store.DeleteMessages(ctx, key)
		require.NoError(t, err)
		// Verify messages are gone
		readMsgs, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		assert.Len(t, readMsgs, 0)
		// Verify metadata is also gone (should return 0)
		count, err := store.GetTokenCount(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestRedisMemoryStore_PerformanceComparison(t *testing.T) {
	t.Run("Should demonstrate O(1) performance for GetTokenCount", func(t *testing.T) {
		store, s, ctx := setupRedisTestStore(t)
		defer s.Close()
		key := "perfTest"
		// Add many messages with metadata
		var totalTokens int
		numMessages := 1000
		for i := 0; i < numMessages; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("This is message number %d with some content", i),
			}
			tokens := 10 // Assume each message has 10 tokens
			totalTokens += tokens
			err := store.AppendMessagesWithMetadata(ctx, key, []llm.Message{msg}, tokens)
			require.NoError(t, err)
		}
		// Time the metadata-based GetTokenCount (O(1))
		start := time.Now()
		count, err := store.GetTokenCount(ctx, key)
		metadataDuration := time.Since(start)
		require.NoError(t, err)
		assert.Equal(t, totalTokens, count)
		// Time reading all messages and calculating tokens (O(n))
		start = time.Now()
		messages, err := store.ReadMessages(ctx, key)
		require.NoError(t, err)
		// Simulate token counting
		calculatedTokens := 0
		for range messages {
			calculatedTokens += 10 // Same assumption as above
		}
		readAllDuration := time.Since(start)
		// Log the performance difference
		t.Logf("GetTokenCount (metadata): %v", metadataDuration)
		t.Logf("ReadMessages + count (O(n)): %v", readAllDuration)
		t.Logf("Performance improvement: %.2fx faster", float64(readAllDuration)/float64(metadataDuration))
		// The metadata approach should be significantly faster
		assert.Less(t, metadataDuration, readAllDuration)
		assert.Equal(t, totalTokens, calculatedTokens)
	})
}

func TestRedisMemoryStore_KeyPrefixHandling(t *testing.T) {
	t.Run("Should handle empty prefix correctly", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err, "Failed to start miniredis")
		defer s.Close()
		rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
		mockRedisClient := &testRedisClient{client: rdb, mini: s}
		// Create store with empty prefix
		store := NewRedisMemoryStore(mockRedisClient, "")
		ctx := context.Background()
		// Use a fully qualified key like MemoryManager would provide
		fullyQualifiedKey := "compozy:proj1:memory:user1"
		msg := llm.Message{Role: "user", Content: "Test message"}
		err = store.AppendMessage(ctx, fullyQualifiedKey, msg)
		require.NoError(t, err)
		// Verify the key is stored exactly as provided (no double prefix)
		actualKey := store.fullKey(fullyQualifiedKey)
		assert.Equal(t, fullyQualifiedKey, actualKey, "Empty prefix should not modify the key")
		// Verify the message can be read back
		messages, err := store.ReadMessages(ctx, fullyQualifiedKey)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, msg, messages[0])
		// Check metadata key format
		metaKey := store.metadataKey(fullyQualifiedKey)
		assert.Equal(t, fullyQualifiedKey+":metadata", metaKey, "Metadata key should just append :metadata")
		// Verify in Redis directly
		storedKeys, err := rdb.Keys(ctx, "*").Result()
		require.NoError(t, err)
		assert.Contains(t, storedKeys, fullyQualifiedKey, "Key should be stored without prefix")
	})
	t.Run("Should handle non-empty prefix correctly", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err, "Failed to start miniredis")
		defer s.Close()
		rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
		mockRedisClient := &testRedisClient{client: rdb, mini: s}
		// Create store with a custom prefix
		customPrefix := "custom:prefix:"
		store := NewRedisMemoryStore(mockRedisClient, customPrefix)
		ctx := context.Background()
		simpleKey := "mykey"
		msg := llm.Message{Role: "assistant", Content: "Response"}
		err = store.AppendMessage(ctx, simpleKey, msg)
		require.NoError(t, err)
		// Verify the key has the prefix
		actualKey := store.fullKey(simpleKey)
		assert.Equal(t, customPrefix+simpleKey, actualKey, "Prefix should be prepended to the key")
		// Verify the message can be read back
		messages, err := store.ReadMessages(ctx, simpleKey)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, msg, messages[0])
		// Check metadata key format
		metaKey := store.metadataKey(simpleKey)
		assert.Equal(t, customPrefix+simpleKey+":metadata", metaKey, "Metadata key should have prefix")
		// Verify in Redis directly
		storedKeys, err := rdb.Keys(ctx, "*").Result()
		require.NoError(t, err)
		assert.Contains(t, storedKeys, customPrefix+simpleKey, "Key should be stored with prefix")
	})
	t.Run("Should prevent double-prefixing issue", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err, "Failed to start miniredis")
		defer s.Close()
		rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
		mockRedisClient := &testRedisClient{client: rdb, mini: s}
		// Simulate the MemoryManager use case
		store := NewRedisMemoryStore(mockRedisClient, "") // Empty prefix as MemoryManager does
		ctx := context.Background()
		// Use a key that already has the compozy:memory: prefix
		preformattedKey := "compozy:testproject:memory:session123"
		msg := llm.Message{Role: "system", Content: "System message"}
		err = store.AppendMessage(ctx, preformattedKey, msg)
		require.NoError(t, err)
		// Verify no double-prefixing occurred
		actualKey := store.fullKey(preformattedKey)
		assert.Equal(t, preformattedKey, actualKey, "Should not double-prefix")
		assert.NotContains(t, actualKey, "compozy:memory:compozy:", "Should not have double prefix")
		// Verify data integrity
		messages, err := store.ReadMessages(ctx, preformattedKey)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, msg, messages[0])
		// Test metadata operations with no double-prefix
		err = store.SetTokenCount(ctx, preformattedKey, 42)
		require.NoError(t, err)
		count, err := store.GetTokenCount(ctx, preformattedKey)
		require.NoError(t, err)
		assert.Equal(t, 42, count)
		// Verify Redis keys don't have double prefix
		storedKeys, err := rdb.Keys(ctx, "*").Result()
		require.NoError(t, err)
		for _, key := range storedKeys {
			assert.NotContains(t, key, "compozy:memory:compozy:", "No key should have double prefix")
		}
	})
}
