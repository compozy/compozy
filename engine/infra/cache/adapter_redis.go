package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisAdapter implements cache contracts (KV, Lists, Hashes, KeysProvider, AtomicListWithMetadata)
// on top of a RedisInterface-compatible client.
type RedisAdapter struct {
	client    RedisInterface
	scanCount int
}

func NewRedisAdapter(client RedisInterface) (*RedisAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}
	return &RedisAdapter{client: client, scanCount: 100}, nil
}

// NewRedisAdapterFromConfig builds the adapter using cache configuration when available.
// If cfg or cfg.CacheConfig is nil or KeyScanCount <= 0, defaults to 100.
func NewRedisAdapterFromConfig(client RedisInterface, cfg *Config) (*RedisAdapter, error) {
	ad, err := NewRedisAdapter(client)
	if err != nil {
		return nil, err
	}
	if cfg != nil && cfg.CacheConfig != nil && cfg.KeyScanCount > 0 {
		ad.scanCount = cfg.KeyScanCount
	}
	return ad, nil
}

// Capabilities returns the supported features for the Redis adapter.
func (a *RedisAdapter) Capabilities() Capabilities {
	return Capabilities{
		KV:                     true,
		Lists:                  true,
		Hashes:                 true,
		PubSub:                 true,
		Locks:                  true,
		KeysIteration:          true,
		AtomicListWithMetadata: true,
	}
}

// --------------------
// KV
// --------------------

func (a *RedisAdapter) Get(ctx context.Context, key string) (string, error) {
	v, err := a.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (a *RedisAdapter) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := a.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return err
	}
	return nil
}

func (a *RedisAdapter) Del(ctx context.Context, keys ...string) (int64, error) {
	n, err := a.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (a *RedisAdapter) MGet(ctx context.Context, keys ...string) ([]string, error) {
	vals, err := a.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		if v == nil {
			out[i] = ""
			continue
		}
		switch t := v.(type) {
		case string:
			out[i] = t
		case []byte:
			out[i] = string(t)
		default:
			out[i] = fmt.Sprintf("%v", t)
		}
	}
	return out, nil
}

func (a *RedisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := a.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotFound
	}
	return true, nil
}

// --------------------
// Lists
// --------------------

func (a *RedisAdapter) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	out, err := a.client.LRange(ctx, key, start, stop).Result()
	if err == redis.Nil {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (a *RedisAdapter) LLen(ctx context.Context, key string) (int64, error) {
	n, err := a.client.LLen(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (a *RedisAdapter) LTrim(ctx context.Context, key string, start, stop int64) error {
	if err := a.client.LTrim(ctx, key, start, stop).Err(); err != nil {
		return err
	}
	return nil
}

func (a *RedisAdapter) RPush(ctx context.Context, key string, values ...any) (int64, error) {
	n, err := a.client.RPush(ctx, key, values...).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// --------------------
// Hashes
// --------------------

func (a *RedisAdapter) HSet(ctx context.Context, key string, values ...any) (int64, error) {
	n, err := a.client.HSet(ctx, key, values...).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (a *RedisAdapter) HGet(ctx context.Context, key, field string) (string, error) {
	v, err := a.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (a *RedisAdapter) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	cur, err := a.client.HIncrBy(ctx, key, field, incr).Result()
	if err != nil {
		return 0, err
	}
	return cur, nil
}

func (a *RedisAdapter) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	n, err := a.client.HDel(ctx, key, fields...).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// --------------------
// Keys iteration
// --------------------

type redisKeysIter struct {
	client  RedisInterface
	pattern string
	cursor  uint64
	done    bool
	count   int
}

func (a *RedisAdapter) Keys(_ context.Context, pattern string) (KeyIterator, error) {
	// Iterator uses the provided context in Next() calls for cancellation.
	return &redisKeysIter{client: a.client, pattern: pattern, cursor: 0, done: false, count: a.scanCount}, nil
}

func (it *redisKeysIter) Next(ctx context.Context) ([]string, bool, error) {
	if it.done {
		return nil, true, nil
	}
	// Use SCAN for incremental, non-blocking iteration
	cmd := it.client.Scan(ctx, it.cursor, it.pattern, int64(it.count))
	keys, next, err := cmd.Result()
	if err != nil {
		return nil, false, err
	}
	it.cursor = next
	if it.cursor == 0 {
		it.done = true
	}
	return keys, it.done && len(keys) == 0, nil
}

// --------------------
// Atomic list + metadata
// --------------------

// Lua script that appends messages to a list, trims it, updates a token counter key
// and applies TTLs atomically.
//
// ARGV schema (ordered):
//   - ARGV[1..msgCount]: string messages to append (RPUSH)
//   - ARGV[msgCount+1]:  maxLen (integer; -1 to skip trimming)
//   - ARGV[msgCount+2]:  tokenDelta (integer; 0 to skip update)
//   - ARGV[msgCount+3]:  ttl_ms (integer milliseconds; <=0 to skip PEXPIRE)
//
// KEYS:
//   - KEYS[1]: list key
//   - KEYS[2]: token key (list_key .. ":tokens")
//
// Returns: LLEN(list)
const luaAppendTrimMeta = `
local listKey = KEYS[1]
local tokenKey = KEYS[2]
local countArgs = table.getn(ARGV)
local maxLen = tonumber(ARGV[countArgs-2])
local tokenDelta = tonumber(ARGV[countArgs-1])
local ttlMs = tonumber(ARGV[countArgs])

-- Append messages
local msgCount = countArgs - 3
if msgCount > 0 then
  for i=1,msgCount,1 do
    redis.call('RPUSH', listKey, ARGV[i])
  end
end

-- Trim if needed
if maxLen and maxLen >= 0 then
  redis.call('LTRIM', listKey, -maxLen, -1)
end

-- Update token counter if needed
if tokenDelta and tokenDelta ~= 0 then
  local cur = redis.call('GET', tokenKey)
  if not cur then cur = '0' end
  local newv = tostring(tonumber(cur) + tokenDelta)
  redis.call('SET', tokenKey, newv)
end

-- TTL
if ttlMs and ttlMs > 0 then
  redis.call('PEXPIRE', listKey, ttlMs)
  redis.call('PEXPIRE', tokenKey, ttlMs)
end

return redis.call('LLEN', listKey)
`

func (a *RedisAdapter) AppendAndTrimWithMetadata(
	ctx context.Context,
	key string,
	messages []string,
	tokenDelta int,
	maxLen int,
	ttl time.Duration,
) (int64, error) {
	keys := []string{key, key + ":tokens"}
	args := make([]any, 0, len(messages)+3)
	for _, m := range messages {
		args = append(args, m)
	}
	args = append(args, maxLen, tokenDelta, ttl.Milliseconds())

	res, err := a.client.Eval(ctx, luaAppendTrimMeta, keys, args...).Result()
	if err != nil {
		logger.FromContext(ctx).With(
			"component", "cache_adapter",
			"cache_driver", "redis",
			"key", key,
			"messages", len(messages),
			"max_len", maxLen,
			"ttl_ms", ttl.Milliseconds(),
		).Debug("atomic append/trim with metadata failed", "error", err)
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return v, nil
	case string:
		var n int64
		if _, scanErr := fmt.Sscan(v, &n); scanErr != nil {
			return 0, fmt.Errorf("script return parse error: %w", scanErr)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unexpected script return type %T", v)
	}
}
