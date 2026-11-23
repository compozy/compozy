package api

import (
	"fmt"
	"maps"
	"net/http"
	"sync"
	"time"
)

const (
	accountWriteLimit  = 60
	accountWriteWindow = time.Minute
	CodeRateLimited    = "rate_limited"
)

type accountWindow struct {
	endsAt time.Time
	used   int
}

type AccountLimiter struct {
	mu        sync.Mutex
	windows   map[string]accountWindow
	nextSweep time.Time
	now       func() time.Time
}

func NewAccountLimiter(now func() time.Time) *AccountLimiter {
	return &AccountLimiter{windows: make(map[string]accountWindow), now: now}
}

// AccountWrites must run after the route authenticates the account.
func (l *AccountLimiter) AccountWrites(
	account func(*http.Request) (string, bool), next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		id, ok := account(r)
		if !ok || id == "" {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Account authentication required.")
			return
		}
		if retry := l.retryAfter(id); retry > 0 {
			setRetryAfter(w, retry)
			WriteError(w, http.StatusTooManyRequests, CodeRateLimited,
				fmt.Sprintf("Too many requests. Try again in %d seconds.", retry))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *AccountLimiter) retryAfter(accountID string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.nextSweep) {
		maps.DeleteFunc(l.windows, func(_ string, window accountWindow) bool { return !now.Before(window.endsAt) })
		l.nextSweep = now.Add(accountWriteWindow)
	}
	window := l.windows[accountID]
	if !now.Before(window.endsAt) {
		window = accountWindow{endsAt: now.Add(accountWriteWindow)}
	}
	if window.used >= accountWriteLimit {
		return int((window.endsAt.Sub(now) + time.Second - 1) / time.Second)
	}
	window.used++
	l.windows[accountID] = window
	return 0
}
