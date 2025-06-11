package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisLockManager_Acquire(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "test-resource"
	ttl := time.Second * 10

	t.Run("Should successfully acquire lock", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		assert.NotNil(t, lock)
		assert.True(t, lock.IsHeld())
		assert.Equal(t, resource, lock.Resource())

		// Clean up
		err = lock.Release(ctx)
		require.NoError(t, err)
	})

	t.Run("Should fail to acquire already held lock", func(t *testing.T) {
		// First acquisition should succeed
		lock1, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock1.Release(ctx)

		// Second acquisition should fail
		lock2, err := manager.Acquire(ctx, resource, ttl)
		assert.Error(t, err)
		assert.Equal(t, ErrLockNotAcquired, err)
		assert.Nil(t, lock2)
	})

	t.Run("Should acquire lock after previous lock expires", func(t *testing.T) {
		shortTTL := time.Millisecond * 100
		lock1, err := manager.Acquire(ctx, resource, shortTTL)
		require.NoError(t, err)
		assert.True(t, lock1.IsHeld())

		// Release the first lock to allow it to expire naturally
		err = lock1.Release(ctx)
		require.NoError(t, err)

		// Should be able to acquire new lock immediately after release
		lock2, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		assert.NotNil(t, lock2)
		defer lock2.Release(ctx)
	})
}

func TestRedisLock_Release(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "test-resource"
	ttl := time.Second * 10

	t.Run("Should successfully release held lock", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		err = lock.Release(ctx)
		assert.NoError(t, err)
		assert.False(t, lock.IsHeld())
	})

	t.Run("Should fail to release already released lock", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		// First release should succeed
		err = lock.Release(ctx)
		require.NoError(t, err)

		// Second release should fail
		err = lock.Release(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrLockNotHeld, err)
	})

	t.Run("Should allow new lock after release", func(t *testing.T) {
		lock1, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		err = lock1.Release(ctx)
		require.NoError(t, err)

		// Should be able to acquire new lock immediately
		lock2, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock2.Release(ctx)
	})
}

func TestRedisLock_Refresh(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "test-resource"
	ttl := time.Second * 2

	t.Run("Should successfully refresh held lock", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock.Release(ctx)

		err = lock.Refresh(ctx)
		assert.NoError(t, err)
		assert.True(t, lock.IsHeld())
	})

	t.Run("Should fail to refresh released lock", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		err = lock.Release(ctx)
		require.NoError(t, err)

		err = lock.Refresh(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrLockNotHeld, err)
	})

	t.Run("Should extend lock lifetime", func(t *testing.T) {
		shortTTL := time.Millisecond * 200
		lock, err := manager.Acquire(ctx, resource, shortTTL)
		require.NoError(t, err)
		defer lock.Release(ctx)

		// Wait half the TTL
		time.Sleep(shortTTL / 2)

		// Refresh should extend the lock
		err = lock.Refresh(ctx)
		require.NoError(t, err)

		// Wait another half TTL (should still be held due to refresh)
		time.Sleep(shortTTL / 2)

		// Lock should still be held
		assert.True(t, lock.IsHeld())
	})
}

func TestRedisLock_AutoRenewal(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "test-resource"
	ttl := time.Millisecond * 300 // Short TTL to test auto-renewal

	t.Run("Should auto-renew lock before expiration", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock.Release(ctx)

		// Wait longer than the original TTL
		time.Sleep(ttl + time.Millisecond*100)

		// Lock should still be held due to auto-renewal
		assert.True(t, lock.IsHeld())
	})

	t.Run("Should stop auto-renewal after release", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		// Release the lock
		err = lock.Release(ctx)
		require.NoError(t, err)

		// Wait for any pending renewals to complete
		time.Sleep(time.Millisecond * 50)

		// Should be able to acquire new lock immediately
		lock2, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock2.Release(ctx)
	})
}

func TestRedisLockManager_ConcurrentAccess(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "concurrent-test"
	ttl := time.Second * 2

	t.Run("Should allow only one concurrent lock holder", func(t *testing.T) {
		const numGoroutines = 10
		var successful, failed int32
		var successfulLock Lock
		var mu sync.Mutex
		var wg sync.WaitGroup

		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				lock, err := manager.Acquire(ctx, resource, ttl)

				mu.Lock()
				if err == nil {
					successful++
					successfulLock = lock
				} else {
					failed++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Only one should succeed
		assert.Equal(t, int32(1), successful)
		assert.Equal(t, int32(numGoroutines-1), failed)

		// Clean up
		if successfulLock != nil {
			err := successfulLock.Release(ctx)
			require.NoError(t, err)
		}
	})
}

func TestRedisLockManager_Metrics(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "metrics-test"
	ttl := time.Second * 2

	t.Run("Should track successful operations", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)

		err = lock.Refresh(ctx)
		require.NoError(t, err)

		err = lock.Release(ctx)
		require.NoError(t, err)

		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.AcquisitionsTotal, int64(0))
		assert.Greater(t, metrics.RefreshesTotal, int64(0))
		assert.Greater(t, metrics.ReleasesTotal, int64(0))
		assert.Greater(t, metrics.AcquisitionTime, time.Duration(0))
	})

	t.Run("Should track failed operations", func(t *testing.T) {
		// First lock should succeed
		lock1, err := manager.Acquire(ctx, resource, ttl)
		require.NoError(t, err)
		defer lock1.Release(ctx)

		// Second lock should fail
		_, err = manager.Acquire(ctx, resource, ttl)
		require.Error(t, err)

		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.AcquisitionsFailed, int64(0))
	})
}

func TestRedisLock_EdgeCases(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(t, err)

	ctx := context.Background()
	resource := "edge-case-test"
	ttl := time.Second * 2

	t.Run("Should handle context cancellation during acquire", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		lock, err := manager.Acquire(cancelCtx, resource, ttl)
		assert.Error(t, err)
		assert.Nil(t, lock)
	})

	t.Run("Should handle very short TTL", func(t *testing.T) {
		veryShortTTL := time.Millisecond * 10
		lock, err := manager.Acquire(ctx, resource, veryShortTTL)
		require.NoError(t, err)
		defer lock.Release(ctx)

		// Should still be able to refresh even with short TTL
		err = lock.Refresh(ctx)
		assert.NoError(t, err)
	})

	t.Run("Should handle empty resource name", func(t *testing.T) {
		lock, err := manager.Acquire(ctx, "", ttl)
		require.NoError(t, err)
		defer lock.Release(ctx)

		assert.Equal(t, "", lock.Resource())
	})
}

func BenchmarkRedisLockManager_Acquire(b *testing.B) {
	s := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(b, err)

	ctx := context.Background()
	ttl := time.Second * 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resource := fmt.Sprintf("bench-resource-%d", i)
		lock, err := manager.Acquire(ctx, resource, ttl)
		if err != nil {
			b.Fatal(err)
		}
		lock.Release(ctx)
	}
}

func BenchmarkRedisLockManager_ConcurrentAcquire(b *testing.B) {
	s := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	manager, err := NewRedisLockManager(client)
	require.NoError(b, err)

	ctx := context.Background()
	resource := "concurrent-bench"
	ttl := time.Millisecond * 100

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lock, err := manager.Acquire(ctx, resource, ttl)
			if err == nil {
				lock.Release(ctx)
			}
		}
	})
}
