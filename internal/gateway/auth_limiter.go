package gateway

import (
	"sync"
	"time"
)

const maxTrackedAuthSources = 4096

type authFailureWindow struct {
	started time.Time
	count   int
}

// AuthFailureLimiter bounds failed remote authentication attempts per transport source.
type AuthFailureLimiter struct {
	mu       sync.Mutex
	failures map[string]authFailureWindow
	maximum  int
	window   time.Duration
	now      func() time.Time
}

func NewAuthFailureLimiter(maximum int, window time.Duration, now func() time.Time) *AuthFailureLimiter {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AuthFailureLimiter{
		failures: make(map[string]authFailureWindow), maximum: maximum, window: window, now: now,
	}
}

// Allow reports whether a source may attempt authentication. Local operator traffic is exempt.
func (l *AuthFailureLimiter) Allow(source string, localOperator bool) bool {
	if localOperator || l == nil || l.maximum <= 0 || l.window <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.failures[source]
	if exists && now.Sub(entry.started) >= l.window {
		delete(l.failures, source)
		return true
	}
	if !exists && len(l.failures) >= maxTrackedAuthSources {
		l.pruneExpired(now)
		if len(l.failures) >= maxTrackedAuthSources {
			return false
		}
	}
	return entry.count < l.maximum
}

// Failure records one failed remote authentication attempt.
func (l *AuthFailureLimiter) Failure(source string, localOperator bool) {
	if localOperator || l == nil || l.maximum <= 0 || l.window <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.failures[source]
	if !exists && len(l.failures) >= maxTrackedAuthSources {
		l.pruneExpired(now)
		if len(l.failures) >= maxTrackedAuthSources {
			return
		}
	}
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		entry = authFailureWindow{started: now}
	}
	entry.count++
	l.failures[source] = entry
}

func (l *AuthFailureLimiter) pruneExpired(now time.Time) {
	for source, entry := range l.failures {
		if now.Sub(entry.started) >= l.window {
			delete(l.failures, source)
		}
	}
}

// Success clears failures after a source proves possession of a valid credential.
func (l *AuthFailureLimiter) Success(source string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	delete(l.failures, source)
	l.mu.Unlock()
}
