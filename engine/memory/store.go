package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/cache" // Assuming this is the correct path
	"github.com/compozy/compozy/engine/llm"         // Assuming llm.Message is here
	"github.com/redis/go-redis/v9"
)

var (
	ErrMemoryKeyNotFound = errors.New("memory key not found")
	ErrInvalidMessage    = errors.New("invalid message format in store")
)

// Memory operation error types for precise error handling
type LockError struct {
	Operation string
	Key       string
	Err       error
}

func (e *LockError) Error() string {
	return fmt.Sprintf("memory lock error during %s for key %s: %v", e.Operation, e.Key, e.Err)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

func NewLockError(operation, key string, err error) *LockError {
	return &LockError{
		Operation: operation,
		Key:       key,
		Err:       err,
	}
}

type ConfigError struct {
	ResourceID string
	Reason     string
	Err        error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("memory configuration error for resource %s: %s", e.ResourceID, e.Reason)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

func NewConfigError(resourceID, reason string, err error) *ConfigError {
	return &ConfigError{
		ResourceID: resourceID,
		Reason:     reason,
		Err:        err,
	}
}

const (
	// Script to append messages with metadata updates and optional trim
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: max length of the list (0 means no trim)
	// ARGV[2]: token count increment
	// ARGV[3...]: messages to append (JSON marshaled)
	// Returns: current length of the list after operation.
	appendAndTrimWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local max_len = tonumber(ARGV[1])
		local token_incr = tonumber(ARGV[2])
		local msg_count = 0

		-- Append messages
		for i = 3, #ARGV do
			redis.call('RPUSH', list_key, ARGV[i])
			msg_count = msg_count + 1
		end

		-- Update metadata
		if token_incr > 0 then
			redis.call('HINCRBY', meta_key, 'token_count', token_incr)
		end
		redis.call('HINCRBY', meta_key, 'message_count', msg_count)

		-- Trim if needed
		if max_len > 0 then
			redis.call('LTRIM', list_key, -max_len, -1)
			-- Update message count after trim
			local new_len = redis.call('LLEN', list_key)
			redis.call('HSET', meta_key, 'message_count', new_len)
		end

		return redis.call('LLEN', list_key)
	`

	// Script to replace all messages with metadata reset
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: TTL in milliseconds
	// ARGV[2]: new total token count
	// ARGV[3...]: messages to append (JSON marshaled)
	// Returns: "OK"
	replaceMessagesWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local ttl_ms = tonumber(ARGV[1])
		local new_token_count = tonumber(ARGV[2])
		local msg_count = 0

		-- Delete existing list
		redis.call('DEL', list_key)

		-- Add new messages
		for i = 3, #ARGV do
			redis.call('RPUSH', list_key, ARGV[i])
			msg_count = msg_count + 1
		end

		-- Update metadata
		redis.call('HSET', meta_key, 'token_count', new_token_count)
		redis.call('HSET', meta_key, 'message_count', msg_count)

		-- Set TTL if needed
		if ttl_ms > 0 then
			redis.call('PEXPIRE', list_key, ttl_ms)
			redis.call('PEXPIRE', meta_key, ttl_ms)
		end

		return "OK"
	`

	// Script to trim messages and update metadata
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: number of messages to keep
	// ARGV[2]: new token count after recalculation
	// Returns: new message count
	trimMessagesWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local keep_count = tonumber(ARGV[1])
		local new_token_count = tonumber(ARGV[2])

		-- Trim messages
		if keep_count == 0 then
			redis.call('LTRIM', list_key, 1, 0)
		else
			redis.call('LTRIM', list_key, -keep_count, -1)
		end

		-- Update metadata
		local new_len = redis.call('LLEN', list_key)
		redis.call('HSET', meta_key, 'message_count', new_len)
		redis.call('HSET', meta_key, 'token_count', new_token_count)

		return new_len
	`

	// Legacy scripts for backward compatibility
	appendAndTrimScript = `
		for i = 2, #ARGV do
			redis.call('RPUSH', KEYS[1], ARGV[i])
		end
		local N = tonumber(ARGV[1])
		if N > 0 then
			redis.call('LTRIM', KEYS[1], -N, -1)
		end
		return redis.call('LLEN', KEYS[1])
	`
	replaceMessagesAndSetTTLScript = `
		redis.call('DEL', KEYS[1])
		for i = 2, #ARGV do
			redis.call('RPUSH', KEYS[1], ARGV[i])
		end
		if tonumber(ARGV[1]) > 0 then
			redis.call('PEXPIRE', KEYS[1], ARGV[1])
		end
		return "OK"
	`

	// Metadata fields stored in Redis hash
	metadataTokenCountField   = "token_count"
	metadataMessageCountField = "message_count"
)

// RedisMemoryStore implements the MemoryStore interface using Redis.
type RedisMemoryStore struct {
	client    cache.RedisInterface
	keyPrefix string // e.g., "compozy:memory:"
}

// NewRedisMemoryStore creates a new RedisMemoryStore.
// The keyPrefix parameter allows for namespacing Redis keys. If empty, no prefix is added.
// CRITICAL: The caller is responsible for providing the complete key prefix if needed.
// This prevents double-prefixing issues when keys are already namespaced by the MemoryManager.
func NewRedisMemoryStore(client cache.RedisInterface, keyPrefix string) *RedisMemoryStore {
	// IMPORTANT: Do not add a default prefix here. The MemoryManager provides
	// fully qualified keys that already include the namespace structure:
	// "compozy:{project_id}:memory:{user_defined_key}"
	return &RedisMemoryStore{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

func (s *RedisMemoryStore) fullKey(key string) string {
	// If no prefix is set, return the key as-is
	if s.keyPrefix == "" {
		return key
	}
	// Check if prefix already ends with colon to avoid double colons
	if strings.HasSuffix(s.keyPrefix, ":") {
		return s.keyPrefix + key
	}
	return s.keyPrefix + ":" + key
}

func (s *RedisMemoryStore) metadataKey(key string) string {
	// If no prefix is set, append metadata suffix directly to the key
	if s.keyPrefix == "" {
		return key + ":metadata"
	}
	// Check if prefix already ends with colon to avoid double colons
	if strings.HasSuffix(s.keyPrefix, ":") {
		return s.keyPrefix + key + ":metadata"
	}
	return s.keyPrefix + ":" + key + ":metadata"
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
	return s.client.Eval(ctx, appendAndTrimWithMetadataScript, []string{fKey, metaKey}, 0, 0, string(msgBytes)).Err()
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
	return s.client.Eval(ctx, appendAndTrimWithMetadataScript, []string{fKey, metaKey}, 0, tokenCount, string(msgBytes)).
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
	return s.client.Eval(ctx, appendAndTrimWithMetadataScript, []string{fKey, metaKey}, args...).Err()
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
	_, err = s.client.Eval(ctx, replaceMessagesAndSetTTLScript, []string{fKey}, args...).Result()
	if err != nil {
		return fmt.Errorf("failed to replace messages for key %s using script: %w", fKey, err)
	}
	return nil
}

// SetExpiration sets or updates the TTL for the given memory key.
// Uses EXPIRE.
func (s *RedisMemoryStore) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	fKey := s.fullKey(key)
	if ttl <= 0 { // Redis EXPIRE with 0 or negative deletes the key or removes TTL. Be specific.
		// To remove TTL, use PERSIST. For this method, assume positive TTL means set, non-positive means error or no-op.
		return errors.New("TTL must be a positive duration to set expiration")
	}
	_, err := s.client.Expire(ctx, fKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", fKey, err)
	}
	return nil
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
	fKey := s.fullKey(key)
	ttl, err := s.client.TTL(ctx, fKey).Result()
	if err == redis.Nil {
		return -2 * time.Second, nil // Key does not exist, consistent with redis.TTL behavior for non-existent keys
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", fKey, err)
	}
	// Redis returns -2 for non-existent keys and -1 for keys without expiry
	// The TTL command returns time.Duration, but miniredis might return it differently
	// Normalize special values
	if ttl == -2*time.Nanosecond {
		return -2 * time.Second, nil
	}
	if ttl == -1*time.Nanosecond {
		return -1 * time.Second, nil
	}
	return ttl, nil
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
			return ErrFlushAlreadyPending
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
	return s.client.Eval(ctx, appendAndTrimWithMetadataScript, []string{fKey, metaKey}, args...).Err()
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
	_, err = s.client.Eval(ctx, replaceMessagesWithMetadataScript, []string{fKey, metaKey}, args...).Result()
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
	_, err := s.client.Eval(ctx, trimMessagesWithMetadataScript, []string{fKey, metaKey}, keepCount, newTokenCount).
		Result()
	if err != nil {
		return fmt.Errorf("failed to trim messages with metadata for key %s: %w", fKey, err)
	}
	return nil
}
