package worktree

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
)

type repositoryLock struct {
	token   chan struct{}
	refs    int
	waiters int
}

type RepositoryLocks struct {
	mu         sync.Mutex
	entries    map[string]*repositoryLock
	maxWaiters int
}

func NewRepositoryLocks(maxWaiters int) *RepositoryLocks {
	if maxWaiters < 0 {
		maxWaiters = 0
	}
	return &RepositoryLocks{entries: make(map[string]*repositoryLock), maxWaiters: maxWaiters}
}

func (l *RepositoryLocks) Acquire(ctx context.Context, commonDir string) (func(), error) {
	key, err := canonicalLockKey(commonDir)
	if err != nil {
		return nil, err
	}
	entry, waiting, err := l.reserve(key)
	if err != nil {
		return nil, err
	}
	if waiting {
		select {
		case <-ctx.Done():
			l.abandon(key, entry)
			return nil, fmt.Errorf("worktree: wait for repository mutation lock: %w", ctx.Err())
		case <-entry.token:
			l.entered(entry)
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.token <- struct{}{}
			l.release(key, entry)
		})
	}, nil
}

func (l *RepositoryLocks) reserve(key string) (*repositoryLock, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil {
		entry = &repositoryLock{token: make(chan struct{}, 1)}
		entry.token <- struct{}{}
		l.entries[key] = entry
	}
	entry.refs++
	select {
	case <-entry.token:
		return entry, false, nil
	default:
		if entry.waiters >= l.maxWaiters {
			entry.refs--
			return nil, false, ErrOperationInProgress
		}
		entry.waiters++
		return entry, true, nil
	}
}

func (l *RepositoryLocks) entered(entry *repositoryLock) {
	l.mu.Lock()
	entry.waiters--
	l.mu.Unlock()
}

func (l *RepositoryLocks) abandon(key string, entry *repositoryLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.waiters--
	entry.refs--
	l.prune(key, entry)
}

func (l *RepositoryLocks) release(key string, entry *repositoryLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	l.prune(key, entry)
}

func (l *RepositoryLocks) prune(key string, entry *repositoryLock) {
	if entry.refs == 0 && entry.waiters == 0 {
		delete(l.entries, key)
	}
}

func canonicalLockKey(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("worktree: resolve repository lock path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("worktree: canonicalize repository lock path: %w", err)
	}
	return filepath.Clean(resolved), nil
}
