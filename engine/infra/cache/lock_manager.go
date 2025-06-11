package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// LockManager provides distributed locking capabilities
type LockManager interface {
	Acquire(ctx context.Context, resource string, ttl time.Duration) (Lock, error)
}

// Lock represents a distributed lock
type Lock interface {
	Release(ctx context.Context) error
	Refresh(ctx context.Context) error
	Resource() string
	IsHeld() bool
}

// RedisLockManager implements distributed locking using Redis and Redlock algorithm
type RedisLockManager struct {
	client  RedisInterface
	metrics *LockMetrics
}

// LockMetrics tracks lock operations for monitoring
type LockMetrics struct {
	mu                 sync.RWMutex
	AcquisitionsTotal  int64
	AcquisitionsFailed int64
	ReleasesTotal      int64
	ReleasesFailed     int64
	RefreshesTotal     int64
	RefreshesFailed    int64
	AcquisitionTime    time.Duration
}

// redisLock implements the Lock interface
type redisLock struct {
	manager    *RedisLockManager
	key        string
	value      string
	ttl        time.Duration
	stopChan   chan struct{}
	mu         sync.RWMutex
	held       bool
	renewalErr error
}

// Lua script for safe lock release (only releases if we own the lock)
const releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`

// Lua script for safe lock refresh (only refreshes if we own the lock)
const refreshLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("pexpire", KEYS[1], ARGV[2])
else
    return 0
end`

// NewRedisLockManager creates a new Redis-based distributed lock manager
func NewRedisLockManager(client RedisInterface) (*RedisLockManager, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	return &RedisLockManager{
		client:  client,
		metrics: &LockMetrics{},
	}, nil
}

// Acquire attempts to acquire a distributed lock on the given resource
func (m *RedisLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (Lock, error) {
	start := time.Now()
	defer func() {
		m.metrics.mu.Lock()
		m.metrics.AcquisitionTime = time.Since(start)
		m.metrics.mu.Unlock()
	}()

	lockKey := fmt.Sprintf("lock:%s", resource)
	lockValue := generateLockValue()

	// Try to acquire lock with NX option (set if not exists)
	ok, err := m.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		m.metrics.mu.Lock()
		m.metrics.AcquisitionsFailed++
		m.metrics.mu.Unlock()
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !ok {
		m.metrics.mu.Lock()
		m.metrics.AcquisitionsFailed++
		m.metrics.mu.Unlock()
		return nil, ErrLockNotAcquired
	}

	m.metrics.mu.Lock()
	m.metrics.AcquisitionsTotal++
	m.metrics.mu.Unlock()

	lock := &redisLock{
		manager:  m,
		key:      lockKey,
		value:    lockValue,
		ttl:      ttl,
		stopChan: make(chan struct{}),
		held:     true,
	}

	// Start auto-renewal goroutine
	go lock.autoRenew()

	return lock, nil
}

// GetMetrics returns current lock metrics
func (m *RedisLockManager) GetMetrics() LockMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	// Return a copy of the metrics without copying the mutex
	return LockMetrics{
		AcquisitionsTotal:  m.metrics.AcquisitionsTotal,
		AcquisitionsFailed: m.metrics.AcquisitionsFailed,
		ReleasesTotal:      m.metrics.ReleasesTotal,
		ReleasesFailed:     m.metrics.ReleasesFailed,
		RefreshesTotal:     m.metrics.RefreshesTotal,
		RefreshesFailed:    m.metrics.RefreshesFailed,
		AcquisitionTime:    m.metrics.AcquisitionTime,
	}
}

// Release releases the distributed lock
func (l *redisLock) Release(ctx context.Context) error {
	l.mu.Lock()
	if !l.held {
		l.mu.Unlock()
		return ErrLockNotHeld
	}
	l.held = false
	close(l.stopChan) // Stop auto-renewal
	l.mu.Unlock()

	// Use Lua script to ensure we only release our own lock
	result, err := l.manager.client.Eval(ctx, releaseLockScript, []string{l.key}, l.value).Result()
	if err != nil {
		l.manager.metrics.mu.Lock()
		l.manager.metrics.ReleasesFailed++
		l.manager.metrics.mu.Unlock()
		return fmt.Errorf("failed to release lock: %w", err)
	}

	l.manager.metrics.mu.Lock()
	l.manager.metrics.ReleasesTotal++
	l.manager.metrics.mu.Unlock()

	// result = 1 means lock was successfully deleted, 0 means we didn't own it
	if resultVal, ok := result.(int64); !ok || resultVal == 0 {
		return ErrLockNotOwned
	}

	return nil
}

// Refresh extends the TTL of the lock
func (l *redisLock) Refresh(ctx context.Context) error {
	l.mu.RLock()
	if !l.held {
		l.mu.RUnlock()
		return ErrLockNotHeld
	}
	key := l.key
	value := l.value
	ttl := l.ttl
	l.mu.RUnlock()

	// Use Lua script to ensure we only refresh our own lock
	result, err := l.manager.client.Eval(ctx, refreshLockScript, []string{key}, value, int64(ttl/time.Millisecond)).
		Result()
	if err != nil {
		l.manager.metrics.mu.Lock()
		l.manager.metrics.RefreshesFailed++
		l.manager.metrics.mu.Unlock()
		return fmt.Errorf("failed to refresh lock: %w", err)
	}

	l.manager.metrics.mu.Lock()
	l.manager.metrics.RefreshesTotal++
	l.manager.metrics.mu.Unlock()

	// result = 1 means lock was successfully refreshed, 0 means we didn't own it
	if resultVal, ok := result.(int64); !ok || resultVal == 0 {
		l.mu.Lock()
		l.held = false
		l.mu.Unlock()
		return ErrLockNotOwned
	}

	return nil
}

// Resource returns the resource name this lock protects
func (l *redisLock) Resource() string {
	return l.key[5:] // Remove "lock:" prefix
}

// IsHeld returns whether the lock is currently held
func (l *redisLock) IsHeld() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.held
}

// autoRenew automatically refreshes the lock before it expires
func (l *redisLock) autoRenew() {
	// Refresh at 1/3 of TTL to ensure we don't lose the lock
	refreshInterval := l.ttl / 3
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			err := l.Refresh(ctx)
			cancel()

			if err != nil {
				l.mu.Lock()
				l.renewalErr = err
				l.held = false
				l.mu.Unlock()
				return
			}
		}
	}
}

// generateLockValue creates a cryptographically random value for the lock
func generateLockValue() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// Enhanced fallback with timestamp and PID to reduce collision risk
		pid := os.Getpid()
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("lock_%d_%d", timestamp, pid)
	}
	return hex.EncodeToString(bytes)
}

// Common errors for lock operations
var (
	ErrLockNotAcquired = fmt.Errorf("lock could not be acquired")
	ErrLockNotHeld     = fmt.Errorf("lock is not currently held")
	ErrLockNotOwned    = fmt.Errorf("lock is not owned by this instance")
)
