package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/CompoZy/llm-router/engine/infra/cache" // Assuming this is the correct path
	"github.com/CompoZy/llm-router/engine/llm"    // Assuming llm.Message is here
	"github.com/redis/go-redis/v9"
)

var (
	ErrMemoryKeyNotFound = errors.New("memory key not found")
	ErrInvalidMessage    = errors.New("invalid message format in store")
)

const (
	// Default script to append messages and trim the list to a max length.
	// KEYS[1]: the list key
	// ARGV[1]: max length of the list
	// ARGV[2...]: messages to append (JSON marshaled)
	// Returns: current length of the list after operation.
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
	// Script to replace all messages and set TTL
	// KEYS[1]: the list key
	// ARGV[1]: TTL in milliseconds
	// ARGV[2...]: messages to append (JSON marshaled)
	// Returns: "OK"
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
)

// RedisMemoryStore implements the MemoryStore interface using Redis.
type RedisMemoryStore struct {
	client    cache.RedisInterface
	keyPrefix string // e.g., "compozy:memory:"
}

// NewRedisMemoryStore creates a new RedisMemoryStore.
func NewRedisMemoryStore(client cache.RedisInterface, keyPrefix string) *RedisMemoryStore {
	if keyPrefix == "" {
		keyPrefix = "compozy:memory:"
	}
	return &RedisMemoryStore{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

func (s *RedisMemoryStore) fullKey(key string) string {
	return s.keyPrefix + key
}

// AppendMessage appends a single message to the store under the given key.
// Uses RPUSH to add to the tail of the list.
func (s *RedisMemoryStore) AppendMessage(ctx context.Context, key string, msg llm.Message) error {
	fKey := s.fullKey(key)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for redis store: %w", err)
	}
	return s.client.Eval(ctx, appendAndTrimScript, []string{fKey}, 0, string(msgBytes)).Err() // ARGV[1]=0 means no trim
}

// AppendMessages appends multiple messages to the store under the given key.
// Uses a Lua script with RPUSH for atomicity if possible, otherwise pipeline.
func (s *RedisMemoryStore) AppendMessages(ctx context.Context, key string, msgs []llm.Message) error {
	fKey := s.fullKey(key)
	if len(msgs) == 0 {
		return nil
	}

	args := make([]interface{}, 1, len(msgs)+1)
	args[0] = 0 // No trim by default in this specific call, trim is separate

	for _, msg := range msgs {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		args = append(args, string(msgBytes))
	}
	return s.client.Eval(ctx, appendAndTrimScript, []string{fKey}, args...).Err()
}

// ReadMessages retrieves all messages associated with the given key.
// Uses LRANGE 0 -1 to get all elements from the list.
func (s *RedisMemoryStore) ReadMessages(ctx context.Context, key string) ([]llm.Message, error) {
	fKey := s.fullKey(key)
	vals, err := s.client.Pipeline().LRange(ctx, fKey, 0, -1).Result()
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
			return nil, fmt.Errorf("%w: could not unmarshal message from key %s: %s", ErrInvalidMessage, fKey, err.Error())
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// CountMessages returns the number of messages for a given key.
// Uses LLEN.
func (s *RedisMemoryStore) CountMessages(ctx context.Context, key string) (int, error) {
	fKey := s.fullKey(key)
	count, err := s.client.Pipeline().LLen(ctx, fKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count messages in redis for key %s: %w", fKey, err)
	}
	return int(count), nil
}

// TrimMessages removes messages from the store to keep only `keepCount` newest messages.
// Uses LTRIM key -keepCount -1 (keeps from end of list).
func (s *RedisMemoryStore) TrimMessages(ctx context.Context, key string, keepCount int) error {
	if keepCount < 0 {
		return errors.New("keepCount must be non-negative")
	}
	fKey := s.fullKey(key)
	// LTRIM works by specifying the new start and end indexes of the list.
	// To keep the last 'keepCount' items, we LTRIM from -keepCount to -1.
	// If keepCount is 0, it means delete all, so LTRIM key 1 0.
	var err error
	if keepCount == 0 {
		err = s.client.Pipeline().LTrim(ctx, fKey, 1, 0).Err()
	} else {
		err = s.client.Pipeline().LTrim(ctx, fKey, int64(-keepCount), -1).Err()
	}
	if err != nil {
		return fmt.Errorf("failed to trim messages in redis for key %s: %w", fKey, err)
	}
	return nil
}

// ReplaceMessages replaces all messages for a key with a new set of messages.
// Uses DEL then RPUSH, ideally in a transaction or Lua script for atomicity.
func (s *RedisMemoryStore) ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error {
	fKey := s.fullKey(key)

	ttl, err := s.client.TTL(ctx, fKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get TTL for key %s before replace: %w", fKey, err)
	}
	if err == redis.Nil { // Key doesn't exist
		ttl = -1 // No TTL or use a default from memory resource if available
	}


	args := make([]interface{}, 1, len(messages)+1)
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
	_, err := s.client.Pipeline().Expire(ctx, fKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", fKey, err)
	}
	return nil
}

// DeleteMessages removes all messages associated with the given key.
// Uses DEL.
func (s *RedisMemoryStore) DeleteMessages(ctx context.Context, key string) error {
	fKey := s.fullKey(key)
	_, err := s.client.Pipeline().Del(ctx, fKey).Result()
	if err != nil {
		return fmt.Errorf("failed to delete messages for key %s: %w", fKey, err)
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
	// If key exists but has no associated expire, TTL returns -1.
	return ttl, nil
}
