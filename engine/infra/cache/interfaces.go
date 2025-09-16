package cache

import (
	"context"
	"time"
)

// KV defines backend-neutral key/value operations.
// Adapters must translate backend-specific errors to package errors (e.g., ErrNotFound).
type KV interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) (int64, error)
	MGet(ctx context.Context, keys ...string) ([]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Lists defines backend-neutral list operations.
type Lists interface {
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LTrim(ctx context.Context, key string, start, stop int64) error
	RPush(ctx context.Context, key string, values ...any) (int64, error)
}

// Hashes defines backend-neutral hash operations.
type Hashes interface {
	HSet(ctx context.Context, key string, values ...any) (int64, error)
	HGet(ctx context.Context, key, field string) (string, error)
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)
	HDel(ctx context.Context, key string, fields ...string) (int64, error)
}

// KeyIterator provides cursor-based key iteration semantics.
// Implementations should avoid loading all keys into memory at once.
type KeyIterator interface {
	Next(ctx context.Context) (keys []string, done bool, err error)
}

// KeysProvider exposes pattern-based key iteration.
// Pattern semantics should follow glob-like matching (e.g., "workflow:*").
type KeysProvider interface {
	Keys(ctx context.Context, pattern string) (KeyIterator, error)
}

// AtomicListWithMetadata defines an explicit atomic operation surface combining
// list append and trim with a metadata update (e.g., token counters, TTL).
// Adapters must guarantee atomicity according to their backend capabilities.
type AtomicListWithMetadata interface {
	AppendAndTrimWithMetadata(
		ctx context.Context,
		key string,
		messages []string,
		tokenDelta int,
		maxLen int,
		ttl time.Duration,
	) (int64, error)
}

// Capabilities describes optional features supported by a cache adapter.
type Capabilities struct {
	KV                     bool
	Lists                  bool
	Hashes                 bool
	PubSub                 bool
	Locks                  bool
	KeysIteration          bool
	AtomicListWithMetadata bool
}

// Capable is implemented by adapters that expose capability flags.
type Capable interface {
	Capabilities() Capabilities
}
