package memory

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to setup miniredis and RedisLockManager for testing
func setupRedisLockManager(t *testing.T) (*cache.RedisLockManager, *miniredis.Miniredis, context.Context) {
	t.Helper()
	s, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	// Similar to store_test.go, wrap rdb to fit cache.RedisInterface
	mockRedisClient := &testRedisClientLock{client: rdb} // Use a distinct name if needed

	lockManager, err := cache.NewRedisLockManager(mockRedisClient)
	require.NoError(t, err)

	ctx := context.Background()
	return lockManager, s, ctx
}

// testRedisClientLock is a wrapper for testing cache.RedisLockManager
type testRedisClientLock struct {
	client *redis.Client
}

func (trcl *testRedisClientLock) Ping(ctx context.Context) *redis.StatusCmd {
	return trcl.client.Ping(ctx)
}

func (trcl *testRedisClientLock) Set(
	ctx context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.StatusCmd {
	return trcl.client.Set(ctx, key, value, expiration)
}

func (trcl *testRedisClientLock) SetNX(
	ctx context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.BoolCmd {
	return trcl.client.SetNX(ctx, key, value, expiration)
}
func (trcl *testRedisClientLock) Get(ctx context.Context, key string) *redis.StringCmd {
	return trcl.client.Get(ctx, key)
}
func (trcl *testRedisClientLock) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	return trcl.client.GetEx(ctx, key, expiration)
}
func (trcl *testRedisClientLock) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	return trcl.client.MGet(ctx, keys...)
}
func (trcl *testRedisClientLock) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return trcl.client.Del(ctx, keys...)
}
func (trcl *testRedisClientLock) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return trcl.client.Exists(ctx, keys...)
}
func (trcl *testRedisClientLock) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return trcl.client.Expire(ctx, key, expiration)
}
func (trcl *testRedisClientLock) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return trcl.client.TTL(ctx, key)
}
func (trcl *testRedisClientLock) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	return trcl.client.Keys(ctx, pattern)
}
func (trcl *testRedisClientLock) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return trcl.client.Scan(ctx, cursor, match, count)
}
func (trcl *testRedisClientLock) Publish(ctx context.Context, channel string, message any) *redis.IntCmd {
	return trcl.client.Publish(ctx, channel, message)
}
func (trcl *testRedisClientLock) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return trcl.client.Subscribe(ctx, channels...)
}
func (trcl *testRedisClientLock) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return trcl.client.PSubscribe(ctx, patterns...)
}
func (trcl *testRedisClientLock) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return trcl.client.Eval(ctx, script, keys, args...)
}
func (trcl *testRedisClientLock) Pipeline() redis.Pipeliner { return trcl.client.Pipeline() }
func (trcl *testRedisClientLock) Close() error              { return trcl.client.Close() }
func (trcl *testRedisClientLock) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return trcl.client.LRange(ctx, key, start, stop)
}
func (trcl *testRedisClientLock) LLen(ctx context.Context, key string) *redis.IntCmd {
	return trcl.client.LLen(ctx, key)
}
func (trcl *testRedisClientLock) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	return trcl.client.LTrim(ctx, key, start, stop)
}
func (trcl *testRedisClientLock) RPush(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return trcl.client.RPush(ctx, key, values...)
}
func (trcl *testRedisClientLock) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return trcl.client.HSet(ctx, key, values...)
}
func (trcl *testRedisClientLock) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return trcl.client.HGet(ctx, key, field)
}
func (trcl *testRedisClientLock) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	return trcl.client.HIncrBy(ctx, key, field, incr)
}
func (trcl *testRedisClientLock) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	return trcl.client.HDel(ctx, key, fields...)
}
func (trcl *testRedisClientLock) TxPipeline() redis.Pipeliner {
	return trcl.client.TxPipeline()
}

func TestMemoryLockManager_AcquireAndRelease(t *testing.T) {
	internalLockManager, s, ctx := setupRedisLockManager(t)
	defer s.Close()

	memLockManager, err := NewLockManager(internalLockManager, "testmlock:")
	require.NoError(t, err)

	resourceID := "myResource123"
	lockTTL := 100 * time.Millisecond // Short TTL for testing auto-renewal impact

	// Acquire the lock
	lock, err := memLockManager.Acquire(ctx, resourceID, lockTTL)
	require.NoError(t, err, "Failed to acquire lock")
	require.NotNil(t, lock)
	assert.True(t, lock.IsHeld(), "Lock should be held after acquiring")
	assert.Equal(t, resourceID, lock.Resource(), "Resource ID mismatch")

	// Check underlying Redis key
	// The key format is "lock:testmlock:myResource123"
	lockKey := "lock:testmlock:" + resourceID
	exists := s.Exists(lockKey)
	assert.True(t, exists, "Lock key should exist in Redis")

	// Try to acquire again (should fail)
	_, err = memLockManager.Acquire(ctx, resourceID, lockTTL)
	assert.Error(t, err, "Acquiring an already held lock should fail")
	assert.ErrorIs(t, err, cache.ErrLockNotAcquired, "Should be ErrLockNotAcquired")

	// Release the lock
	err = lock.Release(ctx)
	assert.NoError(t, err, "Failed to release lock")
	assert.False(t, lock.IsHeld(), "Lock should not be held after releasing")

	// Check underlying Redis key is gone
	lockKey = "lock:testmlock:" + resourceID
	exists = s.Exists(lockKey)
	assert.False(t, exists, "Lock key should not exist in Redis after release")

	// Try to release again (should fail)
	err = lock.Release(ctx)
	assert.Error(t, err, "Releasing an already released lock should fail")
	assert.ErrorIs(t, err, cache.ErrLockNotHeld)
}

func TestMemoryLockManager_LockAutoRenewalAndRefresh(t *testing.T) {
	internalLockManager, s, ctx := setupRedisLockManager(t)
	defer s.Close()

	memLockManager, err := NewLockManager(internalLockManager, "testmlock:")
	require.NoError(t, err)

	resourceID := "autoRenewResource"
	lockTTL := 150 * time.Millisecond // Adjusted TTL for reliable testing of renewal

	lock, err := memLockManager.Acquire(ctx, resourceID, lockTTL)
	require.NoError(t, err)
	assert.True(t, lock.IsHeld())

	// Wait for a period longer than TTL/3 but less than TTL to check auto-renewal
	// Renewal interval is TTL/3, so for 150ms TTL, it's 50ms.
	time.Sleep(lockTTL/2 + 20*time.Millisecond) // e.g., 75ms + 20ms = 95ms
	assert.True(t, lock.IsHeld(), "Lock should still be held due to auto-renewal")

	// Manually refresh
	err = lock.Refresh(ctx)
	require.NoError(t, err, "Manual refresh failed")
	assert.True(t, lock.IsHeld(), "Lock should be held after manual refresh")

	// Check TTL in Redis (it should be close to original TTL after refresh)
	lockKey := "lock:testmlock:" + resourceID
	ttl := s.TTL(lockKey)
	assert.True(t, ttl > lockTTL/2, "TTL should be substantial after refresh")

	err = lock.Release(ctx)
	require.NoError(t, err)
	assert.False(t, lock.IsHeld())

	// Wait for TTL to pass to ensure auto-renewal goroutine has stopped
	time.Sleep(lockTTL * 2)
}

func TestMemoryLockManager_LockExpiration(t *testing.T) {
	internalLockManager, s, ctx := setupRedisLockManager(t)
	defer s.Close()

	memLockManager, err := NewLockManager(internalLockManager, "testmlock:")
	require.NoError(t, err)

	resourceID := "expiringResource"
	lockTTL := 50 * time.Millisecond // Very short TTL

	lock, err := memLockManager.Acquire(ctx, resourceID, lockTTL)
	require.NoError(t, err)

	// Stop auto-renewal by releasing the lock (which closes the stopChan)
	// then check if it expires in Redis
	err = lock.Release(ctx) // This will stop the auto-renew goroutine
	require.NoError(t, err)

	// Re-acquire to test expiration without auto-renewal interference
	// First, ensure it's gone from redis by waiting.
	s.FastForward(lockTTL + 10*time.Millisecond) // Ensure key expires in miniredis

	// Now, try to acquire it again, should succeed as the previous one "expired" (was released and time passed)
	lock2, err := memLockManager.Acquire(ctx, resourceID, lockTTL)
	require.NoError(t, err, "Should be able to acquire lock after previous one expired")
	assert.NotNil(t, lock2)
	assert.True(t, lock2.IsHeld())

	err = lock2.Release(ctx)
	require.NoError(t, err)
}

func TestMemoryLockManager_GetMetrics(t *testing.T) {
	internalLockManager, s, ctx := setupRedisLockManager(t)
	defer s.Close()

	memLockManager, err := NewLockManager(internalLockManager, "testmlock:")
	require.NoError(t, err)

	resourceID := "metricsResource"
	lockTTL := 100 * time.Millisecond

	lock, err := memLockManager.Acquire(ctx, resourceID, lockTTL)
	require.NoError(t, err)

	metrics, err := memLockManager.GetMetrics()
	require.NoError(t, err)
	require.NotNil(t, metrics, "Metrics should not be nil")

	assert.EqualValues(t, 1, metrics.AcquisitionsTotal)
	assert.EqualValues(t, 0, metrics.AcquisitionsFailed)

	err = lock.Release(ctx)
	require.NoError(t, err)

	metrics2, err := memLockManager.GetMetrics()
	require.NoError(t, err)
	require.NotNil(t, metrics2, "Metrics should not be nil")

	assert.EqualValues(t, 1, metrics2.ReleasesTotal)
}
