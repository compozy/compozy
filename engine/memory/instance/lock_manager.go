package instance

import (
	"context"
	"errors"
	"fmt"
	"time"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

// Locker defines the interface for locking operations
type Locker interface {
	Lock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

// Lock represents an acquired lock
type Lock interface {
	Unlock(ctx context.Context) error
}

// DefaultLockTTLs defines default lock timeouts
var DefaultLockTTLs = struct {
	Append time.Duration
	Clear  time.Duration
	Flush  time.Duration
}{
	Append: 30 * time.Second,
	Clear:  10 * time.Second,
	Flush:  5 * time.Minute,
}

// LockManagerImpl implements distributed locking for memory operations
type LockManagerImpl struct {
	locker Locker
	ttls   struct {
		append time.Duration
		clear  time.Duration
		flush  time.Duration
	}
}

// NewLockManager creates a new lock manager
func NewLockManager(locker Locker) *LockManagerImpl {
	return &LockManagerImpl{
		locker: locker,
		ttls: struct {
			append time.Duration
			clear  time.Duration
			flush  time.Duration
		}{
			append: DefaultLockTTLs.Append,
			clear:  DefaultLockTTLs.Clear,
			flush:  DefaultLockTTLs.Flush,
		},
	}
}

// WithAppendTTL sets the append lock TTL
func (lm *LockManagerImpl) WithAppendTTL(ttl time.Duration) *LockManagerImpl {
	lm.ttls.append = ttl
	return lm
}

// WithClearTTL sets the clear lock TTL
func (lm *LockManagerImpl) WithClearTTL(ttl time.Duration) *LockManagerImpl {
	lm.ttls.clear = ttl
	return lm
}

// WithFlushTTL sets the flush lock TTL
func (lm *LockManagerImpl) WithFlushTTL(ttl time.Duration) *LockManagerImpl {
	lm.ttls.flush = ttl
	return lm
}

// AcquireAppendLock acquires a lock for append operations
func (lm *LockManagerImpl) AcquireAppendLock(ctx context.Context, key string) (UnlockFunc, error) {
	lockKey := fmt.Sprintf("%s:append_lock", key)
	lock, err := lm.locker.Lock(ctx, lockKey, lm.ttls.append)
	if err != nil {
		// Check if this is a lock acquisition failure and wrap with core error
		if errors.Is(err, memcore.ErrLockAcquisitionFailed) {
			return nil, fmt.Errorf("%w: %v", memcore.ErrAppendLockFailed, err)
		}
		return nil, fmt.Errorf("failed to acquire append lock: %w", err)
	}

	return lm.createUnlockFunc(ctx, lock), nil
}

// AcquireClearLock acquires a lock for clear operations
func (lm *LockManagerImpl) AcquireClearLock(ctx context.Context, key string) (UnlockFunc, error) {
	lockKey := fmt.Sprintf("%s:clear_lock", key)
	lock, err := lm.locker.Lock(ctx, lockKey, lm.ttls.clear)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire clear lock: %w", err)
	}

	return lm.createUnlockFunc(ctx, lock), nil
}

// AcquireFlushLock acquires a lock for flush operations
func (lm *LockManagerImpl) AcquireFlushLock(ctx context.Context, key string) (UnlockFunc, error) {
	lockKey := fmt.Sprintf("%s:flush_lock", key)
	lock, err := lm.locker.Lock(ctx, lockKey, lm.ttls.flush)
	if err != nil {
		// Check if this is a lock acquisition failure and wrap with core error
		if errors.Is(err, memcore.ErrLockAcquisitionFailed) {
			return nil, fmt.Errorf("%w: %v", memcore.ErrFlushLockFailed, err)
		}
		return nil, fmt.Errorf("failed to acquire flush lock: %w", err)
	}

	return lm.createUnlockFunc(ctx, lock), nil
}

// createUnlockFunc creates a function to release a lock
func (lm *LockManagerImpl) createUnlockFunc(ctx context.Context, lock Lock) UnlockFunc {
	return func() error {
		// Derive an uncancelable context but preserve values for logging/tracing
		base := context.WithoutCancel(ctx)
		unlockCtx, cancel := context.WithTimeout(base, 5*time.Second)
		defer cancel()

		if err := lock.Unlock(unlockCtx); err != nil {
			return fmt.Errorf("failed to release lock: %w", err)
		}

		return nil
	}
}
