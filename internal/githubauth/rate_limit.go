package githubauth

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrRateLimited = errors.New("github: rate limit exceeded")

type RateLimitError struct {
	RetryAt       time.Time
	Authenticated bool
}

var _ error = (*RateLimitError)(nil)

func (e *RateLimitError) Error() string {
	action := "(unauthenticated); sign in with gh auth login or set GITHUB_TOKEN in the daemon environment" +
		" (takes effect after the retry deadline or a daemon restart)"
	if e.Authenticated {
		action = "(authenticated); wait for the GitHub quota to reset"
	}
	return fmt.Sprintf("%s %s; retry after %s", ErrRateLimited, action, e.RetryAt.UTC().Format(time.RFC3339))
}

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

func RateLimit(response *http.Response, now time.Time) *RateLimitError {
	if response.StatusCode != http.StatusTooManyRequests &&
		(response.StatusCode != http.StatusForbidden ||
			(response.Header.Get("X-RateLimit-Remaining") != "0" && response.Header.Get("Retry-After") == "" &&
				!secondaryRateLimit(response))) {
		return nil
	}
	deadline := now.Add(time.Minute)
	if seconds, err := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 64); err == nil && seconds > 0 {
		// Compare seconds before converting to avoid overflowing a duration from an upstream header.
		if seconds <= int64((1<<63-1)/time.Second) {
			deadline = later(deadline, now.Add(time.Duration(seconds)*time.Second))
		}
	} else if retryAt, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil {
		deadline = later(deadline, retryAt)
	}
	if reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil &&
		response.Header.Get("X-RateLimit-Remaining") == "0" {
		deadline = later(deadline, time.Unix(reset, 0))
	}
	return &RateLimitError{RetryAt: deadline, Authenticated: response.Request != nil &&
		response.Request.Header.Get("Authorization") != ""}
}

// Inspect only a bounded prefix and replay it so ordinary forbidden responses retain their diagnostic body.
func secondaryRateLimit(response *http.Response) bool {
	if response.Body == nil {
		return false
	}
	body := response.Body
	prefix, readErr := io.ReadAll(io.LimitReader(body, 8<<10))
	var remainder io.Reader = body
	if readErr != nil {
		remainder = responseReadError{err: readErr}
	}
	response.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(bytes.NewReader(prefix), remainder), Closer: body,
	}
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(prefix, &payload) != nil {
		return false
	}
	message := strings.ToLower(payload.Message)
	return strings.Contains(message, "secondary rate limit") || strings.Contains(message, "abuse detection")
}

type responseReadError struct{ err error }

var _ io.Reader = responseReadError{}

func (r responseReadError) Read([]byte) (int, error) { return 0, r.err }

func later(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}

// RateLimitGate retains upstream deadlines across transient clients owned by one service.
// Each configured API origin has an independent quota boundary.
type RateLimitGate struct {
	mu      sync.Mutex
	limited map[string]*RateLimitError
}

func (g *RateLimitGate) Check(origin string, now time.Time) *RateLimitError {
	g.mu.Lock()
	defer g.mu.Unlock()
	limited := g.limited[origin]
	if limited != nil && now.Before(limited.RetryAt) {
		return limited
	}
	delete(g.limited, origin)
	return nil
}

func (g *RateLimitGate) Record(origin string, limited *RateLimitError) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.limited == nil {
		g.limited = make(map[string]*RateLimitError)
	}
	if prior := g.limited[origin]; prior == nil || limited.RetryAt.After(prior.RetryAt) {
		g.limited[origin] = limited
	}
}
