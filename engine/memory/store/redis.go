package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/redis/go-redis/v9"
)

// Error types and Lua scripts are now defined in separate files

// RedisMemoryStore implements the MemoryStore interface using Redis.
type RedisMemoryStore struct {
	client     cache.RedisInterface
	keyManager *KeyManager
	ttlOps     *TTLOperations
}

// NewRedisMemoryStore creates a new RedisMemoryStore.
// The keyPrefix parameter allows for namespacing Redis keys. If empty, no prefix is added.
// CRITICAL: The caller is responsible for providing the complete key prefix if needed.
// This prevents double-prefixing issues when keys are already namespaced by the MemoryManager.
func NewRedisMemoryStore(client cache.RedisInterface, keyPrefix string) *RedisMemoryStore {
	// IMPORTANT: Do not add a default prefix here. The MemoryManager provides
	// fully qualified keys that already include the namespace structure:
	// "compozy:{project_id}:memory:{user_defined_key}"
	keyManager := NewKeyManager(keyPrefix)
	ttlOps := NewTTLOperations(keyManager, client)

	return &RedisMemoryStore{
		client:     client,
		keyManager: keyManager,
		ttlOps:     ttlOps,
	}
}

func (s *RedisMemoryStore) fullKey(key string) string {
	return s.keyManager.FullKey(key)
}

func (s *RedisMemoryStore) metadataKey(key string) string {
	return s.keyManager.MetadataKey(key)
}

// AppendMessage appends a single message to the store under the given key.
// This method updates the message_count metadata but not token_count.
// The caller should use IncrementTokenCount separately for token tracking.
func (s *RedisMemoryStore) AppendMessage(ctx context.Context, key string, msg llm.Message) error {
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for redis store: %w", err)
	}
	// Use appendAndTrimWithMetadataScript to ensure message count is incremented
	// ARGV[1]=0 (no trim), ARGV[2]=0 (no token increment yet), ARGV[3]=message
	// The script automatically increments message_count for each message added
	return s.client.Eval(ctx, AppendAndTrimWithMetadataScript, []string{fKey, metaKey}, 0, 0, string(msgBytes)).Err()
}

// AppendMessageWithTokenCount atomically appends a message and updates both message and token count metadata.
// This ensures consistency between the message list and metadata.
func (s *RedisMemoryStore) AppendMessageWithTokenCount(
	ctx context.Context,
	key string,
	msg llm.Message,
	tokenCount int,
) error {
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for redis store: %w", err)
	}
	// Use appendAndTrimWithMetadataScript to atomically append message and update metadata
	// ARGV[1]=0 (no trim), ARGV[2]=tokenCount (increment token count), ARGV[3]=message
	// The script automatically increments message_count and token_count atomically
	return s.client.Eval(ctx, AppendAndTrimWithMetadataScript, []string{fKey, metaKey}, 0, tokenCount, string(msgBytes)).
		Err()
}

// AppendMessages appends multiple messages to the store under the given key.
// This method updates the message_count metadata but not token_count.
// The caller should calculate total tokens and use IncrementTokenCount if needed.
func (s *RedisMemoryStore) AppendMessages(ctx context.Context, key string, msgs []llm.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	args := make([]any, 2, len(msgs)+2)
	args[0] = 0 // No trim by default in this specific call
	args[1] = 0 // No token increment in this method
	for _, msg := range msgs {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		args = append(args, string(msgBytes))
	}
	// Use appendAndTrimWithMetadataScript to ensure message count is incremented
	return s.client.Eval(ctx, AppendAndTrimWithMetadataScript, []string{fKey, metaKey}, args...).Err()
}

// ReadMessages retrieves all messages associated with the given key.
// Uses LRANGE 0 -1 to get all elements from the list.
func (s *RedisMemoryStore) ReadMessages(ctx context.Context, key string) ([]llm.Message, error) {
	fKey := s.fullKey(key)
	vals, err := s.client.LRange(ctx, fKey, 0, -1).Result()
	if err == redis.Nil {
		return []llm.Message{}, nil // Key not found, return empty slice
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read messages from redis for key %s: %w", fKey, err)
	}

	messages := make([]llm.Message, 0, len(vals))
	for _, val := range vals {
		var msg llm.Message
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			// Log problematic message but try to continue
			// Consider how to handle poison pill messages - skip or error out?
			// For now, returning an error to indicate data integrity issue.
			return nil, fmt.Errorf(
				"%w: could not unmarshal message from key %s: %s",
				ErrInvalidMessage,
				fKey,
				err.Error(),
			)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// ReadMessagesPaginated retrieves messages with pagination support
func (s *RedisMemoryStore) ReadMessagesPaginated(
	ctx context.Context,
	key string,
	offset, limit int,
) ([]llm.Message, int, error) {
	fKey := s.fullKey(key)
	// Get total count first
	totalCount, err := s.client.LLen(ctx, fKey).Result()
	if err == redis.Nil {
		return []llm.Message{}, 0, nil // Key not found, return empty slice
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get message count from redis for key %s: %w", fKey, err)
	}
	// Validate offset
	if offset >= int(totalCount) {
		return []llm.Message{}, int(totalCount), nil // Offset beyond available data
	}
	// Calculate range for Redis LRANGE (inclusive start, inclusive end)
	start := int64(offset)
	end := int64(offset + limit - 1)
	if end >= totalCount {
		end = totalCount - 1 // Adjust to available data
	}
	// Get paginated messages
	vals, err := s.client.LRange(ctx, fKey, start, end).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read paginated messages from redis for key %s: %w", fKey, err)
	}
	messages := make([]llm.Message, 0, len(vals))
	for _, val := range vals {
		var msg llm.Message
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			return nil, 0, fmt.Errorf(
				"%w: could not unmarshal message from key %s: %s",
				ErrInvalidMessage,
				fKey,
				err.Error(),
			)
		}
		messages = append(messages, msg)
	}
	return messages, int(totalCount), nil
}

// CountMessages returns the number of messages for a given key.
// Uses LLEN.
func (s *RedisMemoryStore) CountMessages(ctx context.Context, key string) (int, error) {
	fKey := s.fullKey(key)
	count, err := s.client.LLen(ctx, fKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count messages in redis for key %s: %w", fKey, err)
	}
	return int(count), nil
}

// ReplaceMessages replaces all messages for a key with a new set of messages.
// Note: This method doesn't update metadata for backward compatibility.
// The caller must calculate total tokens and use SetTokenCount after calling this method.
func (s *RedisMemoryStore) ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error {
	fKey := s.fullKey(key)
	ttl, err := s.client.TTL(ctx, fKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get TTL for key %s before replace: %w", fKey, err)
	}
	if err == redis.Nil { // Key doesn't exist
		ttl = -1 // No TTL or use a default from memory resource if available
	}
	args := make([]any, 1, len(messages)+1)
	args[0] = ttl.Milliseconds()
	if ttl < 0 { // No current TTL or key doesn't exist
		args[0] = int64(0) // Lua script interprets 0 as no PEXPIRE
	}
	for _, msg := range messages {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message for replace: %w", err)
		}
		args = append(args, string(msgBytes))
	}
	_, err = s.client.Eval(ctx, ReplaceMessagesWithMetadataScript, []string{fKey, s.metadataKey(key)}, args...).Result()
	if err != nil {
		return fmt.Errorf("failed to replace messages for key %s using script: %w", fKey, err)
	}
	return nil
}

// SetExpiration sets or updates the TTL for the given memory key.
// Uses EXPIRE.
func (s *RedisMemoryStore) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	return s.ttlOps.SetExpiration(ctx, key, ttl)
}

// DeleteMessages removes all messages associated with the given key.
// Also removes the associated metadata.
func (s *RedisMemoryStore) DeleteMessages(ctx context.Context, key string) error {
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	// Delete both the list and metadata in a pipeline for efficiency
	pipe := s.client.Pipeline()
	pipe.Del(ctx, fKey)
	pipe.Del(ctx, metaKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete messages and metadata for key %s: %w", fKey, err)
	}
	return nil
}

// GetKeyTTL returns the remaining time-to-live for a given key.
func (s *RedisMemoryStore) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) {
	return s.ttlOps.GetKeyTTL(ctx, key)
}

// GetTokenCount retrieves the cached token count from metadata (O(1) operation).
// Returns 0 and no error if metadata doesn't exist (requires migration).
func (s *RedisMemoryStore) GetTokenCount(ctx context.Context, key string) (int, error) {
	metaKey := s.metadataKey(key)
	result, err := s.client.HGet(ctx, metaKey, metadataTokenCountField).Result()
	if err == redis.Nil {
		// Metadata doesn't exist - return 0 for migration path
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get token count for key %s: %w", key, err)
	}
	var count int
	if _, err := fmt.Sscanf(result, "%d", &count); err != nil {
		return 0, fmt.Errorf("failed to parse token count for key %s: %w", key, err)
	}
	return count, nil
}

// GetMessageCount retrieves the cached message count from metadata (O(1) operation).
// Returns 0 and no error if metadata doesn't exist (requires migration).
func (s *RedisMemoryStore) GetMessageCount(ctx context.Context, key string) (int, error) {
	metaKey := s.metadataKey(key)
	result, err := s.client.HGet(ctx, metaKey, metadataMessageCountField).Result()
	if err == redis.Nil {
		// Metadata doesn't exist - return 0 for migration path
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get message count for key %s: %w", key, err)
	}
	var count int
	if _, err := fmt.Sscanf(result, "%d", &count); err != nil {
		return 0, fmt.Errorf("failed to parse message count for key %s: %w", key, err)
	}
	return count, nil
}

// IncrementTokenCount atomically increments the token count metadata.
func (s *RedisMemoryStore) IncrementTokenCount(ctx context.Context, key string, delta int) error {
	if delta == 0 {
		return nil
	}
	metaKey := s.metadataKey(key)
	if err := s.client.HIncrBy(ctx, metaKey, metadataTokenCountField, int64(delta)).Err(); err != nil {
		return fmt.Errorf("failed to increment token count for key %s: %w", key, err)
	}
	return nil
}

// SetTokenCount sets the token count metadata to a specific value.
func (s *RedisMemoryStore) SetTokenCount(ctx context.Context, key string, count int) error {
	metaKey := s.metadataKey(key)
	if err := s.client.HSet(ctx, metaKey, metadataTokenCountField, count).Err(); err != nil {
		return fmt.Errorf("failed to set token count for key %s: %w", key, err)
	}
	return nil
}

// IsFlushPending checks if a flush operation is currently pending for this key.
func (s *RedisMemoryStore) IsFlushPending(ctx context.Context, key string) (bool, error) {
	pendingKey := s.flushPendingKey(key)
	result, err := s.client.Get(ctx, pendingKey).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check flush pending flag: %w", err)
	}
	return result == "1", nil
}

// MarkFlushPending sets or clears the flush pending flag for this key.
func (s *RedisMemoryStore) MarkFlushPending(ctx context.Context, key string, pending bool) error {
	pendingKey := s.flushPendingKey(key)
	if pending {
		// Use SetNX for an atomic check-and-set operation
		// It returns true if the key was set, false if it already existed
		wasSet, err := s.client.SetNX(ctx, pendingKey, "1", 30*time.Minute).Result()
		if err != nil {
			return fmt.Errorf("failed to set flush pending flag: %w", err)
		}
		if !wasSet {
			// The key was already set by another process
			return memcore.ErrFlushAlreadyPending
		}
	} else {
		// Clear the flag - not a race-sensitive operation
		err := s.client.Del(ctx, pendingKey).Err()
		if err != nil {
			return fmt.Errorf("failed to clear flush pending flag: %w", err)
		}
	}
	return nil
}

// flushPendingKey returns the Redis key for tracking pending flush state.
func (s *RedisMemoryStore) flushPendingKey(key string) string {
	return "__compozy_internal__:flush_pending:" + s.fullKey(key)
}

// AppendMessagesWithMetadata appends messages and updates metadata atomically.
// This is the preferred method for maintaining consistent token counts.
func (s *RedisMemoryStore) AppendMessagesWithMetadata(
	ctx context.Context,
	key string,
	msgs []llm.Message,
	tokenIncrement int,
) error {
	if len(msgs) == 0 {
		return nil
	}
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	args := make([]any, 2, len(msgs)+2)
	args[0] = 0 // No trim in this operation
	args[1] = tokenIncrement
	for _, msg := range msgs {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		args = append(args, string(msgBytes))
	}
	return s.client.Eval(ctx, AppendAndTrimWithMetadataScript, []string{fKey, metaKey}, args...).Err()
}

// ReplaceMessagesWithMetadata replaces all messages and updates metadata atomically.
// This ensures token count stays in sync with the actual messages.
func (s *RedisMemoryStore) ReplaceMessagesWithMetadata(
	ctx context.Context,
	key string,
	messages []llm.Message,
	totalTokens int,
) error {
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	ttl, err := s.client.TTL(ctx, fKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get TTL for key %s before replace: %w", fKey, err)
	}
	if err == redis.Nil {
		ttl = -1
	}
	args := make([]any, 2, len(messages)+2)
	args[0] = ttl.Milliseconds()
	if ttl < 0 {
		args[0] = int64(0)
	}
	args[1] = totalTokens
	for _, msg := range messages {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message for replace: %w", err)
		}
		args = append(args, string(msgBytes))
	}
	_, err = s.client.Eval(ctx, ReplaceMessagesWithMetadataScript, []string{fKey, metaKey}, args...).Result()
	if err != nil {
		return fmt.Errorf("failed to replace messages with metadata for key %s: %w", fKey, err)
	}
	return nil
}

// TrimMessagesWithMetadata trims messages and updates metadata atomically.
// The caller must provide the new token count after trimming.
func (s *RedisMemoryStore) TrimMessagesWithMetadata(
	ctx context.Context,
	key string,
	keepCount int,
	newTokenCount int,
) error {
	if keepCount < 0 {
		return errors.New("keepCount must be non-negative")
	}
	fKey := s.fullKey(key)
	metaKey := s.metadataKey(key)
	_, err := s.client.Eval(ctx, TrimMessagesWithMetadataScript, []string{fKey, metaKey}, keepCount, newTokenCount).
		Result()
	if err != nil {
		return fmt.Errorf("failed to trim messages with metadata for key %s: %w", fKey, err)
	}
	return nil
}
