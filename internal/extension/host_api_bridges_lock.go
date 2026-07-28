package extensionpkg

import (
	"context"

	"errors"

	"strings"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type hostAPIKeyLocker struct {
	mu    sync.Mutex
	locks map[string]*hostAPIKeyLock
}

type hostAPIKeyLock struct {
	refs int
	mu   sync.Mutex
}

func newHostAPIKeyLocker() *hostAPIKeyLocker {
	return &hostAPIKeyLocker{locks: make(map[string]*hostAPIKeyLock)}
}

func (l *hostAPIKeyLocker) lock(key string) func() {
	normalized := strings.TrimSpace(key)
	if normalized == "" {
		normalized = hostAPIBridgesDefaultKey
	}

	l.mu.Lock()
	entry := l.locks[normalized]
	if entry == nil {
		entry = &hostAPIKeyLock{}
		l.locks[normalized] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()

		l.mu.Lock()
		defer l.mu.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.locks, normalized)
		}
	}
}

func retrySQLiteBusy(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := range hostAPIBusyRetryAttempts {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn()
		if !isSQLiteBusy(lastErr) {
			return lastErr
		}
		if attempt == hostAPIBusyRetryAttempts-1 {
			break
		}

		delay := time.Duration(attempt+1) * 5 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		}
	}
	return lastErr
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	switch sqliteErr.Code() & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return true
	default:
		return false
	}
}
