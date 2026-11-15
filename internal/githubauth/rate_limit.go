package githubauth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var ErrRateLimited = errors.New("github: rate limit exceeded")

type RateLimitError struct {
	RetryAt       time.Time
	Authenticated bool
}

var _ error = (*RateLimitError)(nil)

func (e *RateLimitError) Error() string {
	action := "(unauthenticated); sign in with gh auth login or set GITHUB_TOKEN in the daemon environment"
	if e.Authenticated {
		action = "(authenticated); wait for the GitHub quota to reset"
	}
	return fmt.Sprintf("%s %s; retry after %s", ErrRateLimited, action, e.RetryAt.UTC().Format(time.RFC3339))
}

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

func RateLimit(response *http.Response, now time.Time) *RateLimitError {
	if response.StatusCode != http.StatusTooManyRequests &&
		!(response.StatusCode == http.StatusForbidden &&
			(response.Header.Get("X-RateLimit-Remaining") == "0" || response.Header.Get("Retry-After") != "")) {
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
	if reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		deadline = later(deadline, time.Unix(reset, 0))
	}
	return &RateLimitError{RetryAt: deadline, Authenticated: response.Request != nil &&
		response.Request.Header.Get("Authorization") != ""}
}

func later(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
