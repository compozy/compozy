package gateway

import (
	"errors"
	"sync"
	"time"
)

const maxTrackedAuthSources = 4096

type authFailureWindow struct {
	started  time.Time
	failures int
	inFlight int
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

// Reconfigure applies new authentication limits without replacing active HTTP handlers.
func (l *AuthFailureLimiter) Reconfigure(maximum int, window time.Duration) error {
	if l == nil {
		return errors.New("gateway: authentication limiter is required")
	}
	if maximum <= 0 {
		return errors.New("gateway: authentication failure limit must be positive")
	}
	if window <= 0 {
		return errors.New("gateway: authentication failure window must be positive")
	}
	l.mu.Lock()
	l.maximum = maximum
	l.window = window
	l.pruneExpired(l.now())
	l.mu.Unlock()
	return nil
}

// Allow reports whether a source may attempt authentication. Local operator traffic is exempt.
func (l *AuthFailureLimiter) Allow(source string, localOperator bool) bool {
	if localOperator || l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.maximum <= 0 || l.window <= 0 {
		return true
	}
	now := l.now()
	entry, exists := l.failures[source]
	if exists && now.Sub(entry.started) >= l.window {
		entry = authFailureWindow{started: now}
		exists = false
	}
	if !exists && len(l.failures) >= maxTrackedAuthSources {
		l.pruneExpired(now)
		if len(l.failures) >= maxTrackedAuthSources {
			return false
		}
	}
	if entry.started.IsZero() {
		entry.started = now
	}
	if entry.failures+entry.inFlight >= l.maximum {
		return false
	}
	entry.inFlight++
	l.failures[source] = entry
	return true
}

// Failure records one failed remote authentication attempt.
func (l *AuthFailureLimiter) Failure(source string, localOperator bool) {
	if localOperator || l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.maximum <= 0 || l.window <= 0 {
		return
	}
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
	if entry.inFlight > 0 {
		entry.inFlight--
	}
	entry.failures++
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
	entry, exists := l.failures[source]
	if exists && entry.inFlight > 1 {
		entry.failures = 0
		entry.inFlight--
		entry.started = l.now()
		l.failures[source] = entry
	} else {
		delete(l.failures, source)
	}
	l.mu.Unlock()
}
