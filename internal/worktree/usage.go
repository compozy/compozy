package worktree

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"
)

const exclusiveWorktreeUsageWeight = int64(math.MaxInt64)

type worktreeUsageLock struct {
	semaphore *semaphore.Weighted
	refs      int
}

type worktreeUsageLocks struct {
	mu      sync.Mutex
	entries map[string]*worktreeUsageLock
}

func newWorktreeUsageLocks() *worktreeUsageLocks {
	return &worktreeUsageLocks{entries: make(map[string]*worktreeUsageLock)}
}

func (l *worktreeUsageLocks) acquire(ctx context.Context, key string) (func(), error) {
	entry := l.reserve(key)
	if err := entry.semaphore.Acquire(ctx, 1); err != nil {
		l.releaseRef(key, entry)
		return nil, err
	}
	return sync.OnceFunc(func() {
		entry.semaphore.Release(1)
		l.releaseRef(key, entry)
	}), nil
}

func (l *worktreeUsageLocks) tryAcquireExclusive(key string) (func(), bool) {
	entry := l.reserve(key)
	if !entry.semaphore.TryAcquire(exclusiveWorktreeUsageWeight) {
		l.releaseRef(key, entry)
		return nil, false
	}
	return sync.OnceFunc(func() {
		entry.semaphore.Release(exclusiveWorktreeUsageWeight)
		l.releaseRef(key, entry)
	}), true
}

func (l *worktreeUsageLocks) reserve(key string) *worktreeUsageLock {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry == nil {
		entry = &worktreeUsageLock{semaphore: semaphore.NewWeighted(exclusiveWorktreeUsageWeight)}
		l.entries[key] = entry
	}
	entry.refs++
	return entry
}

func (l *worktreeUsageLocks) releaseRef(key string, entry *worktreeUsageLock) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, key)
	}
}

func worktreeUsageKey(workspaceID string, worktreeID string) string {
	return strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(worktreeID)
}

// AcquireUsage keeps a ready worktree available until the returned release function runs.
func (s *Service) AcquireUsage(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*Worktree, func(), error) {
	item, err := s.Get(ctx, workspaceID, ref)
	if err != nil {
		return nil, nil, err
	}
	if item.State != StateReady || strings.TrimSpace(item.Path) == "" {
		return nil, nil, ErrNotReady
	}

	release, err := s.usage.acquire(ctx, worktreeUsageKey(item.WorkspaceID, item.ID))
	if err != nil {
		return nil, nil, fmt.Errorf("worktree: acquire usage lease: %w", err)
	}
	current, err := s.Get(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		release()
		return nil, nil, err
	}
	if current.State != StateReady || strings.TrimSpace(current.Path) == "" {
		release()
		return nil, nil, ErrNotReady
	}
	return current, release, nil
}
